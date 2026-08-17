package store

import (
	"errors"

	"github.com/ljunn/heromail/internal/domain"
)

type AccountRepository interface {
	Register(email, password, displayName string) (domain.User, string, error)
	Login(email, password string) (domain.User, string, error)
	ResolveAccessToken(token string) (domain.User, bool)
	Logout(token string) error
	UpdateProfile(userID, displayName, password string) (domain.User, error)
	ListUsersPage(page, pageSize int) ([]domain.User, int64)
	CreateAPIKey(userID, name string, scopes []string) (domain.APIKey, string, error)
	ListAPIKeysPage(userID string, page, pageSize int) ([]domain.APIKey, int64)
	RevokeAPIKey(userID, keyID string) error
	ListWalletLedgersPage(userID string, page, pageSize int) ([]domain.WalletLedger, int64)
	AdjustBalance(actorID, userID string, amount float64, description, ip string) (domain.User, error)
	ListAuditLogsPage(page, pageSize int) ([]domain.AuditLog, int64)
	WriteAudit(actorID, action, resourceType, resourceID, detail, ip string) error
}

var (
	ErrInvalidCredentials = errors.New("邮箱或密码错误")
	ErrEmailExists        = errors.New("邮箱已注册")
	ErrAPIKeyNotFound     = errors.New("API Key 不存在")
)
