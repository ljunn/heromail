package domain

import "time"

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

// DefaultMailboxPoolName 是系统唯一的邮箱资产池名称，邮箱类型由 Mailbox.Provider 区分。
const DefaultMailboxPoolName = "邮箱池"
