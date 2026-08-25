package mail

import (
	"fmt"
	"strings"
	"testing"
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
	}{
		{name: "冒号", line: "alpha@outlook.com:secret", address: "alpha@outlook.com", provider: "outlook", password: "secret"},
		{name: "德国 Outlook", line: "alpha@outlook.de:secret", address: "alpha@outlook.de", provider: "outlook_de", password: "secret"},
		{name: "四横线", line: "beta@hotmail.com----secret", address: "beta@hotmail.com", provider: "hotmail", password: "secret"},
		{name: "竖线", line: "gamma@outlook.com|secret", address: "gamma@outlook.com", provider: "outlook", password: "secret"},
		{name: "CSV", line: fmt.Sprintf("delta@hotmail.com,secret,%s,refresh-value", clientID), address: "delta@hotmail.com", provider: "hotmail", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "交换字段顺序", line: fmt.Sprintf("echo@outlook.com----secret----refresh-value----%s", clientID), address: "echo@outlook.com", provider: "outlook", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "JSON Lines", line: fmt.Sprintf(`{"email":"foxtrot@hotmail.com","password":"secret","client_id":"%s","refresh_token":"refresh-value"}`, clientID), address: "foxtrot@hotmail.com", provider: "hotmail", password: "secret", clientID: clientID, refreshToken: "refresh-value"},
		{name: "Gmail 应用密码", line: "gmail@gmail.com:app-password", address: "gmail@gmail.com", provider: "gmail", password: "app-password"},
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
			if credential["password"] != test.password || credential["client_id"] != test.clientID || credential["refresh_token"] != test.refreshToken {
				t.Fatalf("凭证字段识别错误：%+v", credential)
			}
		})
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
