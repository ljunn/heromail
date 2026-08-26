package mail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

func TestParseMIMEMessagePrefersHTMLAndKeepsPlainPreview(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=hero-boundary",
		"",
		"--hero-boundary",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"验证码是 628419",
		"--hero-boundary",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<html><body><strong>验证码是 628419</strong></body></html>",
		"--hero-boundary--",
	}, "\r\n")

	body, preview, bodyType := parseMIMEMessage([]byte(raw))
	if bodyType != "html" || !strings.Contains(body, "<strong>") || preview != "验证码是 628419" {
		t.Fatalf("MIME 正文解析错误：type=%q body=%q preview=%q", bodyType, body, preview)
	}
}

func TestParseMIMEMessageDecodesQuotedPrintablePlainText(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Your code is =36=32=38=34=31=39",
	}, "\r\n")

	body, preview, bodyType := parseMIMEMessage([]byte(raw))
	if bodyType != "text" || body != "Your code is 628419" || preview != body {
		t.Fatalf("纯文本 MIME 解码错误：type=%q body=%q preview=%q", bodyType, body, preview)
	}
}

type verificationRepositoryStub struct {
	credential store.MailboxCredential
	method     string
	status     string
	message    string
	updated    map[string]string
	validUntil time.Time
	updates    int
}

type historyVerificationRepository struct {
	*verificationRepositoryStub
	services []domain.Service
	marked   []string
}

func (s *historyVerificationRepository) ListServices() []domain.Service { return s.services }
func (s *historyVerificationRepository) MarkMailboxServiceConsumed(_, serviceID string, _ time.Time) error {
	s.marked = append(s.marked, serviceID)
	return nil
}

func (s *verificationRepositoryStub) GetMailboxCredential(string) (store.MailboxCredential, error) {
	return s.credential, nil
}

func (s *verificationRepositoryStub) UpdateMailboxCredential(_, _ string, credential map[string]string, validUntil time.Time, _ string) error {
	s.updated = credential
	s.validUntil = validUntil
	s.updates++
	return nil
}

func (s *verificationRepositoryStub) UpdateMailboxVerification(_, _, method, status, message string, _ time.Time, _ string) error {
	s.method, s.status, s.message = method, status, message
	return nil
}

type graphConnectorStub struct {
	refreshErr  error
	profileErr  error
	messagesErr error
	allMessages []Message
	allErr      error
	profile     Profile
	calls       int
	refreshes   int
	allCalls    int
}

type oauthRefresherStub struct {
	refreshes int
}

func (s *oauthRefresherStub) RefreshCredential(context.Context, map[string]string) (map[string]string, time.Time, error) {
	s.refreshes++
	validUntil := time.Now().Add(time.Hour)
	return map[string]string{"access_token": "refreshed-google-access", "expires_at": validUntil.Format(time.RFC3339)}, validUntil, nil
}

func TestMailboxVerifierRefreshesExpiredGmailOAuthBeforeIMAP(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "gmail-mailbox", Address: "user@gmail.com", Provider: domain.MailboxProviderGmail},
		Config:  map[string]string{"access_token": "expired-google-access", "refresh_token": "google-refresh", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)},
	}}
	refresher := &oauthRefresherStub{}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, nil, imap, refresher).Verify(context.Background(), "system", "gmail-mailbox", "")
	if err != nil || result.Method != domain.MailboxConnectionIMAP || refresher.refreshes != 1 || imap.calls != 1 {
		t.Fatalf("Gmail OAuth 过期后未刷新并验证：result=%+v err=%v refreshes=%d imap=%d", result, err, refresher.refreshes, imap.calls)
	}
	if imap.credentials[0]["access_token"] != "refreshed-google-access" || repository.updates != 1 {
		t.Fatalf("刷新后的 Gmail Token 未用于 IMAP：credential=%v updates=%d", imap.credentials[0], repository.updates)
	}
}

func (s *graphConnectorStub) RefreshCredential(context.Context, map[string]string) (map[string]string, time.Time, error) {
	s.refreshes++
	if s.refreshErr != nil {
		return nil, time.Time{}, s.refreshErr
	}
	return map[string]string{"access_token": "new-access", "refresh_token": "new-refresh", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}, time.Now().Add(time.Hour), nil
}

func TestMailboxVerifierUsesAccessTokenWithoutExpiryBeforeRefresh(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.de"},
		Config:  map[string]string{"access_token": "access"},
	}}
	graph := &graphConnectorStub{profile: Profile{Address: "user@outlook.de"}}
	imap := &imapConnectorStub{}

	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", "mailbox-1", "")
	if err != nil {
		t.Fatalf("现有 Access Token 验证失败：%v", err)
	}
	if result.Method != domain.MailboxConnectionMicrosoftGraph || graph.refreshes != 0 || imap.calls != 0 {
		t.Fatalf("未优先使用现有 Access Token：result=%+v refreshes=%d imap=%d", result, graph.refreshes, imap.calls)
	}
	if repository.updates != 1 || !repository.validUntil.After(time.Now().Add(14*time.Minute)) {
		t.Fatalf("无过期时间的 Access Token 未持久化保守有效期：updates=%d valid_until=%s", repository.updates, repository.validUntil)
	}
}

func TestMailboxVerifierCompactsInvalidGrantAndFallsBackToIMAP(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-invalid-grant", Address: "user@outlook.com"},
		Config:  map[string]string{"refresh_token": "expired-refresh", "password": "imap-password"},
	}}
	graph := &graphConnectorStub{refreshErr: errors.New(`Microsoft Token 接口返回 400: {"error":"invalid_grant","error_description":"AADSTS70000"}`)}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", "mailbox-invalid-grant", "")
	if err != nil || result.Method != domain.MailboxConnectionIMAP || result.Status != domain.MailboxVerificationVerified || imap.calls != 1 {
		t.Fatalf("invalid_grant 未正确回退 IMAP：result=%+v err=%v imap=%d", result, err, imap.calls)
	}
}

func TestCompactGraphErrorExplainsInvalidGrant(t *testing.T) {
	message := compactGraphError(errors.New(`Microsoft Token 接口返回 400: {"error":"invalid_grant","error_description":"AADSTS70000"}`))
	if !strings.Contains(message, "重新授权 Graph") || strings.Contains(message, "AADSTS70000") {
		t.Fatalf("invalid_grant 未转为可操作提示：%q", message)
	}
}

func (s *graphConnectorStub) Profile(context.Context, string) (Profile, error) {
	s.calls++
	return s.profile, s.profileErr
}

func (s *graphConnectorStub) Messages(context.Context, string) ([]Message, error) {
	return []Message{}, s.messagesErr
}

func (s *graphConnectorStub) AllMessages(context.Context, string) ([]Message, error) {
	s.allCalls++
	return s.allMessages, s.allErr
}

type imapConnectorStub struct {
	err         error
	calls       int
	credentials []map[string]string
	allMessages []Message
	allErr      error
	allCalls    int
}

func (s *imapConnectorStub) Verify(_ context.Context, _ string, credential map[string]string) error {
	s.calls++
	s.credentials = append(s.credentials, credential)
	return s.err
}

func (s *imapConnectorStub) AllMessages(_ context.Context, _ string, credential map[string]string) ([]Message, error) {
	s.allCalls++
	s.credentials = append(s.credentials, credential)
	return s.allMessages, s.allErr
}

func (s *imapConnectorStub) Messages(_ context.Context, _ string, credential map[string]string) ([]Message, error) {
	s.credentials = append(s.credentials, credential)
	return s.allMessages, s.allErr
}

func TestMailboxVerifierFallsBackToPasswordWhenGraphTokenIsInvalid(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-password-fallback", Address: "user@outlook.com"},
		Config:  map[string]string{"access_token": "expired-access", "refresh_token": "expired-refresh", "password": "app-password", "expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)},
	}}
	graph := &graphConnectorStub{refreshErr: errors.New(`Microsoft Token 接口返回 400: {"error":"invalid_grant"}`)}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err != nil || result.Method != domain.MailboxConnectionIMAP || imap.calls != 1 {
		t.Fatalf("Graph 失效后没有成功回退密码登录：result=%+v err=%v calls=%d", result, err, imap.calls)
	}
	if len(imap.credentials) != 1 || imap.credentials[0]["access_token"] != "" || imap.credentials[0]["password"] != "app-password" {
		t.Fatalf("IMAP 回退仍携带失效 access_token：%+v", imap.credentials)
	}
}

func TestMailboxVerifierReadsMessagesWithGraphPriorityAndIMAPFallback(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
		Config:  map[string]string{"access_token": "access", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339), "password": "secret"},
	}}
	graph := &graphConnectorStub{allMessages: []Message{{ID: "graph-message"}}}
	imap := &imapConnectorStub{allMessages: []Message{{ID: "imap-message"}}}
	verifier := NewMailboxVerifier(repository, graph, imap)

	messages, err := verifier.ReadMessages(context.Background(), "admin", "mailbox-1", "")
	if err != nil || len(messages) != 1 || messages[0].ID != "graph-message" || graph.allCalls != 1 || imap.allCalls != 0 {
		t.Fatalf("Graph 收件读取结果错误：messages=%+v err=%v graph_calls=%d imap_calls=%d", messages, err, graph.allCalls, imap.allCalls)
	}

	graph.allErr = errors.New("Graph Mail.Read 不可用")
	messages, err = verifier.ReadMessages(context.Background(), "admin", "mailbox-1", "")
	if err != nil || len(messages) != 1 || messages[0].ID != "imap-message" || imap.allCalls != 1 {
		t.Fatalf("Graph 失败后的 IMAP 回退错误：messages=%+v err=%v imap_calls=%d", messages, err, imap.allCalls)
	}
	if len(imap.credentials) != 1 || imap.credentials[0]["access_token"] != "" || imap.credentials[0]["password"] != "secret" {
		t.Fatalf("收件回退仍携带失效 access_token：%+v", imap.credentials)
	}
}

func TestMailboxVerifierUsesIMAPDirectlyForNonMicrosoftProviders(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@gmail.com", Provider: domain.MailboxProviderGmail},
		Config:  map[string]string{"access_token": "google-access", "password": "app-password"},
	}}
	graph := &graphConnectorStub{allMessages: []Message{{ID: "wrong-graph-message"}}}
	imap := &imapConnectorStub{allMessages: []Message{{ID: "imap-message"}}}
	verifier := NewMailboxVerifier(repository, graph, imap)

	result, err := verifier.Verify(context.Background(), "admin", "mailbox-1", "")
	if err != nil || result.Method != domain.MailboxConnectionIMAP || graph.calls != 0 || graph.refreshes != 0 || imap.calls != 1 {
		t.Fatalf("非 Microsoft 邮箱验证通道路由错误：result=%+v err=%v graph=%d refresh=%d imap=%d", result, err, graph.calls, graph.refreshes, imap.calls)
	}
	messages, err := verifier.ReadMessages(context.Background(), "admin", "mailbox-1", "")
	if err != nil || len(messages) != 1 || messages[0].ID != "imap-message" || graph.allCalls != 0 || imap.allCalls != 1 {
		t.Fatalf("非 Microsoft 邮箱收件通道路由错误：messages=%+v err=%v graph=%d imap=%d", messages, err, graph.allCalls, imap.allCalls)
	}
}

func TestIMAPServerConfigForSupportedProviders(t *testing.T) {
	tests := []struct {
		address string
		server  string
	}{
		{address: "user@outlook.com", server: "outlook.office365.com:993"},
		{address: "user@gmail.com", server: "imap.gmail.com:993"},
		{address: "user@icloud.com", server: "imap.mail.me.com:993"},
		{address: "user@mail.com", server: "imap.mail.com:993"},
	}
	for _, test := range tests {
		config, err := imapServerConfig(test.address)
		if err != nil || config.Address != test.server {
			t.Fatalf("邮箱 %s 的 IMAP 配置 = %+v, %v，期望 %s", test.address, config, err, test.server)
		}
	}
}

func TestMailboxVerifierScansHistoryAfterConnectionVerification(t *testing.T) {
	repository := &historyVerificationRepository{
		verificationRepositoryStub: &verificationRepositoryStub{credential: store.MailboxCredential{
			Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
			Config:  map[string]string{"access_token": "access", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)},
		}},
		services: []domain.Service{{ID: "service-adobe", SenderDomains: []string{"adobe.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`}},
	}
	graph := &graphConnectorStub{allMessages: []Message{{Sender: "noreply@adobe.com", Subject: "Verification code", BodyPreview: "628419"}}}
	verifier := NewMailboxVerifier(repository, graph, &imapConnectorStub{})

	matched, err := verifier.ScanMailboxHistory(context.Background(), "system", "mailbox-1", "")
	if err != nil || matched != 1 || len(repository.marked) != 1 || repository.marked[0] != "service-adobe" {
		t.Fatalf("历史收件扫描错误：matched=%d marked=%v err=%v", matched, repository.marked, err)
	}
}

func TestMailboxVerifierPrefersGraph(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
		Config:  map[string]string{"access_token": "access", "expires_at": expiresAt.Format(time.RFC3339)},
	}}
	graph := &graphConnectorStub{profile: Profile{Address: "user@outlook.com"}}
	imap := &imapConnectorStub{}
	verifier := NewMailboxVerifier(repository, graph, imap)

	result, err := verifier.Verify(context.Background(), "admin", "mailbox-1", "127.0.0.1")
	if err != nil {
		t.Fatalf("验证失败：%v", err)
	}
	if result.Method != domain.MailboxConnectionMicrosoftGraph || repository.status != domain.MailboxVerificationVerified {
		t.Fatalf("Graph 验证状态错误：result=%+v repository=%+v", result, repository)
	}
	if imap.calls != 0 {
		t.Fatalf("Graph 成功后仍调用了 IMAP：%d", imap.calls)
	}
	if repository.updates != 1 || !repository.validUntil.Equal(expiresAt) {
		t.Fatalf("现有 Graph Token 有效期未持久化：updates=%d valid_until=%s", repository.updates, repository.validUntil)
	}
}

func TestMailboxVerifierFallsBackToIMAP(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@hotmail.com"},
		Config:  map[string]string{"password": "secret"},
	}}
	graph := &graphConnectorStub{}
	imap := &imapConnectorStub{}
	verifier := NewMailboxVerifier(repository, graph, imap)

	result, err := verifier.Verify(context.Background(), "system", "mailbox-1", "")
	if err != nil {
		t.Fatalf("IMAP 回退失败：%v", err)
	}
	if result.Method != domain.MailboxConnectionIMAP || imap.calls != 1 || repository.status != domain.MailboxVerificationVerified {
		t.Fatalf("IMAP 回退状态错误：result=%+v calls=%d repository=%+v", result, imap.calls, repository)
	}
}

func TestMailboxVerifierFallsBackWhenGraphCannotReadInbox(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
		Config: map[string]string{
			"access_token": "access",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
			"password":     "secret",
		},
	}}
	graph := &graphConnectorStub{profile: Profile{Address: "user@outlook.com"}, messagesErr: errors.New("Mail.Read 权限不足")}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", "mailbox-1", "")
	if err != nil {
		t.Fatalf("Graph 收件失败后没有回退 IMAP：%v", err)
	}
	if result.Method != domain.MailboxConnectionIMAP || imap.calls != 1 {
		t.Fatalf("回退连接方式错误：result=%+v calls=%d", result, imap.calls)
	}
}

func TestMailboxVerifierStoresCombinedFailureWithoutCredential(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@hotmail.com"},
		Config:  map[string]string{"password": "secret"},
	}}
	verifier := NewMailboxVerifier(repository, &graphConnectorStub{}, &imapConnectorStub{err: errors.New("authentication failed")})

	_, err := verifier.Verify(context.Background(), "system", "mailbox-1", "")
	if err == nil || repository.status != domain.MailboxVerificationFailed {
		t.Fatalf("双通道失败未持久化：err=%v repository=%+v", err, repository)
	}
	if repository.message == "" || repository.message == "secret" {
		t.Fatalf("失败原因无效：%q", repository.message)
	}
}
