package store

import "time"

type sqlUser struct {
	ID           string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"uniqueIndex;size:320;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	Role         string `gorm:"size:32;not null;index"`
	Status       string `gorm:"size:32;not null;index"`
	BalanceCents int64  `gorm:"not null;default:0"`
	DisplayName  string `gorm:"size:120"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

func (sqlUser) TableName() string { return "users" }

type sqlSession struct {
	ID        string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"size:64;not null;index"`
	TokenHash string `gorm:"uniqueIndex;size:64;not null"`
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

func (sqlSession) TableName() string { return "sessions" }

type sqlAPIKey struct {
	ID         string   `gorm:"primaryKey;size:64"`
	UserID     string   `gorm:"size:64;not null;index"`
	Name       string   `gorm:"size:120;not null"`
	Prefix     string   `gorm:"size:20;not null;index"`
	SecretHash string   `gorm:"uniqueIndex;size:64;not null"`
	Scopes     []string `gorm:"serializer:json;type:jsonb"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

func (sqlAPIKey) TableName() string { return "api_keys" }

type sqlService struct {
	ID               string   `gorm:"primaryKey;size:64"`
	Code             string   `gorm:"uniqueIndex;size:80;not null"`
	Name             string   `gorm:"size:120;not null"`
	Description      string   `gorm:"size:500"`
	Enabled          bool     `gorm:"not null;index"`
	AllowedProviders []string `gorm:"serializer:json;type:jsonb"`
	PriceCents       int64    `gorm:"not null"`
	TTLSeconds       int      `gorm:"not null"`
	SenderDomains    []string `gorm:"serializer:json;type:jsonb"`
	SubjectKeywords  []string `gorm:"serializer:json;type:jsonb"`
	Regex            string   `gorm:"size:500"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (sqlService) TableName() string { return "target_services" }

// sqlSeedState 记录一次性初始化是否已经完成，避免运行时配置被重复种回数据库。
type sqlSeedState struct {
	Key       string `gorm:"primaryKey;size:120"`
	CreatedAt time.Time
}

func (sqlSeedState) TableName() string { return "seed_states" }

type sqlMailbox struct {
	ID                  string    `gorm:"primaryKey;size:64"`
	Address             string    `gorm:"uniqueIndex;size:320;not null"`
	Provider            string    `gorm:"size:40;not null;index"`
	Pool                string    `gorm:"size:120;not null;index"`
	State               string    `gorm:"size:32;not null;index"`
	HealthScore         int       `gorm:"not null;index"`
	OAuthValidUntil     time.Time `gorm:"column:oauth_valid_until"`
	EncryptedCredential string    `gorm:"type:text"`
	ActiveOrderID       string    `gorm:"size:64;index"`
	TodayCodes          int       `gorm:"not null;default:0"`
	LastReceivedAt      *time.Time
	ConnectionMethod    string `gorm:"size:32;index"`
	VerificationStatus  string `gorm:"size:24;index"`
	LastVerifiedAt      *time.Time
	VerificationError   string `gorm:"size:500"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (sqlMailbox) TableName() string { return "mailboxes" }

type sqlMailboxService struct {
	MailboxID string    `gorm:"primaryKey;size:64"`
	ServiceID string    `gorm:"primaryKey;size:64;index"`
	State     string    `gorm:"size:32;not null;index"`
	ChangedAt time.Time `gorm:"not null"`
}

func (sqlMailboxService) TableName() string { return "mailbox_service_states" }

type sqlOrder struct {
	ID             string `gorm:"primaryKey;size:64"`
	UserID         string `gorm:"size:64;not null;index;uniqueIndex:idx_user_request"`
	ServiceID      string `gorm:"size:64;not null;index"`
	ServiceCode    string `gorm:"size:80;not null"`
	ServiceName    string `gorm:"size:120;not null"`
	MailboxID      string `gorm:"size:64;not null;index"`
	MailboxAddress string `gorm:"size:320;not null"`
	Status         string `gorm:"size:40;not null;index"`
	Code           string `gorm:"size:32"`
	PriceCents     int64  `gorm:"not null"`
	CreatedAt      time.Time
	AssignedAt     *time.Time
	SubmittedAt    *time.Time
	CodeReceivedAt *time.Time
	CompletedAt    *time.Time
	ExpiresAt      time.Time `gorm:"not null;index"`
	Refunded       bool      `gorm:"not null;default:false"`
	RequestID      string    `gorm:"size:160;index:idx_user_request"`
	FailureReason  string    `gorm:"size:500"`
	UpdatedAt      time.Time
}

func (sqlOrder) TableName() string { return "registration_orders" }

type sqlWalletLedger struct {
	ID                string `gorm:"primaryKey;size:64"`
	UserID            string `gorm:"size:64;not null;index"`
	OrderID           string `gorm:"size:64;index"`
	PaymentOrderID    string `gorm:"size:64;index"`
	Type              string `gorm:"size:40;not null;index"`
	AmountCents       int64  `gorm:"not null"`
	BalanceAfterCents int64  `gorm:"not null"`
	Description       string `gorm:"size:500"`
	CreatedAt         time.Time
}

func (sqlWalletLedger) TableName() string { return "wallet_ledgers" }

type sqlAuditLog struct {
	ID           string `gorm:"primaryKey;size:64"`
	ActorID      string `gorm:"size:64;index"`
	Action       string `gorm:"size:120;not null;index"`
	ResourceType string `gorm:"size:80;not null;index"`
	ResourceID   string `gorm:"size:120;index"`
	Detail       string `gorm:"type:text"`
	IP           string `gorm:"size:80"`
	CreatedAt    time.Time
}

func (sqlAuditLog) TableName() string { return "audit_logs" }

type sqlMailboxPool struct {
	ID              string `gorm:"primaryKey;size:64"`
	Name            string `gorm:"uniqueIndex;size:120;not null"`
	Provider        string `gorm:"size:40;not null;index"`
	Region          string `gorm:"size:40;index"`
	Enabled         bool   `gorm:"not null;index"`
	DailyLimit      int    `gorm:"not null;default:100"`
	CooldownSeconds int    `gorm:"not null;default:60"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (sqlMailboxPool) TableName() string { return "mailbox_pools" }

type sqlPaymentProvider struct {
	ID              string   `gorm:"primaryKey;size:64"`
	Name            string   `gorm:"size:120;not null"`
	Type            string   `gorm:"size:40;not null;index"`
	Methods         []string `gorm:"serializer:json;type:jsonb"`
	Enabled         bool     `gorm:"not null;index"`
	Priority        int      `gorm:"not null;default:100;index"`
	EncryptedConfig string   `gorm:"type:text;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (sqlPaymentProvider) TableName() string { return "payment_providers" }

type sqlPaymentOrder struct {
	ID              string `gorm:"primaryKey;size:64"`
	UserID          string `gorm:"size:64;not null;index"`
	ProviderID      string `gorm:"size:64;not null;index"`
	ProviderName    string `gorm:"size:120;not null"`
	Method          string `gorm:"size:40;not null;index"`
	Status          string `gorm:"size:40;not null;index"`
	AmountCents     int64  `gorm:"not null"`
	ProviderTradeNo string `gorm:"size:160;index"`
	PayURL          string `gorm:"type:text"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ExpiresAt       time.Time `gorm:"not null;index"`
	PaidAt          *time.Time
	CompletedAt     *time.Time
	CanceledAt      *time.Time
}

func (sqlPaymentOrder) TableName() string { return "payment_orders" }

type sqlMailEvent struct {
	ID         string    `gorm:"primaryKey;size:64"`
	MailboxID  string    `gorm:"size:64;not null;uniqueIndex:idx_mail_message"`
	MessageID  string    `gorm:"size:320;not null;uniqueIndex:idx_mail_message"`
	Sender     string    `gorm:"size:320"`
	Subject    string    `gorm:"size:500"`
	ReceivedAt time.Time `gorm:"not null;index"`
	CreatedAt  time.Time
}

func (sqlMailEvent) TableName() string { return "mail_events" }

type sqlWebhookEndpoint struct {
	ID              string   `gorm:"primaryKey;size:64"`
	UserID          string   `gorm:"size:64;not null;index"`
	URL             string   `gorm:"size:1000;not null"`
	Events          []string `gorm:"serializer:json;type:jsonb"`
	Enabled         bool     `gorm:"not null;index"`
	EncryptedSecret string   `gorm:"type:text;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (sqlWebhookEndpoint) TableName() string { return "webhook_endpoints" }

type sqlWebhookDelivery struct {
	ID           string `gorm:"primaryKey;size:64"`
	EndpointID   string `gorm:"size:64;not null;index"`
	OrderID      string `gorm:"size:64;not null;index"`
	Event        string `gorm:"size:80;not null;index"`
	Status       string `gorm:"size:32;not null;index"`
	Attempts     int    `gorm:"not null;default:0"`
	Payload      string `gorm:"type:text;not null"`
	ResponseCode int
	LastError    string    `gorm:"size:1000"`
	NextRetryAt  time.Time `gorm:"not null;index"`
	DeliveredAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (sqlWebhookDelivery) TableName() string { return "webhook_deliveries" }
