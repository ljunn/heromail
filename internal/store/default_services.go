package store

import "github.com/ljunn/heromail/internal/domain"

// defaultServices 只用于空库首次初始化；运行和升级过程不得补回管理员删除的平台。
func defaultServices() []domain.Service {
	providers := func() []string { return append([]string(nil), domain.SupportedMailboxProviders...) }
	prices := func(price float64) map[string]float64 {
		result := make(map[string]float64, len(domain.SupportedMailboxProviders))
		for _, provider := range domain.SupportedMailboxProviders {
			result[provider] = price
		}
		return result
	}
	return []domain.Service{
		{ID: "svc-adobe", Code: "adobe", Name: "Adobe", Description: "Adobe 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.60), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"adobe.com"}, SubjectKeywords: []string{"verification code", "verify your email", "adobe code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-imagine", Code: "imagine", Name: "Imagine", Description: "ImagineArt 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.45), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"imagine.art", "vyro.ai"}, SubjectKeywords: []string{"verification", "verify", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-krea", Code: "krea", Name: "Krea", Description: "Krea 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.50), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"krea.ai"}, SubjectKeywords: []string{"verification", "verify", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-leonardo", Code: "leonardo", Name: "Leonardo", Description: "Leonardo.Ai 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.50), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"leonardo.ai"}, SubjectKeywords: []string{"verification", "verify", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-openai", Code: "openai", Name: "OpenAI", Description: "OpenAI 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.60), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"openai.com", "tm.openai.com"}, SubjectKeywords: []string{"verification code", "verify your email", "your code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-runway", Code: "runway", Name: "Runway", Description: "Runway 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.60), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"runwayml.com"}, SubjectKeywords: []string{"verification", "verify", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-grok", Code: "grok", Name: "Grok", Description: "xAI Grok 账户注册", Enabled: true, AllowedProviders: providers(), ProviderPrices: prices(0.60), TTLSeconds: domain.MinimumOrderTTLSeconds, SenderDomains: []string{"x.ai"}, SubjectKeywords: []string{"validate your email"}, Regex: `(?i)\b([A-Z0-9]{3}-[A-Z0-9]{3}|[A-Z0-9]{6})\b`},
	}
}
