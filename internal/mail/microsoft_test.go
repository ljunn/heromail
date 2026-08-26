package mail

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/store"
)

type pagingWorkerRepository struct {
	mailboxes []store.MailboxCredential
	pages     int
}

func (s *pagingWorkerRepository) ListMailboxCredentialsPage(afterID string, limit int) ([]store.MailboxCredential, error) {
	s.pages++
	start := 0
	for start < len(s.mailboxes) && s.mailboxes[start].Mailbox.ID <= afterID {
		start++
	}
	end := start + limit
	if end > len(s.mailboxes) {
		end = len(s.mailboxes)
	}
	return s.mailboxes[start:end], nil
}

func (*pagingWorkerRepository) UpdateMailboxCredential(string, string, map[string]string, time.Time, string) error {
	return nil
}
func (*pagingWorkerRepository) UpdateMailboxVerification(string, string, string, string, string, time.Time, string) error {
	return nil
}
func (*pagingWorkerRepository) WaitingOrdersForMailbox(string) []domain.Order { return nil }
func (*pagingWorkerRepository) ServiceByID(string) (domain.Service, bool) {
	return domain.Service{}, false
}
func (*pagingWorkerRepository) MarkMailEvent(string, string, string, string, time.Time) (bool, error) {
	return true, nil
}

type codeReceiverStub struct{}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type concurrentPollRepository struct {
	mailboxes []store.MailboxCredential
}

func (s *concurrentPollRepository) ListMailboxCredentialsPage(afterID string, limit int) ([]store.MailboxCredential, error) {
	start := 0
	for start < len(s.mailboxes) && s.mailboxes[start].Mailbox.ID <= afterID {
		start++
	}
	end := start + limit
	if end > len(s.mailboxes) {
		end = len(s.mailboxes)
	}
	return s.mailboxes[start:end], nil
}
func (*concurrentPollRepository) UpdateMailboxCredential(string, string, map[string]string, time.Time, string) error {
	return nil
}
func (*concurrentPollRepository) UpdateMailboxVerification(string, string, string, string, string, time.Time, string) error {
	return nil
}
func (*concurrentPollRepository) WaitingOrdersForMailbox(id string) []domain.Order {
	return []domain.Order{{ID: "order-" + id, ServiceID: "service-openai", Status: domain.OrderWaitingCode, AssignedAt: time.Now().Add(-time.Minute)}}
}
func (*concurrentPollRepository) ServiceByID(string) (domain.Service, bool) {
	return domain.Service{ID: "service-openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`}, true
}
func (*concurrentPollRepository) MarkMailEvent(string, string, string, string, time.Time) (bool, error) {
	return false, nil
}

type concurrentMessageConnector struct {
	active    atomic.Int32
	maxActive atomic.Int32
}

func (*concurrentMessageConnector) Verify(context.Context, string, map[string]string) error {
	return nil
}
func (s *concurrentMessageConnector) Messages(ctx context.Context, _ string, _ map[string]string) ([]Message, error) {
	active := s.active.Add(1)
	for {
		previous := s.maxActive.Load()
		if active <= previous || s.maxActive.CompareAndSwap(previous, active) {
			break
		}
	}
	defer s.active.Add(-1)
	select {
	case <-time.After(20 * time.Millisecond):
		return []Message{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (codeReceiverStub) ReceiveCodeValue(string, string) (domain.Order, error) {
	return domain.Order{}, nil
}

type earlyMailRepository struct {
	order   domain.Order
	service domain.Service
	marked  bool
}

type historyMatchingRepository struct {
	services []domain.Service
	marked   []string
}

func (*historyMatchingRepository) ListMailboxCredentialsPage(string, int) ([]store.MailboxCredential, error) {
	return nil, nil
}
func (*historyMatchingRepository) UpdateMailboxCredential(string, string, map[string]string, time.Time, string) error {
	return nil
}
func (*historyMatchingRepository) UpdateMailboxVerification(string, string, string, string, string, time.Time, string) error {
	return nil
}
func (s *historyMatchingRepository) WaitingOrdersForMailbox(string) []domain.Order { return nil }
func (s *historyMatchingRepository) ServiceByID(id string) (domain.Service, bool) {
	for _, service := range s.services {
		if service.ID == id {
			return service, true
		}
	}
	return domain.Service{}, false
}
func (s *historyMatchingRepository) ListServices() []domain.Service { return s.services }
func (s *historyMatchingRepository) MarkMailboxServiceConsumed(_, serviceID string, _ time.Time) error {
	s.marked = append(s.marked, serviceID)
	return nil
}
func (*historyMatchingRepository) MarkMailEvent(string, string, string, string, time.Time) (bool, error) {
	return true, nil
}

func (*earlyMailRepository) ListMailboxCredentialsPage(string, int) ([]store.MailboxCredential, error) {
	return nil, nil
}
func (*earlyMailRepository) UpdateMailboxCredential(string, string, map[string]string, time.Time, string) error {
	return nil
}
func (*earlyMailRepository) UpdateMailboxVerification(string, string, string, string, string, time.Time, string) error {
	return nil
}
func (s *earlyMailRepository) WaitingOrdersForMailbox(string) []domain.Order {
	return []domain.Order{s.order}
}
func (s *earlyMailRepository) ServiceByID(string) (domain.Service, bool) {
	return s.service, true
}
func (s *earlyMailRepository) MarkMailEvent(string, string, string, string, time.Time) (bool, error) {
	s.marked = true
	return true, nil
}

type messageConnectorStub struct {
	messages []Message
}

func (*messageConnectorStub) Verify(context.Context, string, map[string]string) error { return nil }
func (s *messageConnectorStub) Messages(context.Context, string, map[string]string) ([]Message, error) {
	return s.messages, nil
}

type recordingCodeReceiver struct {
	orderID string
	code    string
}

func (s *recordingCodeReceiver) ReceiveCodeValue(orderID, code string) (domain.Order, error) {
	s.orderID, s.code = orderID, code
	return domain.Order{ID: orderID, Code: code, Status: domain.OrderCodeReceived}, nil
}

type retryingCodeReceiver struct {
	attempts int
}

func (s *retryingCodeReceiver) ReceiveCodeValue(orderID, code string) (domain.Order, error) {
	s.attempts++
	if s.attempts == 1 {
		return domain.Order{}, fmt.Errorf("测试用瞬时数据库错误")
	}
	return domain.Order{ID: orderID, Code: code, Status: domain.OrderCodeReceived}, nil
}

func TestWorkerScansMailboxCredentialsPastFirstPage(t *testing.T) {
	repository := &pagingWorkerRepository{mailboxes: make([]store.MailboxCredential, 205)}
	for index := range repository.mailboxes {
		repository.mailboxes[index].Mailbox.ID = fmt.Sprintf("mailbox-%03d", index)
	}
	worker := NewWorker(repository, codeReceiverStub{}, NewMicrosoftClient(MicrosoftConfig{}), time.Minute)
	worker.poll(context.Background())
	if repository.pages != 3 {
		t.Fatalf("邮箱凭证分页次数 = %d，期望 3", repository.pages)
	}
}

func TestWorkerPollsActiveMailboxesConcurrently(t *testing.T) {
	repository := &concurrentPollRepository{mailboxes: make([]store.MailboxCredential, 6)}
	for index := range repository.mailboxes {
		repository.mailboxes[index] = store.MailboxCredential{Mailbox: domain.Mailbox{ID: fmt.Sprintf("mailbox-%03d", index), Address: fmt.Sprintf("user-%03d@outlook.com", index), ConnectionMethod: domain.MailboxConnectionIMAP}, Config: map[string]string{"password": "test-only"}}
	}
	worker := NewWorkerWithConcurrency(repository, codeReceiverStub{}, NewMicrosoftClient(MicrosoftConfig{}), time.Minute, 3)
	connector := &concurrentMessageConnector{}
	worker.imap = connector
	worker.poll(context.Background())
	if connector.maxActive.Load() < 3 {
		t.Fatalf("活跃邮箱没有并发收码，最大并发数=%d", connector.maxActive.Load())
	}
}

func TestWorkerReceivesMailSentBeforeUserConfirmation(t *testing.T) {
	now := time.Now().UTC()
	repository := &earlyMailRepository{
		order: domain.Order{
			ID:          "order-early-mail",
			ServiceID:   "service-openai",
			Status:      domain.OrderAssigned,
			AssignedAt:  now.Add(-5 * time.Minute),
			SubmittedAt: now,
			ExpiresAt:   now.Add(5 * time.Minute),
		},
		service: domain.Service{ID: "service-openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
	}
	receiver := &recordingCodeReceiver{}
	worker := NewWorker(repository, receiver, NewMicrosoftClient(MicrosoftConfig{}), time.Minute)
	worker.imap = &messageConnectorStub{messages: []Message{{
		ID:          "message-before-click",
		Sender:      "noreply@openai.com",
		Subject:     "Verification code",
		BodyPreview: "Your code is 628419",
		ReceivedAt:  now.Add(-3 * time.Minute),
	}}}

	worker.pollMailbox(context.Background(), store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com", ConnectionMethod: domain.MailboxConnectionIMAP},
		Config:  map[string]string{"password": "test-only"},
	})

	if !repository.marked {
		t.Fatal("用户确认前已到达的邮件没有进入去重记录")
	}
	if receiver.orderID != repository.order.ID || receiver.code != "628419" {
		t.Fatalf("用户确认前已到达的验证码未写入订单：order=%q code=%q", receiver.orderID, receiver.code)
	}
}

func TestWorkerRetriesMailWhenReceivingCodeFails(t *testing.T) {
	now := time.Now().UTC()
	repository := &earlyMailRepository{
		order: domain.Order{
			ID:          "order-retry-mail",
			ServiceID:   "service-openai",
			Status:      domain.OrderWaitingCode,
			AssignedAt:  now.Add(-time.Minute),
			SubmittedAt: now.Add(-time.Minute),
			ExpiresAt:   now.Add(5 * time.Minute),
		},
		service: domain.Service{ID: "service-openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
	}
	receiver := &retryingCodeReceiver{}
	worker := NewWorker(repository, receiver, NewMicrosoftClient(MicrosoftConfig{}), time.Minute)
	worker.imap = &messageConnectorStub{messages: []Message{{
		ID:          "message-retry",
		Sender:      "noreply@openai.com",
		Subject:     "Verification code",
		BodyPreview: "Your code is 628419",
		ReceivedAt:  now,
	}}}
	mailbox := store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-retry", Address: "user@outlook.com", ConnectionMethod: domain.MailboxConnectionIMAP},
		Config:  map[string]string{"password": "test-only"},
	}

	worker.pollMailbox(context.Background(), mailbox)
	if repository.marked {
		t.Fatal("收码失败的邮件不应提前写入去重记录")
	}
	worker.pollMailbox(context.Background(), mailbox)
	if !repository.marked || receiver.attempts != 2 {
		t.Fatalf("收码失败后没有在下一轮重试：marked=%v attempts=%d", repository.marked, receiver.attempts)
	}
}

func TestWorkerSkipsIdleMailboxWithoutActiveOrder(t *testing.T) {
	now := time.Now().UTC()
	repository := &historyMatchingRepository{services: []domain.Service{
		{ID: "service-adobe", Code: "adobe", SenderDomains: []string{"adobe.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
		{ID: "service-openai", Code: "openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
	}}
	worker := NewWorker(repository, codeReceiverStub{}, NewMicrosoftClient(MicrosoftConfig{}), time.Minute)
	worker.imap = &messageConnectorStub{messages: []Message{
		{ID: "adobe-history", Sender: "noreply@adobe.com", Subject: "Verification code", BodyPreview: "Your code is 628419", ReceivedAt: now.Add(-2 * time.Minute)},
		{ID: "openai-history", Sender: "noreply@openai.com", Subject: "Verification code", BodyPreview: "Your code is 314159", ReceivedAt: now.Add(-time.Minute)},
	}}

	worker.pollMailbox(context.Background(), store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-history", Address: "user@outlook.com", ConnectionMethod: domain.MailboxConnectionIMAP},
		Config:  map[string]string{"password": "test-only"},
	})

	if len(repository.marked) != 0 {
		t.Fatalf("空闲邮箱不应由实时收码 Worker 扫描历史邮件：%v", repository.marked)
	}
}

func TestWorkerIgnoresMailBeforeMailboxAssignment(t *testing.T) {
	now := time.Now().UTC()
	repository := &earlyMailRepository{
		order: domain.Order{
			ID:          "order-stale-mail",
			ServiceID:   "service-openai",
			Status:      domain.OrderWaitingCode,
			AssignedAt:  now.Add(-5 * time.Minute),
			SubmittedAt: now,
			ExpiresAt:   now.Add(5 * time.Minute),
		},
		service: domain.Service{ID: "service-openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
	}
	receiver := &recordingCodeReceiver{}
	worker := NewWorker(repository, receiver, NewMicrosoftClient(MicrosoftConfig{}), time.Minute)
	worker.imap = &messageConnectorStub{messages: []Message{{
		ID:          "message-before-assignment",
		Sender:      "noreply@openai.com",
		Subject:     "Verification code",
		BodyPreview: "Your code is 123456",
		ReceivedAt:  now.Add(-10 * time.Minute),
	}}}

	worker.pollMailbox(context.Background(), store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-1", Address: "user@outlook.com", ConnectionMethod: domain.MailboxConnectionIMAP},
		Config:  map[string]string{"password": "test-only"},
	})

	if repository.marked || receiver.orderID != "" {
		t.Fatalf("分配前的历史邮件不应匹配当前订单：marked=%v order=%q", repository.marked, receiver.orderID)
	}
}

func TestMicrosoftAuthorizationURL(t *testing.T) {
	client := NewMicrosoftClient(MicrosoftConfig{ClientID: "client-id", ClientSecret: "client-secret", Tenant: "common", RedirectURI: "https://mail.example.com/callback"})
	parsed, err := url.Parse(client.AuthURL("state-token"))
	if err != nil {
		t.Fatalf("解析授权地址失败：%v", err)
	}
	query := parsed.Query()
	if query.Get("state") != "state-token" || query.Get("redirect_uri") != "https://mail.example.com/callback" {
		t.Fatalf("授权地址参数不正确：%s", parsed.String())
	}
	for _, scope := range []string{"offline_access", "Mail.Read", "User.Read"} {
		if !strings.Contains(query.Get("scope"), scope) {
			t.Fatalf("授权地址缺少权限 %s", scope)
		}
	}
}

func TestRefreshTokenRequestKeepsOriginallyGrantedScopes(t *testing.T) {
	values := refreshTokenValues("client-id", "refresh-token", "client-secret")
	if values.Get("scope") != "" {
		t.Fatalf("refresh_token 请求不应重新声明 scope：%q", values.Get("scope"))
	}
	if values.Get("client_secret") != "client-secret" || values.Get("grant_type") != "refresh_token" {
		t.Fatalf("refresh_token 请求参数不完整：%v", values)
	}
}

func TestWorkerDoesNotRefreshGraphAfterMailboxUsesIMAP(t *testing.T) {
	client := NewMicrosoftClient(MicrosoftConfig{ClientID: "client-id", Tenant: "common"})
	var refreshCalls int
	client.http = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		refreshCalls++
		return nil, fmt.Errorf("不应在 IMAP 通道刷新 Graph")
	})}
	repository := &historyMatchingRepository{services: []domain.Service{{
		ID: "service-openai", SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`,
	}}}
	worker := NewWorker(repository, codeReceiverStub{}, client, time.Minute)
	worker.imap = &messageConnectorStub{}
	worker.pollMailbox(context.Background(), store.MailboxCredential{
		Mailbox: domain.Mailbox{ID: "mailbox-imap", Address: "user@outlook.com", Provider: domain.MailboxProviderOutlook, ConnectionMethod: domain.MailboxConnectionIMAP},
		Config:  map[string]string{"refresh_token": "expired-token", "password": "test-only"},
	})
	if refreshCalls != 0 {
		t.Fatalf("IMAP 通道仍刷新 Graph %d 次", refreshCalls)
	}
}

func TestMatchCodeRequiresAllRules(t *testing.T) {
	service := domain.Service{SenderDomains: []string{"adobe.com"}, SubjectKeywords: []string{"verification", "验证码"}, Regex: `\b(\d{6})\b`}
	tests := []struct {
		name    string
		message Message
		code    string
		matched bool
	}{
		{name: "正常邮件", message: Message{Sender: "noreply@adobe.com", Subject: "Verification code", BodyPreview: "Your code is 628419"}, code: "628419", matched: true},
		{name: "正文验证码", message: Message{Sender: "noreply@adobe.com", Subject: "Verification code", Body: "Your code is 314159"}, code: "314159", matched: true},
		{name: "子域发件人", message: Message{Sender: "noreply@mail.adobe.com", Subject: "验证码", BodyPreview: "123456"}, code: "123456", matched: true},
		{name: "伪造域名", message: Message{Sender: "noreply@adobe.com.example.org", Subject: "Verification code", BodyPreview: "123456"}, matched: false},
		{name: "主题不匹配", message: Message{Sender: "noreply@adobe.com", Subject: "Welcome", BodyPreview: "123456"}, matched: false},
		{name: "没有配置关键词", message: Message{Sender: "noreply@adobe.com", Subject: "Verification code", BodyPreview: "123456"}, matched: false},
		{name: "验证码格式不匹配", message: Message{Sender: "noreply@adobe.com", Subject: "Verification code", BodyPreview: "ABCDEF"}, matched: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testService := service
			if test.name == "没有配置关键词" {
				testService.SubjectKeywords = nil
			}
			code, matched := matchCode(testService, test.message)
			if matched != test.matched || code != test.code {
				t.Fatalf("匹配结果 = (%q, %v)，期望 (%q, %v)", code, matched, test.code, test.matched)
			}
		})
	}
}

func TestMatchCodeUsesObservedGrokAndOpenAIRules(t *testing.T) {
	grok := domain.Service{SenderDomains: []string{"x.ai"}, SubjectKeywords: []string{"validate your email"}, Regex: `(?i)\b([A-Z0-9]{3}-[A-Z0-9]{3}|[A-Z0-9]{6})\b`}
	code, matched := matchCode(grok, Message{Sender: "noreply@x.ai", Subject: "Validate your email", BodyPreview: "Use C1O-6KS to continue"})
	if !matched || code != "C1O-6KS" {
		t.Fatalf("Grok 验证码匹配 = (%q, %v)，期望 (C1O-6KS, true)", code, matched)
	}

	openai := domain.Service{SenderDomains: []string{"openai.com", "tm.openai.com"}, SubjectKeywords: []string{"verification code", "verify your email", "your code"}, Regex: `\b(\d{6})\b`}
	for _, subject := range []string{"Verification code", "Verify your email", "Your code"} {
		if code, ok := matchCode(openai, Message{Sender: "noreply@tm.openai.com", Subject: subject, BodyPreview: "628419"}); !ok || code != "628419" {
			t.Fatalf("OpenAI 标题 %q 未按实际规则匹配：(%q, %v)", subject, code, ok)
		}
	}
	if _, ok := matchCode(openai, Message{Sender: "noreply@openai.com", Subject: "Welcome to OpenAI", BodyPreview: "628419"}); ok {
		t.Fatal("OpenAI 非验证码标题被错误匹配")
	}
}

func TestFilterMessagesForOrderEnforcesServiceAndLeaseWindow(t *testing.T) {
	assignedAt := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	order := domain.Order{AssignedAt: assignedAt, ExpiresAt: assignedAt.Add(10 * time.Minute)}
	service := domain.Service{SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification", "验证码"}, Regex: `\b(\d{6})\b`}
	messages := []Message{
		{ID: "openai-current", Sender: "noreply@openai.com", Subject: "Your verification code", ReceivedAt: assignedAt.Add(time.Minute)},
		{ID: "openai-second-keyword", Sender: "noreply@openai.com", Subject: "OpenAI 验证码", ReceivedAt: assignedAt.Add(2 * time.Minute)},
		{ID: "adobe", Sender: "noreply@adobe.com", Subject: "Verification code", ReceivedAt: assignedAt.Add(3 * time.Minute)},
		{ID: "wrong-subject", Sender: "noreply@openai.com", Subject: "Welcome", ReceivedAt: assignedAt.Add(4 * time.Minute)},
		{ID: "old-openai", Sender: "noreply@openai.com", Subject: "Verification code", ReceivedAt: assignedAt.Add(-2 * time.Minute)},
		{ID: "late-openai", Sender: "noreply@openai.com", Subject: "Verification code", ReceivedAt: order.ExpiresAt.Add(time.Second)},
	}

	filtered := FilterMessagesForOrder(service, order, messages)
	if len(filtered) != 2 || filtered[0].ID != "openai-current" || filtered[1].ID != "openai-second-keyword" {
		t.Fatalf("订单邮件隔离结果错误：%+v", filtered)
	}
}
