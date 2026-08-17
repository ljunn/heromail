package mail

import (
	"net/url"
	"strings"
	"testing"

	"github.com/ljunn/heromail/internal/domain"
)

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

func TestMatchCodeRequiresAllRules(t *testing.T) {
	service := domain.Service{SenderDomains: []string{"github.com"}, SubjectKeywords: []string{"verification", "验证码"}, Regex: `\b(\d{6})\b`}
	tests := []struct {
		name    string
		message Message
		code    string
		matched bool
	}{
		{name: "正常邮件", message: Message{Sender: "noreply@github.com", Subject: "Verification code", BodyPreview: "Your code is 628419"}, code: "628419", matched: true},
		{name: "子域发件人", message: Message{Sender: "noreply@mail.github.com", Subject: "验证码", BodyPreview: "123456"}, code: "123456", matched: true},
		{name: "伪造域名", message: Message{Sender: "noreply@github.com.example.org", Subject: "Verification code", BodyPreview: "123456"}, matched: false},
		{name: "主题不匹配", message: Message{Sender: "noreply@github.com", Subject: "Welcome", BodyPreview: "123456"}, matched: false},
		{name: "验证码格式不匹配", message: Message{Sender: "noreply@github.com", Subject: "Verification code", BodyPreview: "ABCDEF"}, matched: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, matched := matchCode(service, test.message)
			if matched != test.matched || code != test.code {
				t.Fatalf("匹配结果 = (%q, %v)，期望 (%q, %v)", code, matched, test.code, test.matched)
			}
		})
	}
}
