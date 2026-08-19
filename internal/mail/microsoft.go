package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type MicrosoftConfig struct {
	ClientID     string
	ClientSecret string
	Tenant       string
	RedirectURI  string
	GraphBaseURL string
}

type MicrosoftClient struct {
	config MicrosoftConfig
	http   *http.Client
}

type Profile struct {
	Address string
}

type Message struct {
	ID          string
	Sender      string
	Subject     string
	BodyPreview string
	ReceivedAt  time.Time
}

type CodeReceiver interface {
	ReceiveCodeValue(id, code string) (domain.Order, error)
}

func NewMicrosoftClient(config MicrosoftConfig) *MicrosoftClient {
	if config.Tenant == "" {
		config.Tenant = "common"
	}
	if config.GraphBaseURL == "" {
		config.GraphBaseURL = "https://graph.microsoft.com/v1.0"
	}
	return &MicrosoftClient{config: config, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *MicrosoftClient) Enabled() bool {
	return c.config.ClientID != "" && c.config.ClientSecret != "" && c.config.RedirectURI != ""
}

func (c *MicrosoftClient) AuthURL(state string) string {
	values := url.Values{
		"client_id":     {c.config.ClientID},
		"response_type": {"code"},
		"redirect_uri":  {c.config.RedirectURI},
		"response_mode": {"query"},
		"scope":         {"offline_access Mail.Read User.Read"},
		"state":         {state},
		"prompt":        {"select_account"},
	}
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s", c.config.Tenant, values.Encode())
}

func (c *MicrosoftClient) Exchange(ctx context.Context, code string) (map[string]string, time.Time, error) {
	return c.token(ctx, url.Values{"client_id": {c.config.ClientID}, "client_secret": {c.config.ClientSecret}, "grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {c.config.RedirectURI}, "scope": {"offline_access Mail.Read User.Read"}})
}

func (c *MicrosoftClient) Refresh(ctx context.Context, refreshToken string) (map[string]string, time.Time, error) {
	return c.token(ctx, url.Values{"client_id": {c.config.ClientID}, "client_secret": {c.config.ClientSecret}, "grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "scope": {"offline_access Mail.Read User.Read"}})
}

func (c *MicrosoftClient) RefreshCredential(ctx context.Context, credential map[string]string) (map[string]string, time.Time, error) {
	clientID := strings.TrimSpace(credential["client_id"])
	if clientID == "" {
		clientID = c.config.ClientID
	}
	refreshToken := strings.TrimSpace(credential["refresh_token"])
	if clientID == "" || refreshToken == "" {
		return nil, time.Time{}, errors.New("缺少 Microsoft client_id 或 refresh_token")
	}
	values := url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"scope":         {"offline_access Mail.Read User.Read"},
	}
	clientSecret := strings.TrimSpace(credential["client_secret"])
	if clientSecret == "" && clientID == c.config.ClientID {
		clientSecret = c.config.ClientSecret
	}
	if clientSecret != "" {
		values.Set("client_secret", clientSecret)
	}
	tenant := strings.TrimSpace(credential["tenant"])
	if tenant == "" {
		tenant = c.config.Tenant
	}
	return c.tokenForTenant(ctx, tenant, values)
}

func (c *MicrosoftClient) token(ctx context.Context, values url.Values) (map[string]string, time.Time, error) {
	return c.tokenForTenant(ctx, c.config.Tenant, values)
}

func (c *MicrosoftClient) tokenForTenant(ctx context.Context, tenant string, values url.Values) (map[string]string, time.Time, error) {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return nil, time.Time{}, fmt.Errorf("Microsoft Token 接口返回 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return nil, time.Time{}, errors.New("Microsoft Token 响应无效")
	}
	credential := map[string]string{"access_token": payload.AccessToken, "refresh_token": payload.RefreshToken}
	validUntil := time.Now().UTC().Add(time.Duration(payload.ExpiresIn) * time.Second)
	credential["expires_at"] = validUntil.Format(time.RFC3339)
	return credential, validUntil, nil
}

func (c *MicrosoftClient) Profile(ctx context.Context, accessToken string) (Profile, error) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.config.GraphBaseURL+"/me?$select=mail,userPrincipalName", nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.http.Do(request)
	if err != nil {
		return Profile{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("Microsoft Graph 用户接口返回 %d", response.StatusCode)
	}
	var payload struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return Profile{}, err
	}
	address := payload.Mail
	if address == "" {
		address = payload.UserPrincipalName
	}
	if address == "" {
		return Profile{}, errors.New("Microsoft 账户没有可用邮箱地址")
	}
	return Profile{Address: strings.ToLower(address)}, nil
}

func (c *MicrosoftClient) Messages(ctx context.Context, accessToken string) ([]Message, error) {
	endpoint := c.config.GraphBaseURL + "/me/mailFolders/inbox/messages?$top=25&$select=id,subject,from,receivedDateTime,bodyPreview&$orderby=receivedDateTime%20desc"
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Microsoft Graph 邮件接口返回 %d", response.StatusCode)
	}
	var payload struct {
		Value []struct {
			ID          string `json:"id"`
			Subject     string `json:"subject"`
			BodyPreview string `json:"bodyPreview"`
			Received    string `json:"receivedDateTime"`
			From        struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"from"`
		} `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(payload.Value))
	for _, item := range payload.Value {
		receivedAt, _ := time.Parse(time.RFC3339, item.Received)
		messages = append(messages, Message{ID: item.ID, Sender: strings.ToLower(item.From.EmailAddress.Address), Subject: item.Subject, BodyPreview: item.BodyPreview, ReceivedAt: receivedAt})
	}
	return messages, nil
}

type Worker struct {
	repository workerRepository
	receiver   CodeReceiver
	client     *MicrosoftClient
	imap       IMAPMessageConnector
	interval   time.Duration
}

type workerRepository interface {
	ListMailboxCredentialsPage(afterID string, limit int) ([]store.MailboxCredential, error)
	UpdateMailboxCredential(actorID, mailboxID string, credential map[string]string, validUntil time.Time, ip string) error
	UpdateMailboxVerification(actorID, mailboxID, method, status, verificationError string, verifiedAt time.Time, ip string) error
	WaitingOrdersForMailbox(mailboxID string) []domain.Order
	ServiceByID(serviceID string) (domain.Service, bool)
	MarkMailEvent(mailboxID, messageID, sender, subject string, receivedAt time.Time) (bool, error)
}

func NewWorker(repository workerRepository, receiver CodeReceiver, client *MicrosoftClient, interval time.Duration) *Worker {
	if interval < 5*time.Second {
		interval = 15 * time.Second
	}
	return &Worker{repository: repository, receiver: receiver, client: client, imap: NewMicrosoftIMAPConnector(), interval: interval}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	const batchSize = 100
	afterID := ""
	for {
		mailboxes, err := w.repository.ListMailboxCredentialsPage(afterID, batchSize)
		if err != nil {
			return
		}
		for _, mailbox := range mailboxes {
			w.pollMailbox(ctx, mailbox)
		}
		if len(mailboxes) < batchSize {
			return
		}
		afterID = mailboxes[len(mailboxes)-1].Mailbox.ID
	}
}

func (w *Worker) pollMailbox(ctx context.Context, mailbox store.MailboxCredential) {
	orders := w.repository.WaitingOrdersForMailbox(mailbox.Mailbox.ID)
	if len(orders) == 0 {
		return
	}
	credential := mailbox.Config
	useIMAP := mailbox.Mailbox.ConnectionMethod == domain.MailboxConnectionIMAP
	validUntil, _ := time.Parse(time.RFC3339, credential["expires_at"])
	needsRefresh := credential["refresh_token"] != "" && (credential["access_token"] == "" || time.Until(validUntil) < 5*time.Minute)
	if needsRefresh {
		refreshed, newValidUntil, refreshErr := w.client.RefreshCredential(ctx, credential)
		if refreshErr != nil {
			return
		}
		credential, validUntil = mergeCredential(credential, refreshed), newValidUntil
		_ = w.repository.UpdateMailboxCredential("system", mailbox.Mailbox.ID, credential, validUntil, "")
	}
	var messages []Message
	var messageErr error
	if useIMAP {
		messages, messageErr = w.imap.Messages(ctx, mailbox.Mailbox.Address, credential)
	} else {
		if credential["access_token"] == "" {
			return
		}
		messages, messageErr = w.client.Messages(ctx, credential["access_token"])
	}
	if messageErr != nil {
		if useIMAP {
			message := messageErr.Error()
			if len(message) > 480 {
				message = message[:480]
			}
			_ = w.repository.UpdateMailboxVerification("system", mailbox.Mailbox.ID, domain.MailboxConnectionIMAP, domain.MailboxVerificationFailed, message, time.Now().UTC(), "")
		}
		return
	}
	for _, message := range messages {
		if message.ReceivedAt.Before(orders[0].SubmittedAt.Add(-time.Minute)) {
			continue
		}
		fresh, markErr := w.repository.MarkMailEvent(mailbox.Mailbox.ID, message.ID, message.Sender, message.Subject, message.ReceivedAt)
		if markErr != nil || !fresh {
			continue
		}
		for _, order := range orders {
			service, ok := w.repository.ServiceByID(order.ServiceID)
			if !ok {
				continue
			}
			code, matched := matchCode(service, message)
			if matched {
				_, _ = w.receiver.ReceiveCodeValue(order.ID, code)
				break
			}
		}
	}
}

func matchCode(service domain.Service, message Message) (string, bool) {
	senderAllowed := false
	for _, domainName := range service.SenderDomains {
		domainName = strings.ToLower(strings.TrimSpace(domainName))
		if strings.HasSuffix(message.Sender, "@"+domainName) || strings.HasSuffix(message.Sender, "."+domainName) {
			senderAllowed = true
			break
		}
	}
	if !senderAllowed {
		return "", false
	}
	if len(service.SubjectKeywords) > 0 {
		subject := strings.ToLower(message.Subject)
		matched := false
		for _, keyword := range service.SubjectKeywords {
			if strings.Contains(subject, strings.ToLower(keyword)) {
				matched = true
				break
			}
		}
		if !matched {
			return "", false
		}
	}
	pattern, err := regexp.Compile(service.Regex)
	if err != nil {
		return "", false
	}
	matches := pattern.FindStringSubmatch(message.Subject + "\n" + message.BodyPreview)
	if len(matches) > 1 {
		return matches[1], true
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return "", false
}
