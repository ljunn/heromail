package mail

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ljunn/heromail/internal/domain"
)

func TestMailboxLineParserSupportsConfiguredProviderFormats(t *testing.T) {
	clientID := "00000000-0000-0000-0000-000000000001"
	tests := []struct {
		name         string
		line         string
		address      string
		provider     string
		password     string
		clientID     string
		refreshToken string
		totpURL      string
	}{
		{name: "冒号", line: "alpha@outlook.com:secret", address: "alpha@outlook.com", provider: "outlook", password: "secret"},
		{name: "德国 Outlook", line: "alpha@outlook.de:secret", address: "alpha@outlook.de", provider: "outlook_de", password: "secret"},
		{name: "四横线", line: "beta@hotmail.com----secret", address: "beta@hotmail.com", provider: "hotmail", password: "secret"},
		{name: "竖线", line: "gamma@outlook.com|secret", address: "gamma@outlook.com", provider: "outlook", password: "secret"},
		{name: "CSV", line: fmt.Sprintf("delta@hotmail.com,secret,%s,refresh-value", clientID), address: "delta@hotmail.com", provider: "hotmail", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "交换字段顺序", line: fmt.Sprintf("echo@outlook.com----secret----refresh-value----%s", clientID), address: "echo@outlook.com", provider: "outlook", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "JSON Lines", line: fmt.Sprintf(`{"email":"foxtrot@hotmail.com","password":"secret","client_id":"%s","refresh_token":"refresh-value"}`, clientID), address: "foxtrot@hotmail.com", provider: "hotmail", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "Gmail 应用密码", line: "gmail@gmail.com:app-password", address: "gmail@gmail.com", provider: "gmail", password: "app-password"},
		{name: "Gmail 账号密码和 2FA", line: "twofactor-test@gmail.com----test-password----https://2fa.live/tok/test-token", address: "twofactor-test@gmail.com", provider: "gmail", password: "test-password", totpURL: "https://2fa.live/tok/test-token"},
		{name: "iCloud 应用密码", line: "apple@icloud.com----app-password", address: "apple@icloud.com", provider: "icloud", password: "app-password"},
		{name: "Mail.com 应用密码", line: "mailbox@mail.com|app-password", address: "mailbox@mail.com", provider: "mailcom", password: "app-password"},
	}
	parser := NewMailboxLineParser()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := parser.Parse(test.line)
			if err != nil {
				t.Fatalf("解析失败：%v", err)
			}
			if record.Address != test.address || record.Provider != test.provider {
				t.Fatalf("邮箱识别错误：%+v", record)
			}
			credential := record.Credential()
			if credential["password"] != test.password || credential["client_id"] != test.clientID || credential["refresh_token"] != test.refreshToken || credential["totp_url"] != test.totpURL {
				t.Fatalf("凭证字段识别错误：%+v", credential)
			}
		})
	}
}

func TestMailboxLineParserMarksLegacyMicrosoftOAuthAsIMAP(t *testing.T) {
	line := `{"email":"legacy@outlook.com","client_id":"00000000-0000-0000-0000-000000000001","refresh_token":"refresh-value"}`
	record, err := NewMailboxLineParser().Parse(line)
	if err != nil {
		t.Fatalf("解析旧 Microsoft OAuth 记录失败：%v", err)
	}
	if record.ConnectionMethod != domain.MailboxConnectionAuto || record.OAuthProtocol != "imap" {
		t.Fatalf("旧 Microsoft OAuth 未进入自动探测：%+v", record)
	}
	if record.Credential()["oauth_protocol"] != "imap" {
		t.Fatalf("导入凭据未保留 OAuth 协议：%v", record.Credential())
	}
}

func TestMailboxLineParserSupportsAdobeRegisterJSONFields(t *testing.T) {
	line := `{"email":"source@outlook.com","password":"secret","clientId":"00000000-0000-0000-0000-000000000001","refreshToken":"refresh-value","imapUser":"source-login@outlook.com"}`
	record, err := NewMailboxLineParser().Parse(line)
	if err != nil {
		t.Fatalf("解析源项目 JSON 记录失败：%v", err)
	}
	credential := record.Credential()
	if credential["client_id"] == "" || credential["refresh_token"] != "refresh-value" || credential["imap_user"] != "source-login@outlook.com" {
		t.Fatalf("源项目驼峰字段没有正确转换：%v", credential)
	}
	caseInsensitive := `{"EMAIL":"case@outlook.com","PASSWORD":"secret","CLIENTID":"00000000-0000-0000-0000-000000000001","REFRESHTOKEN":"refresh-value","IMAPUSER":"case-login@outlook.com"}`
	record, err = NewMailboxLineParser().Parse(caseInsensitive)
	if err != nil {
		t.Fatalf("大小写不一致的源项目 JSON 记录解析失败：%v", err)
	}
	credential = record.Credential()
	if record.Address != "case@outlook.com" || credential["client_id"] == "" || credential["refresh_token"] != "refresh-value" || credential["imap_user"] != "case-login@outlook.com" {
		t.Fatalf("大小写不一致的源项目字段没有正确转换：%v", credential)
	}
}

func TestMailboxLineParserRejectsUnsupportedProviderWithoutLeakingLine(t *testing.T) {
	line := "someone@example.com:very-secret-password"
	_, err := NewMailboxLineParser().Parse(line)
	if err == nil || !strings.Contains(err.Error(), "不支持该邮箱类型") {
		t.Fatalf("未拒绝不支持的邮箱：%v", err)
	}
	if strings.Contains(err.Error(), "very-secret-password") {
		t.Fatalf("错误信息泄露了凭证：%v", err)
	}
}

func TestStreamMailboxImportReadsLineByLine(t *testing.T) {
	const total = 10_000
	var input strings.Builder
	for index := 0; index < total; index++ {
		fmt.Fprintf(&input, "mail-%d@outlook.com:secret-%d\n", index, index)
	}
	seen := 0
	result, err := StreamMailboxImport(strings.NewReader(input.String()), NewMailboxLineParser(), func(record MailboxImportRecord) error {
		seen++
		if record.Address == "" || record.Credential()["password"] == "" {
			t.Fatal("回调收到空记录")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("流式导入失败：%v", err)
	}
	if seen != total || result.Imported != total || result.Failed != 0 {
		t.Fatalf("导入统计错误：seen=%d result=%+v", seen, result)
	}
}
