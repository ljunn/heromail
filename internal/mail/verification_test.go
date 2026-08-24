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
	allMessages []Message
	allErr      error
	allCalls    int
}

func (s *imapConnectorStub) Verify(context.Context, string, map[string]string) error {
	s.calls++
	return s.err
}

func (s *imapConnectorStub) AllMessages(context.Context, string, map[string]string) ([]Message, error) {
	s.allCalls++
	return s.allMessages, s.allErr
}

func (s *imapConnectorStub) Messages(context.Context, string, map[string]string) ([]Message, error) {
	return s.allMessages, s.allErr
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
}

func TestMailboxVerifierScansHistoryAfterConnectionVerification(t *testing.T) {
	repository := &historyVerificationRepository{
		verificationRepositoryStub: &verificationRepositoryStub{credential: store.MailboxCredential{
			Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
			Config:  map[string]string{"access_token": "access", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)},
		}},
		services: []domain.Service{{ID: "service-github", SenderDomains: []string{"github.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`}},
	}
	graph := &graphConnectorStub{allMessages: []Message{{Sender: "noreply@github.com", Subject: "Verification code", BodyPreview: "628419"}}}
	verifier := NewMailboxVerifier(repository, graph, &imapConnectorStub{})

	matched, err := verifier.ScanMailboxHistory(context.Background(), "system", "mailbox-1", "")
	if err != nil || matched != 1 || len(repository.marked) != 1 || repository.marked[0] != "service-github" {
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
