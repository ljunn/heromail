package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	gomail "github.com/emersion/go-message/mail"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type MailboxVerificationRepository interface {
	GetMailboxCredential(mailboxID string) (store.MailboxCredential, error)
	UpdateMailboxCredential(actorID, mailboxID string, credential map[string]string, validUntil time.Time, ip string) error
	UpdateMailboxVerification(actorID, mailboxID, method, status, verificationError string, verifiedAt time.Time, ip string) error
}

type GraphConnector interface {
	RefreshCredential(ctx context.Context, credential map[string]string) (map[string]string, time.Time, error)
	Profile(ctx context.Context, accessToken string) (Profile, error)
	Messages(ctx context.Context, accessToken string) ([]Message, error)
}

type graphAllMessageConnector interface {
	AllMessages(ctx context.Context, accessToken string) ([]Message, error)
}

type IMAPConnector interface {
	Verify(ctx context.Context, address string, credential map[string]string) error
}

type IMAPMessageConnector interface {
	IMAPConnector
	Messages(ctx context.Context, address string, credential map[string]string) ([]Message, error)
}

type imapAllMessageConnector interface {
	AllMessages(ctx context.Context, address string, credential map[string]string) ([]Message, error)
}

type mailboxHistoryRepository interface {
	ListServices() []domain.Service
	store.MailboxServiceStateRepository
}

type MailboxVerificationResult struct {
	Method     string    `json:"method"`
	Status     string    `json:"status"`
	VerifiedAt time.Time `json:"verified_at"`
}

type MailboxVerifier struct {
	repository MailboxVerificationRepository
	graph      GraphConnector
	imap       IMAPConnector
}

func NewMailboxVerifier(repository MailboxVerificationRepository, graph GraphConnector, imap IMAPConnector) *MailboxVerifier {
	return &MailboxVerifier{repository: repository, graph: graph, imap: imap}
}

func (v *MailboxVerifier) Verify(ctx context.Context, actorID, mailboxID, ip string) (MailboxVerificationResult, error) {
	credential, err := v.repository.GetMailboxCredential(mailboxID)
	if err != nil {
		return MailboxVerificationResult{}, err
	}
	now := time.Now().UTC()
	config := cloneCredential(credential.Config)
	microsoftMailbox := supportsMicrosoftGraph(credential.Mailbox)
	graphErr := errors.New("缺少 Graph OAuth 凭证")
	if microsoftMailbox && v.graph != nil && (config["access_token"] != "" || config["refresh_token"] != "") {
		accessToken := config["access_token"]
		validUntil, _ := time.Parse(time.RFC3339, config["expires_at"])
		didRefresh := false
		if accessToken == "" || (!validUntil.IsZero() && !validUntil.After(now)) {
			refreshedCredential, newValidUntil, refreshErr := v.graph.RefreshCredential(ctx, config)
			if refreshErr != nil {
				graphErr = refreshErr
			} else {
				config = mergeCredential(config, refreshedCredential)
				accessToken = config["access_token"]
				if saveErr := v.repository.UpdateMailboxCredential(actorID, mailboxID, config, newValidUntil, ip); saveErr != nil {
					graphErr = saveErr
				} else {
					graphErr = nil
					didRefresh = true
				}
			}
		} else {
			graphErr = nil
		}
		if graphErr == nil && accessToken != "" {
			graphErr = v.verifyGraphAccess(ctx, credential.Mailbox.Address, accessToken)
			if graphErr != nil && !didRefresh && config["refresh_token"] != "" {
				newCredential, newValidUntil, refreshErr := v.graph.RefreshCredential(ctx, config)
				if refreshErr == nil {
					config = mergeCredential(config, newCredential)
					accessToken = config["access_token"]
					if saveErr := v.repository.UpdateMailboxCredential(actorID, mailboxID, config, newValidUntil, ip); saveErr == nil {
						graphErr = v.verifyGraphAccess(ctx, credential.Mailbox.Address, accessToken)
						didRefresh = graphErr == nil
					} else {
						graphErr = saveErr
					}
				}
			}
			if graphErr == nil {
				if !didRefresh {
					if validUntil.IsZero() {
						validUntil = now.Add(15 * time.Minute)
					}
					if saveErr := v.repository.UpdateMailboxCredential(actorID, mailboxID, config, validUntil, ip); saveErr != nil {
						graphErr = saveErr
					}
				}
			}
			if graphErr == nil {
				return v.markVerified(actorID, mailboxID, domain.MailboxConnectionMicrosoftGraph, now, ip)
			}
		}
	}

	imapErr := errors.New("缺少 IMAP 凭证")
	if v.imap != nil && (config["access_token"] != "" || config["password"] != "") {
		verifyContext, cancel := context.WithTimeout(ctx, 25*time.Second)
		imapErr = v.imap.Verify(verifyContext, credential.Mailbox.Address, config)
		cancel()
		if imapErr == nil {
			return v.markVerified(actorID, mailboxID, domain.MailboxConnectionIMAP, now, ip)
		}
	}

	message := compactVerificationError(graphErr, imapErr)
	if !microsoftMailbox {
		message = compactIMAPError(imapErr)
	}
	if updateErr := v.repository.UpdateMailboxVerification(actorID, mailboxID, domain.MailboxConnectionAuto, domain.MailboxVerificationFailed, message, now, ip); updateErr != nil {
		return MailboxVerificationResult{}, updateErr
	}
	return MailboxVerificationResult{Method: domain.MailboxConnectionAuto, Status: domain.MailboxVerificationFailed, VerifiedAt: now}, errors.New(message)
}

// ReadMessages 读取授权请求所需的邮箱内容；Microsoft 优先 Graph，其余渠道直接使用 IMAP。
// 凭证只在本次请求的内存中使用，正文也不会写入审计日志。
func (v *MailboxVerifier) ReadMessages(ctx context.Context, actorID, mailboxID, ip string) ([]Message, error) {
	credential, err := v.repository.GetMailboxCredential(mailboxID)
	if err != nil {
		return nil, err
	}
	config := cloneCredential(credential.Config)
	microsoftMailbox := supportsMicrosoftGraph(credential.Mailbox)
	graphErr := errors.New("缺少 Graph OAuth 凭证")
	if microsoftMailbox && v.graph != nil && (config["access_token"] != "" || config["refresh_token"] != "") {
		accessToken := config["access_token"]
		validUntil, _ := time.Parse(time.RFC3339, config["expires_at"])
		if accessToken == "" || (!validUntil.IsZero() && !validUntil.After(time.Now().UTC())) {
			refreshed, newValidUntil, refreshErr := v.graph.RefreshCredential(ctx, config)
			if refreshErr != nil {
				graphErr = refreshErr
			} else {
				config = mergeCredential(config, refreshed)
				accessToken = config["access_token"]
				graphErr = v.repository.UpdateMailboxCredential(actorID, mailboxID, config, newValidUntil, ip)
			}
		}
		if graphErr == nil || (graphErr.Error() == "缺少 Graph OAuth 凭证" && accessToken != "") {
			if accessToken != "" {
				if reader, ok := v.graph.(graphAllMessageConnector); ok {
					messages, readErr := reader.AllMessages(ctx, accessToken)
					if readErr == nil {
						return messages, nil
					}
					graphErr = readErr
				} else {
					messages, readErr := v.graph.Messages(ctx, accessToken)
					if readErr == nil {
						return messages, nil
					}
					graphErr = readErr
				}
			}
		}
	}
	if v.imap != nil && (config["access_token"] != "" || config["password"] != "") {
		imapContext, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if reader, ok := v.imap.(imapAllMessageConnector); ok {
			if messages, readErr := reader.AllMessages(imapContext, credential.Mailbox.Address, config); readErr == nil {
				return messages, nil
			} else {
				if !microsoftMailbox {
					return nil, fmt.Errorf("IMAP：%w", readErr)
				}
				return nil, fmt.Errorf("Graph：%v；IMAP：%v", graphErr, readErr)
			}
		}
		if reader, ok := v.imap.(IMAPMessageConnector); ok {
			if messages, readErr := reader.Messages(imapContext, credential.Mailbox.Address, config); readErr == nil {
				return messages, nil
			} else {
				if !microsoftMailbox {
					return nil, fmt.Errorf("IMAP：%w", readErr)
				}
				return nil, fmt.Errorf("Graph：%v；IMAP：%v", graphErr, readErr)
			}
		}
	}
	if !microsoftMailbox {
		return nil, errors.New("缺少 IMAP 凭证")
	}
	return nil, fmt.Errorf("Graph：%s", compactGraphError(graphErr))
}

// ScanMailboxHistory 在邮箱连接验证成功后检查历史邮件，写入邮箱×平台的已注册状态。
// 不写入 mail_events，避免历史邮件被标记后阻断后续活跃订单收码。
func (v *MailboxVerifier) ScanMailboxHistory(ctx context.Context, actorID, mailboxID, ip string) (int, error) {
	repository, ok := v.repository.(mailboxHistoryRepository)
	if !ok {
		return 0, nil
	}
	messages, err := v.ReadMessages(ctx, actorID, mailboxID, ip)
	if err != nil {
		return 0, err
	}
	services := repository.ListServices()
	matched := 0
	for _, message := range messages {
		for _, service := range services {
			if _, ok := matchCode(service, message); !ok {
				continue
			}
			if err := repository.MarkMailboxServiceConsumed(mailboxID, service.ID, message.ReceivedAt); err != nil {
				return matched, err
			}
			matched++
		}
	}
	return matched, nil
}

func (v *MailboxVerifier) verifyGraphAccess(ctx context.Context, address, accessToken string) error {
	profile, err := v.graph.Profile(ctx, accessToken)
	if err != nil {
		return err
	}
	if !strings.EqualFold(profile.Address, address) {
		return errors.New("Graph 账户与邮箱地址不一致")
	}
	_, err = v.graph.Messages(ctx, accessToken)
	return err
}

func (v *MailboxVerifier) markVerified(actorID, mailboxID, method string, now time.Time, ip string) (MailboxVerificationResult, error) {
	if err := v.repository.UpdateMailboxVerification(actorID, mailboxID, method, domain.MailboxVerificationVerified, "", now, ip); err != nil {
		return MailboxVerificationResult{}, err
	}
	return MailboxVerificationResult{Method: method, Status: domain.MailboxVerificationVerified, VerifiedAt: now}, nil
}

func cloneCredential(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func mergeCredential(existing, refreshed map[string]string) map[string]string {
	result := cloneCredential(existing)
	for key, value := range refreshed {
		if value != "" {
			result[key] = value
		}
	}
	return result
}

func compactVerificationError(graphErr, imapErr error) string {
	message := fmt.Sprintf("Graph：%s；IMAP：%s", compactGraphError(graphErr), compactGraphError(imapErr))
	if len(message) > 480 {
		message = message[:480]
	}
	return message
}

func compactGraphError(err error) string {
	if err == nil {
		return "无错误"
	}
	message := err.Error()
	if strings.Contains(message, "invalid_grant") || strings.Contains(message, "AADSTS70000") {
		return "Microsoft OAuth 授权已失效（invalid_grant），请重新授权 Graph 或切换 IMAP"
	}
	if strings.Contains(message, "AADSTS65001") {
		return "Microsoft OAuth 尚未授予所需权限，请重新授权 Graph"
	}
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func compactIMAPError(imapErr error) string {
	message := fmt.Sprintf("IMAP：%v", imapErr)
	if len(message) > 480 {
		message = message[:480]
	}
	return message
}

func supportsMicrosoftGraph(mailbox domain.Mailbox) bool {
	provider := mailbox.Provider
	if provider == "" {
		provider, _ = domain.DetectMailboxProvider(mailbox.Address)
	}
	return domain.IsMicrosoftMailboxProvider(provider)
}

type MicrosoftIMAPConnector struct {
}

func NewMicrosoftIMAPConnector() *MicrosoftIMAPConnector {
	return &MicrosoftIMAPConnector{}
}

type imapEndpoint struct {
	Address         string
	ServerName      string
	LoginCandidates []string
}

func imapServerConfig(address string) (imapEndpoint, error) {
	provider, ok := domain.DetectMailboxProvider(address)
	if !ok {
		return imapEndpoint{}, errors.New("不支持该邮箱类型")
	}
	endpoint := imapEndpoint{LoginCandidates: []string{address}}
	switch provider {
	case domain.MailboxProviderOutlook, domain.MailboxProviderOutlookDE, domain.MailboxProviderHotmail:
		endpoint.Address, endpoint.ServerName = "outlook.office365.com:993", "outlook.office365.com"
	case domain.MailboxProviderGmail:
		endpoint.Address, endpoint.ServerName = "imap.gmail.com:993", "imap.gmail.com"
	case domain.MailboxProviderICloud:
		endpoint.Address, endpoint.ServerName = "imap.mail.me.com:993", "imap.mail.me.com"
		if local, _, found := strings.Cut(address, "@"); found && local != "" {
			endpoint.LoginCandidates = []string{local, address}
		}
	case domain.MailboxProviderMailCom:
		endpoint.Address, endpoint.ServerName = "imap.mail.com:993", "imap.mail.com"
	}
	return endpoint, nil
}

func (c *MicrosoftIMAPConnector) Verify(ctx context.Context, address string, credential map[string]string) error {
	client, done, err := c.connect(ctx, address, credential)
	if err != nil {
		return err
	}
	defer done()
	if err := client.Noop().Wait(); err != nil {
		return fmt.Errorf("健康检查失败：%w", err)
	}
	return nil
}

func (c *MicrosoftIMAPConnector) connect(ctx context.Context, address string, credential map[string]string) (*imapclient.Client, func(), error) {
	endpoint, err := imapServerConfig(address)
	if err != nil {
		return nil, func() {}, err
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	client, err := imapclient.DialTLS(endpoint.Address, &imapclient.Options{
		Dialer:    dialer,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: endpoint.ServerName},
	})
	if err != nil {
		return nil, func() {}, fmt.Errorf("连接失败：%w", err)
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-done:
		}
	}()
	closeClient := func() {
		close(done)
		_ = client.Close()
	}
	if accessToken := credential["access_token"]; accessToken != "" {
		err = client.Authenticate(newXOAuth2Client(address, accessToken))
	} else if password := credential["password"]; password != "" {
		for _, username := range endpoint.LoginCandidates {
			err = client.Login(username, password).Wait()
			if err == nil {
				break
			}
		}
	} else {
		closeClient()
		return nil, func() {}, errors.New("缺少 OAuth Token 或密码")
	}
	if err != nil {
		closeClient()
		return nil, func() {}, fmt.Errorf("认证失败：%w", err)
	}
	return client, closeClient, nil
}

func (c *MicrosoftIMAPConnector) Messages(ctx context.Context, address string, credential map[string]string) ([]Message, error) {
	return c.messages(ctx, address, credential, 25)
}

// AllMessages 读取收件箱中的全部邮件，最多保留最近 1000 封。
func (c *MicrosoftIMAPConnector) AllMessages(ctx context.Context, address string, credential map[string]string) ([]Message, error) {
	return c.messages(ctx, address, credential, 1000)
}

func (c *MicrosoftIMAPConnector) messages(ctx context.Context, address string, credential map[string]string, limit int) ([]Message, error) {
	client, done, err := c.connect(ctx, address, credential)
	if err != nil {
		return nil, err
	}
	defer done()
	selected, err := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("打开收件箱失败：%w", err)
	}
	if selected.NumMessages == 0 {
		return []Message{}, nil
	}
	if limit < 1 {
		limit = 25
	}
	start := uint32(1)
	if uint32(limit) < selected.NumMessages {
		start = selected.NumMessages - uint32(limit) + 1
	}
	set := imap.SeqSetNum(start, selected.NumMessages)
	bodySection := &imap.FetchItemBodySection{Partial: &imap.SectionPartial{Offset: 0, Size: 64 * 1024}, Peek: true}
	items, err := client.Fetch(set, &imap.FetchOptions{UID: true, Envelope: true, InternalDate: true, BodySection: []*imap.FetchItemBodySection{bodySection}}).Collect()
	if err != nil {
		return nil, fmt.Errorf("读取收件箱失败：%w", err)
	}
	messages := make([]Message, 0, len(items))
	for _, item := range items {
		if item.Envelope == nil {
			continue
		}
		sender := ""
		if len(item.Envelope.From) > 0 {
			sender = strings.ToLower(item.Envelope.From[0].Addr())
		}
		receivedAt := item.InternalDate
		if receivedAt.IsZero() {
			receivedAt = item.Envelope.Date
		}
		body, preview, bodyType := parseMIMEMessage(item.FindBodySection(bodySection))
		messageID := item.Envelope.MessageID
		if messageID == "" {
			messageID = fmt.Sprintf("imap-%d", item.UID)
		}
		messages = append(messages, Message{ID: messageID, Sender: sender, Subject: item.Envelope.Subject, BodyPreview: preview, Body: body, BodyType: bodyType, ReceivedAt: receivedAt})
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].ReceivedAt.After(messages[j].ReceivedAt) })
	return messages, nil
}

func parseMIMEMessage(raw []byte) (body, preview, bodyType string) {
	if len(raw) == 0 {
		return "", "", ""
	}
	reader, _ := gomail.CreateReader(bytes.NewReader(raw))
	if reader == nil {
		return string(raw), string(raw), "text"
	}
	defer reader.Close()

	var htmlBody, plainBody string
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if part == nil {
			break
		}
		header, ok := part.Header.(*gomail.InlineHeader)
		if !ok {
			continue
		}
		mediaType, _, _ := header.ContentType()
		content, readErr := io.ReadAll(io.LimitReader(part.Body, 2<<20))
		if readErr != nil {
			continue
		}
		switch strings.ToLower(mediaType) {
		case "text/html":
			if htmlBody == "" {
				htmlBody = strings.TrimSpace(string(content))
			}
		case "text/plain":
			if plainBody == "" {
				plainBody = strings.TrimSpace(string(content))
			}
		}
	}
	if htmlBody != "" {
		return htmlBody, plainBody, "html"
	}
	if plainBody != "" {
		return plainBody, plainBody, "text"
	}
	return string(raw), string(raw), "text"
}

type xoauth2Client struct {
	username string
	token    string
	started  bool
}

func newXOAuth2Client(username, token string) *xoauth2Client {
	return &xoauth2Client{username: username, token: token}
}

func (c *xoauth2Client) Start() (string, []byte, error) {
	c.started = true
	response := "user=" + c.username + "\x01auth=Bearer " + c.token + "\x01\x01"
	return "XOAUTH2", []byte(response), nil
}

func (c *xoauth2Client) Next([]byte) ([]byte, error) {
	if !c.started {
		return nil, errors.New("XOAUTH2 尚未开始")
	}
	return []byte{}, nil
}
