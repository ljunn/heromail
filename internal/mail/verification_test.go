package mail

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type verificationRepositoryStub struct {
	credential store.MailboxCredential
	method     string
	status     string
	message    string
	updated    map[string]string
}

func (s *verificationRepositoryStub) GetMailboxCredential(string) (store.MailboxCredential, error) {
	return s.credential, nil
}

func (s *verificationRepositoryStub) UpdateMailboxCredential(_, _ string, credential map[string]string, _ time.Time, _ string) error {
	s.updated = credential
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
	profile     Profile
	calls       int
}

func (s *graphConnectorStub) RefreshCredential(context.Context, map[string]string) (map[string]string, time.Time, error) {
	if s.refreshErr != nil {
		return nil, time.Time{}, s.refreshErr
	}
	return map[string]string{"access_token": "new-access", "refresh_token": "new-refresh", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)}, time.Now().Add(time.Hour), nil
}

func (s *graphConnectorStub) Profile(context.Context, string) (Profile, error) {
	s.calls++
	return s.profile, s.profileErr
}

func (s *graphConnectorStub) Messages(context.Context, string) ([]Message, error) {
	return []Message{}, s.messagesErr
}

type imapConnectorStub struct {
	err   error
	calls int
}

func (s *imapConnectorStub) Verify(context.Context, string, map[string]string) error {
	s.calls++
	return s.err
}

func TestMailboxVerifierPrefersGraph(t *testing.T) {
	repository := &verificationRepositoryStub{credential: store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com"},
		Config:  map[string]string{"access_token": "access", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)},
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
