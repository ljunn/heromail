package domain

import (
	"strings"
	"time"
)

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type WalletLedger struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id,omitempty"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balance_after"`
	OrderID      string    `json:"order_id,omitempty"`
	PaymentID    string    `json:"payment_order_id,omitempty"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuditLog struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Detail       string    `json:"detail"`
	IP           string    `json:"ip"`
	CreatedAt    time.Time `json:"created_at"`
}

type MailboxPool struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	Region         string    `json:"region"`
	Enabled        bool      `json:"enabled"`
	DailyLimit     int       `json:"daily_limit"`
	CooldownSecond int       `json:"cooldown_seconds"`
	MailboxCount   int64     `json:"mailbox_count"`
	CreatedAt      time.Time `json:"created_at"`
}

const (
	DefaultMailboxPoolName   = "邮箱池"
	MailboxProviderOutlook   = "outlook"
	MailboxProviderOutlookDE = "outlook_de"
	MailboxProviderHotmail   = "hotmail"
)

var SupportedMailboxProviders = []string{MailboxProviderOutlook, MailboxProviderOutlookDE, MailboxProviderHotmail}

// DetectMailboxProvider 按邮箱域名识别首版支持的 Microsoft 邮箱类型。
func DetectMailboxProvider(address string) (string, bool) {
	address = strings.ToLower(strings.TrimSpace(address))
	switch {
	case strings.HasSuffix(address, "@outlook.de"):
		return MailboxProviderOutlookDE, true
	case strings.Contains(address, "@outlook."):
		return MailboxProviderOutlook, true
	case strings.Contains(address, "@hotmail."):
		return MailboxProviderHotmail, true
	default:
		return "", false
	}
}

func IsSupportedMailboxProvider(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	for _, supported := range SupportedMailboxProviders {
		if provider == supported {
			return true
		}
	}
	return false
}
