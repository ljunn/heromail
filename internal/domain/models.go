package domain

import "time"

type OrderStatus string

const (
	OrderCreated          OrderStatus = "created"
	OrderAllocating       OrderStatus = "allocating"
	OrderAssigned         OrderStatus = "assigned"
	OrderWaitingCode      OrderStatus = "waiting_code"
	OrderCodeReceived     OrderStatus = "code_received"
	OrderCompleted        OrderStatus = "completed"
	OrderCanceled         OrderStatus = "canceled"
	OrderExpiredRefunded  OrderStatus = "expired_refunded"
	OrderAllocationFailed OrderStatus = "allocation_failed"
	OrderDisputed         OrderStatus = "disputed"
)

type MailboxState string

const (
	MailboxAvailable MailboxState = "available"
	MailboxLeased    MailboxState = "leased"
	MailboxCooldown  MailboxState = "cooldown"
	MailboxBlocked   MailboxState = "blocked"
	MailboxError     MailboxState = "auth_error"
	MailboxPending   MailboxState = "pending_verification"
)

type ServiceMailboxState string

const (
	ServiceAvailable ServiceMailboxState = "available"
	ServiceLeased    ServiceMailboxState = "leased"
	ServiceConsumed  ServiceMailboxState = "consumed"
	ServiceCooldown  ServiceMailboxState = "cooldown"
	ServiceBlocked   ServiceMailboxState = "blocked"
)

const (
	MailboxConnectionAuto           = "auto"
	MailboxConnectionMicrosoftOAuth = "microsoft_oauth"
	MailboxConnectionMicrosoftGraph = "microsoft_graph"
	MailboxConnectionIMAP           = "imap"

	MailboxVerificationPending  = "pending_verification"
	MailboxVerificationVerified = "verified"
	MailboxVerificationFailed   = "failed"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Balance     float64   `json:"balance"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type Service struct {
	ID               string   `json:"id"`
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Enabled          bool     `json:"enabled"`
	AllowedProviders []string `json:"allowed_providers"`
	Price            float64  `json:"price"`
	TTLSeconds       int      `json:"ttl_seconds"`
	SenderDomains    []string `json:"sender_domains"`
	SubjectKeywords  []string `json:"subject_keywords"`
	Regex            string   `json:"regex"`
}

type MailboxService struct {
	ServiceID string              `json:"service_id"`
	State     ServiceMailboxState `json:"state"`
	ChangedAt time.Time           `json:"changed_at"`
}

type Mailbox struct {
	ID                  string                    `json:"id"`
	Address             string                    `json:"address"`
	Provider            string                    `json:"provider"`
	Pool                string                    `json:"pool"`
	State               MailboxState              `json:"state"`
	HealthScore         int                       `json:"health_score"`
	OAuthValidUntil     time.Time                 `json:"oauth_valid_until"`
	ActiveOrderID       string                    `json:"active_order_id,omitempty"`
	TodayCodes          int                       `json:"today_codes"`
	LastReceivedAt      time.Time                 `json:"last_received_at,omitempty"`
	ConnectionMethod    string                    `json:"connection_method,omitempty"`
	VerificationStatus  string                    `json:"verification_status,omitempty"`
	LastVerifiedAt      time.Time                 `json:"last_verified_at,omitempty"`
	VerificationError   string                    `json:"verification_error,omitempty"`
	RegisteredPlatforms []string                  `json:"registered_platforms"`
	Services            map[string]MailboxService `json:"services"`
}

type Order struct {
	ID             string      `json:"id"`
	UserID         string      `json:"user_id"`
	ServiceID      string      `json:"service_id"`
	ServiceCode    string      `json:"service_code"`
	ServiceName    string      `json:"service_name"`
	MailboxID      string      `json:"mailbox_id"`
	MailboxAddress string      `json:"mailbox_address"`
	Status         OrderStatus `json:"status"`
	Code           string      `json:"code,omitempty"`
	Price          float64     `json:"price"`
	CreatedAt      time.Time   `json:"created_at"`
	AssignedAt     time.Time   `json:"assigned_at,omitempty"`
	SubmittedAt    time.Time   `json:"submitted_at,omitempty"`
	CodeReceivedAt time.Time   `json:"code_received_at,omitempty"`
	CompletedAt    time.Time   `json:"completed_at,omitempty"`
	ExpiresAt      time.Time   `json:"expires_at"`
	Refunded       bool        `json:"refunded"`
	RequestID      string      `json:"request_id,omitempty"`
	FailureReason  string      `json:"failure_reason,omitempty"`
}

type Overview struct {
	AvailableMailboxes int     `json:"available_mailboxes"`
	TotalMailboxes     int     `json:"total_mailboxes"`
	OutlookMailboxes   int     `json:"outlook_mailboxes"`
	OutlookDEMailboxes int     `json:"outlook_de_mailboxes"`
	HotmailMailboxes   int     `json:"hotmail_mailboxes"`
	PendingMailboxes   int     `json:"pending_mailboxes"`
	VerifiedMailboxes  int     `json:"verified_mailboxes"`
	ActiveLeases       int     `json:"active_leases"`
	TodayOrders        int     `json:"today_orders"`
	SuccessRate        float64 `json:"success_rate"`
	AverageCodeSeconds float64 `json:"average_code_seconds"`
	TodayRevenue       float64 `json:"today_revenue"`
	AuthErrors         int     `json:"auth_errors"`
	BlockedMailboxes   int     `json:"blocked_mailboxes"`
}
