package domain

import "time"

type PaymentProvider struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Methods   []string  `json:"methods"`
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PaymentOrder struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	ProviderID      string     `json:"provider_id"`
	ProviderName    string     `json:"provider_name"`
	Method          string     `json:"method"`
	Status          string     `json:"status"`
	Amount          float64    `json:"amount"`
	ProviderTradeNo string     `json:"provider_trade_no,omitempty"`
	PayURL          string     `json:"pay_url,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CanceledAt      *time.Time `json:"canceled_at,omitempty"`
}
