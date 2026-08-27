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

type microsoftIMAPGraphConnectorStub struct {
	graphConnectorStub
	imapRefreshes  int
	imapRefreshErr error
}

func (s *microsoftIMAPGraphConnectorStub) RefreshIMAPCredential(context.Context, map[string]string) (map[string]string, time.Time, error) {
	s.imapRefreshes++
	if s.imapRefreshErr != nil {
		return nil, time.Time{}, s.imapRefreshErr
	}
	validUntil := time.Now().Add(time.Hour)
	return map[string]string{"access_token": "imap-access", "refresh_token": "imap-refresh", "expires_at": validUntil.Format(time.RFC3339)}, validUntil, nil
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

func TestMailboxVerifierDoesNotReuseGraphTokenAsIMAPFallback(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-invalid-grant-no-password", Address: "user@outlook.com"},
		Config:  map[string]string{"refresh_token": "expired-refresh"},
	}}
	graph := &graphConnectorStub{refreshErr: errors.New(`Microsoft Token 接口返回 400: {"error":"invalid_grant"}`)}
	imap := &imapConnectorStub{}
	_, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err == nil || imap.calls != 0 {
		t.Fatalf("Graph OAuth 失效且没有密码时不应伪造 IMAP 回退：err=%v calls=%d", err, imap.calls)
	}
	if !strings.Contains(repository.message, "重新授权 Graph") {
		t.Fatalf("失败原因没有引导重新授权：%q", repository.message)
	}
}

func TestMailboxVerifierUsesIMAPForImportedMicrosoftIMAPOAuth(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-imported-imap", Address: "user@outlook.com"},
		Config:  map[string]string{"client_id": "source-client", "refresh_token": "source-refresh"},
	}}
	graph := &microsoftIMAPGraphConnectorStub{}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err != nil || result.Method != domain.MailboxConnectionIMAP || graph.imapRefreshes != 1 || graph.refreshes != 0 || imap.calls != 1 {
		t.Fatalf("导入的 Microsoft OAuth 未直接使用 IMAP：result=%+v err=%v imap_refreshes=%d graph_refreshes=%d imap=%d", result, err, graph.imapRefreshes, graph.refreshes, imap.calls)
	}
	if imap.credentials[0]["access_token"] != "imap-access" {
		t.Fatalf("刷新后的 IMAP Token 未传给连接器：%v", imap.credentials[0])
	}
}

func TestMailboxVerifierFallsBackToPasswordWhenImportedIMAPOAuthFails(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-imported-oauth-password", Address: "user@outlook.com"},
		Config: map[string]string{
			"oauth_protocol": "imap",
			"client_id":      "source-client",
			"refresh_token":  "source-refresh",
			"access_token":   "oauth-access",
			"expires_at":     time.Now().Add(time.Hour).Format(time.RFC3339),
			"password":       "working-password",
		},
	}}
	imap := &imapConnectorStub{verifyErrors: []error{errors.New("OAuth 认证失败"), nil}}
	result, err := NewMailboxVerifier(repository, nil, imap).Verify(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err != nil || result.Status != domain.MailboxVerificationVerified || result.Method != domain.MailboxConnectionIMAP {
		t.Fatalf("导入 OAuth 失败后未回退密码：result=%+v err=%v", result, err)
	}
	if imap.calls != 2 || imap.credentials[1]["access_token"] != "" || imap.credentials[1]["password"] != "working-password" {
		t.Fatalf("未按 OAuth、密码顺序尝试两种登录方式：calls=%d credentials=%v", imap.calls, imap.credentials)
	}
}

func TestMailboxVerifierKeepsPersistedPasswordRoute(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-imported-password-route", Address: "user@outlook.com"},
		Config: map[string]string{
			"oauth_protocol":   "imap",
			"client_id":        "source-client",
			"refresh_token":    "source-refresh",
			"access_token":     "stale-oauth-access",
			"imap_auth_method": "password",
			"password":         "working-password",
		},
	}}
	graph := &graphConnectorStub{profileErr: errors.New("不应访问 Graph")}
	imap := &imapConnectorStub{}
	result, err := NewMailboxVerifier(repository, graph, imap).Verify(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err != nil || result.Status != domain.MailboxVerificationVerified || graph.calls != 0 || imap.calls != 1 {
		t.Fatalf("已确认密码的邮箱未固定走 IMAP：result=%+v err=%v graph=%d imap=%d", result, err, graph.calls, imap.calls)
	}
	if imap.credentials[0]["access_token"] != "" || imap.credentials[0]["password"] != "working-password" {
		t.Fatalf("密码路由仍携带 OAuth Token：%v", imap.credentials[0])
	}
}

func TestMailboxVerifierReadsMessagesWithImportedIMAPOAuthPasswordFallback(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-imported-oauth-password-read", Address: "user@outlook.com"},
		Config: map[string]string{
			"oauth_protocol": "imap",
			"client_id":      "source-client",
			"refresh_token":  "source-refresh",
			"access_token":   "oauth-access",
			"expires_at":     time.Now().Add(time.Hour).Format(time.RFC3339),
			"password":       "working-password",
		},
	}}
	imap := &imapConnectorStub{allMessages: []Message{{ID: "password-message"}}, allErrors: []error{errors.New("OAuth 收件失败"), nil}}
	messages, err := NewMailboxVerifier(repository, nil, imap).ReadMessages(context.Background(), "system", repository.credential.Mailbox.ID, "")
	if err != nil || len(messages) != 1 || messages[0].ID != "password-message" {
		t.Fatalf("收件读取未回退密码：messages=%+v err=%v", messages, err)
	}
	if imap.allCalls != 2 || imap.credentials[1]["access_token"] != "" || imap.credentials[1]["password"] != "working-password" {
		t.Fatalf("收件读取未按 OAuth、密码顺序尝试：calls=%d credentials=%v", imap.allCalls, imap.credentials)
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
	err          error
	verifyErrors []error
	calls        int
	credentials  []map[string]string
	allMessages  []Message
	allErr       error
	allErrors    []error
	allCalls     int
}

func (s *imapConnectorStub) Verify(_ context.Context, _ string, credential map[string]string) error {
	s.calls++
	s.credentials = append(s.credentials, credential)
	if len(s.verifyErrors) > 0 {
		err := s.verifyErrors[0]
		s.verifyErrors = s.verifyErrors[1:]
		return err
	}
	return s.err
}

func (s *imapConnectorStub) AllMessages(_ context.Context, _ string, credential map[string]string) ([]Message, error) {
	s.allCalls++
	s.credentials = append(s.credentials, credential)
	if len(s.allErrors) > 0 {
		err := s.allErrors[0]
		s.allErrors = s.allErrors[1:]
		return s.allMessages, err
	}
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

func TestIMAPUsernameUsesImportedSourceValue(t *testing.T) {
	if got := imapUsername("source@outlook.com", map[string]string{"imap_user": "login@outlook.com"}); got != "login@outlook.com" {
		t.Fatalf("未使用导入的 IMAP 用户名：%q", got)
	}
	if got := imapUsername("source@outlook.com", nil); got != "source@outlook.com" {
		t.Fatalf("缺少 IMAP 用户名时应回退邮箱地址：%q", got)
	}
	client := newXOAuth2Client(imapUsername("source@outlook.com", map[string]string{"imap_user": "login@outlook.com"}), "access-token")
	mechanism, response, err := client.Start()
	if err != nil || mechanism != "XOAUTH2" || string(response) != "user=login@outlook.com\x01auth=Bearer access-token\x01\x01" {
		t.Fatalf("XOAUTH2 未使用源项目的 IMAP 用户名：mechanism=%q response=%q err=%v", mechanism, response, err)
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
