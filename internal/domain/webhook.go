package domain

import "time"

type WebhookEndpoint struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID           string     `json:"id"`
	EndpointID   string     `json:"endpoint_id"`
	OrderID      string     `json:"order_id"`
	Event        string     `json:"event"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	ResponseCode int        `json:"response_code"`
	LastError    string     `json:"last_error,omitempty"`
	NextRetryAt  time.Time  `json:"next_retry_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
