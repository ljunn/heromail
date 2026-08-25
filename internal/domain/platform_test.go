package domain

import "testing"

func TestDetectMailboxProviderSupportsConfiguredChannels(t *testing.T) {
	tests := map[string]string{
		"user@outlook.com":    MailboxProviderOutlook,
		"user@outlook.de":     MailboxProviderOutlookDE,
		"user@hotmail.com":    MailboxProviderHotmail,
		"user@gmail.com":      MailboxProviderGmail,
		"user@googlemail.com": MailboxProviderGmail,
		"user@icloud.com":     MailboxProviderICloud,
		"user@me.com":         MailboxProviderICloud,
		"user@mac.com":        MailboxProviderICloud,
		"user@mail.com":       MailboxProviderMailCom,
	}
	for address, expected := range tests {
		provider, ok := DetectMailboxProvider(address)
		if !ok || provider != expected {
			t.Fatalf("邮箱 %s 识别为 %q, %t，期望 %q", address, provider, ok, expected)
		}
	}
}

func TestDetectMailboxProviderRejectsUnknownDomain(t *testing.T) {
	if provider, ok := DetectMailboxProvider("user@example.com"); ok || provider != "" {
		t.Fatalf("未知邮箱域名被错误识别为 %q", provider)
	}
}
