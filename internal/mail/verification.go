package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
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

type IMAPConnector interface {
	Verify(ctx context.Context, address string, credential map[string]string) error
}

type IMAPMessageConnector interface {
	IMAPConnector
	Messages(ctx context.Context, address string, credential map[string]string) ([]Message, error)
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
	graphErr := errors.New("缺少 Graph OAuth 凭证")
	if v.graph != nil && (config["access_token"] != "" || config["refresh_token"] != "") {
		accessToken := config["access_token"]
		validUntil, _ := time.Parse(time.RFC3339, config["expires_at"])
		if accessToken == "" || time.Until(validUntil) < 5*time.Minute {
			refreshed, newValidUntil, refreshErr := v.graph.RefreshCredential(ctx, config)
			if refreshErr != nil {
				graphErr = refreshErr
			} else {
				config = mergeCredential(config, refreshed)
				accessToken = config["access_token"]
				if saveErr := v.repository.UpdateMailboxCredential(actorID, mailboxID, config, newValidUntil, ip); saveErr != nil {
					graphErr = saveErr
				} else {
					graphErr = nil
				}
			}
		} else {
			graphErr = nil
		}
		if graphErr == nil && accessToken != "" {
			profile, profileErr := v.graph.Profile(ctx, accessToken)
			if profileErr != nil {
				graphErr = profileErr
			} else if !strings.EqualFold(profile.Address, credential.Mailbox.Address) {
				graphErr = errors.New("Graph 账户与邮箱地址不一致")
			} else if _, messagesErr := v.graph.Messages(ctx, accessToken); messagesErr != nil {
				graphErr = messagesErr
			} else {
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
	if updateErr := v.repository.UpdateMailboxVerification(actorID, mailboxID, domain.MailboxConnectionAuto, domain.MailboxVerificationFailed, message, now, ip); updateErr != nil {
		return MailboxVerificationResult{}, updateErr
	}
	return MailboxVerificationResult{Method: domain.MailboxConnectionAuto, Status: domain.MailboxVerificationFailed, VerifiedAt: now}, errors.New(message)
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
	message := fmt.Sprintf("Graph：%v；IMAP：%v", graphErr, imapErr)
	if len(message) > 480 {
		message = message[:480]
	}
	return message
}

type MicrosoftIMAPConnector struct {
	address string
}

func NewMicrosoftIMAPConnector() *MicrosoftIMAPConnector {
	return &MicrosoftIMAPConnector{address: "outlook.office365.com:993"}
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
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	client, err := imapclient.DialTLS(c.address, &imapclient.Options{
		Dialer:    dialer,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "outlook.office365.com"},
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
		err = client.Login(address, password).Wait()
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
	start := uint32(1)
	if selected.NumMessages > 25 {
		start = selected.NumMessages - 24
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
		body := item.FindBodySection(bodySection)
		messageID := item.Envelope.MessageID
		if messageID == "" {
			messageID = fmt.Sprintf("imap-%d", item.UID)
		}
		messages = append(messages, Message{ID: messageID, Sender: sender, Subject: item.Envelope.Subject, BodyPreview: string(body), ReceivedAt: receivedAt})
	}
	return messages, nil
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
