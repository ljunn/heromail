package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
	"github.com/ljunn/heromail/internal/mail"
	"github.com/ljunn/heromail/internal/store"
)

type orderMessagesRepository struct {
	store.Repository
	store.ResourceRepository
	credential store.MailboxCredential
}

func (s *orderMessagesRepository) GetMailboxCredential(mailboxID string) (store.MailboxCredential, error) {
	if mailboxID != s.credential.Mailbox.ID {
		return store.MailboxCredential{}, store.ErrMailboxNotFound
	}
	return s.credential, nil
}

func (*orderMessagesRepository) UpdateMailboxCredential(string, string, map[string]string, time.Time, string) error {
	return nil
}

func (*orderMessagesRepository) UpdateMailboxVerification(string, string, string, string, string, time.Time, string) error {
	return nil
}

type orderMessagesGraph struct {
	messages []mail.Message
	calls    int
}

func (*orderMessagesGraph) RefreshCredential(context.Context, map[string]string) (map[string]string, time.Time, error) {
	return nil, time.Time{}, nil
}

func (*orderMessagesGraph) Profile(context.Context, string) (mail.Profile, error) {
	return mail.Profile{}, nil
}

func (s *orderMessagesGraph) Messages(context.Context, string) ([]mail.Message, error) {
	s.calls++
	return s.messages, nil
}

func (s *orderMessagesGraph) AllMessages(context.Context, string) ([]mail.Message, error) {
	s.calls++
	return s.messages, nil
}

func TestUserOrderMessagesAreOwnerScopedAndPlatformIsolated(t *testing.T) {
	base := store.New()
	order, err := base.CreateOrder("user-001", "svc-openai", "order-messages", []string{domain.MailboxProviderOutlook})
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	repository := &orderMessagesRepository{
		Repository: base,
		credential: store.MailboxCredential{
			Mailbox: domain.Mailbox{ID: order.MailboxID, Address: order.MailboxAddress},
			Config:  map[string]string{"access_token": "test-access", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339)},
		},
	}
	graph := &orderMessagesGraph{messages: []mail.Message{
		{ID: "openai-1", Sender: "noreply@openai.com", Subject: "Your verification code", Body: "OpenAI message one", ReceivedAt: order.AssignedAt.Add(time.Minute)},
		{ID: "openai-2", Sender: "noreply@openai.com", Subject: "Your code", Body: "OpenAI message two", ReceivedAt: order.AssignedAt.Add(2 * time.Minute)},
		{ID: "adobe", Sender: "noreply@adobe.com", Subject: "Verification code", Body: "adobe secret", ReceivedAt: order.AssignedAt.Add(3 * time.Minute)},
		{ID: "welcome", Sender: "noreply@openai.com", Subject: "Welcome", Body: "unrelated openai mail", ReceivedAt: order.AssignedAt.Add(4 * time.Minute)},
		{ID: "old", Sender: "noreply@openai.com", Subject: "Verification code", Body: "old openai mail", ReceivedAt: order.AssignedAt.Add(-2 * time.Minute)},
	}}
	server := NewServer(repository)
	server.MailboxVerifier = mail.NewMailboxVerifier(repository, graph, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+order.ID+"/messages?page=1&page_size=1", nil)
	request.Header.Set("X-HeroMail-User", "user-001")
	response := httptest.NewRecorder()
	server.Router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("读取订单邮件返回 %d，响应：%s", response.Code, response.Body.String())
	}
	var body struct {
		Data       []mail.Message `json:"data"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析订单邮件响应失败：%v", err)
	}
	if len(body.Data) != 1 || body.Pagination.Total != 2 {
		t.Fatalf("订单邮件分页结果错误：data=%d total=%d", len(body.Data), body.Pagination.Total)
	}
	responseText := response.Body.String()
	for _, secret := range []string{"adobe secret", "unrelated openai mail", "old openai mail"} {
		if strings.Contains(responseText, secret) {
			t.Fatalf("订单邮件响应泄露了隔离范围外的正文 %q", secret)
		}
	}

	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+order.ID+"/messages", nil)
	foreignRequest.Header.Set("X-HeroMail-User", "user-002")
	foreignResponse := httptest.NewRecorder()
	server.Router.ServeHTTP(foreignResponse, foreignRequest)
	if foreignResponse.Code != http.StatusNotFound {
		t.Fatalf("其他用户读取订单邮件返回 %d，期望 %d", foreignResponse.Code, http.StatusNotFound)
	}
	if graph.calls != 1 {
		t.Fatalf("越权请求不应读取邮箱，Graph 调用次数=%d", graph.calls)
	}
}
