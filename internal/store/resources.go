package store

import (
	"errors"
	"time"

	"github.com/ljunn/heromail/internal/domain"
)

type MailboxCredential struct {
	Mailbox domain.Mailbox
	Config  map[string]string
}

type OAuthState struct {
	ActorID string `json:"actor_id"`
	Pool    string `json:"pool"`
}

type ResourceRepository interface {
	ListMailboxPoolsPage(page, pageSize int) ([]domain.MailboxPool, int64)
	SaveMailboxPool(actorID string, pool domain.MailboxPool, ip string) (domain.MailboxPool, error)
	DeleteMailboxPool(actorID, poolID, ip string) error
	SaveMailbox(actorID string, mailbox domain.Mailbox, credential map[string]string, ip string) (domain.Mailbox, error)
	DeleteMailbox(actorID, mailboxID, ip string) error
	GetMailboxCredential(mailboxID string) (MailboxCredential, error)
	ListMailboxCredentials(limit int) ([]MailboxCredential, error)
	UpdateMailboxCredential(mailboxID string, credential map[string]string, validUntil time.Time) error
	SaveService(actorID string, service domain.Service, ip string) (domain.Service, error)
	DeleteService(actorID, serviceID, ip string) error
	CreateOAuthState(state string, value OAuthState, ttl time.Duration) error
	ConsumeOAuthState(state string) (OAuthState, error)
	WaitingOrdersForMailbox(mailboxID string) []domain.Order
	ServiceByID(serviceID string) (domain.Service, bool)
	MarkMailEvent(mailboxID, messageID, sender, subject string, receivedAt time.Time) (bool, error)
}

var (
	ErrMailboxPoolNotFound = errors.New("邮箱池不存在")
	ErrMailboxNotFound     = errors.New("邮箱不存在")
)
