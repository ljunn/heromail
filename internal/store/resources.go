package store

import (
	"context"
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
	MailboxPoolByName(name string) (domain.MailboxPool, bool)
	SaveMailbox(actorID string, mailbox domain.Mailbox, credential map[string]string, ip string) (domain.Mailbox, error)
	DeleteMailbox(actorID, mailboxID, ip string) error
	GetMailboxCredential(mailboxID string) (MailboxCredential, error)
	ListMailboxCredentialsPage(afterID string, limit int) ([]MailboxCredential, error)
	UpdateMailboxCredential(actorID, mailboxID string, credential map[string]string, validUntil time.Time, ip string) error
	UpdateMailboxVerification(actorID, mailboxID, method, status, verificationError string, verifiedAt time.Time, ip string) error
	PendingMailboxVerificationIDs(limit int) ([]string, error)
	SaveService(actorID string, service domain.Service, ip string) (domain.Service, error)
	DeleteService(actorID, serviceID, ip string) error
	CreateOAuthState(state string, value OAuthState, ttl time.Duration) error
	ConsumeOAuthState(state string) (OAuthState, error)
	WaitingOrdersForMailbox(mailboxID string) []domain.Order
	ServiceByID(serviceID string) (domain.Service, bool)
	MarkMailEvent(mailboxID, messageID, sender, subject string, receivedAt time.Time) (bool, error)
}

// MailboxServiceStateRepository 用于收件扫描将历史命中平台写入邮箱×平台状态。
// 单独定义可选接口，避免影响只实现邮箱资源的测试适配器。
type MailboxServiceStateRepository interface {
	MarkMailboxServiceConsumed(mailboxID, serviceID string, changedAt time.Time) error
}

type MailboxServiceAdminRepository interface {
	MarkMailboxServiceRegistered(actorID, mailboxID, serviceID, ip string) error
}

type MailboxVerificationQueue interface {
	EnqueueMailboxVerification(ctx context.Context, mailboxID string) error
	DequeueMailboxVerification(ctx context.Context, timeout time.Duration) (string, error)
}

var (
	ErrMailboxPoolNotFound = errors.New("邮箱池不存在")
	ErrMailboxNotFound     = errors.New("邮箱不存在")
)
