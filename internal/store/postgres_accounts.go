package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *PostgresStore) Register(email, password, displayName string) (domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 10 {
		return domain.User{}, "", ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, "", err
	}
	user := sqlUser{ID: uuid.NewString(), Email: email, PasswordHash: string(hash), Role: "user", Status: "active", DisplayName: strings.TrimSpace(displayName)}
	var token string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrEmailExists
			}
			return err
		}
		var sessionErr error
		token, sessionErr = createSession(tx, user.ID)
		if sessionErr != nil {
			return sessionErr
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: user.ID, Action: "user.register", ResourceType: "user", ResourceID: user.ID, Detail: "用户完成注册"}).Error
	})
	return mapUser(user), token, err
}

func (s *PostgresStore) Login(email, password string) (domain.User, string, error) {
	var user sqlUser
	if err := s.db.Where("email = ? AND status = ?", strings.ToLower(strings.TrimSpace(email)), "active").First(&user).Error; err != nil {
		return domain.User{}, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, "", ErrInvalidCredentials
	}
	var token string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		token, err = createSession(tx, user.ID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Model(&sqlUser{}).Where("id = ?", user.ID).Update("last_login_at", now).Error
	})
	return mapUser(user), token, err
}

func (s *PostgresStore) ResolveAccessToken(token string) (domain.User, bool) {
	hash := hashToken(token)
	var user sqlUser
	if strings.HasPrefix(token, "hs_") {
		var session sqlSession
		err := s.db.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, time.Now()).First(&session).Error
		if err != nil || s.db.First(&user, "id = ? AND status = ?", session.UserID, "active").Error != nil {
			return domain.User{}, false
		}
		return mapUser(user), true
	}
	if strings.HasPrefix(token, "hm_") {
		var key sqlAPIKey
		err := s.db.Where("secret_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", hash, time.Now()).First(&key).Error
		if err != nil || s.db.First(&user, "id = ? AND status = ?", key.UserID, "active").Error != nil {
			return domain.User{}, false
		}
		now := time.Now().UTC()
		s.db.Model(&key).Update("last_used_at", now)
		return mapUser(user), true
	}
	return domain.User{}, false
}

func (s *PostgresStore) Logout(token string) error {
	if !strings.HasPrefix(token, "hs_") {
		return nil
	}
	now := time.Now().UTC()
	return s.db.Model(&sqlSession{}).Where("token_hash = ? AND revoked_at IS NULL", hashToken(token)).Update("revoked_at", now).Error
}

func (s *PostgresStore) UpdateProfile(userID, displayName string) (domain.User, error) {
	if err := s.db.Model(&sqlUser{}).Where("id = ?", userID).Update("display_name", strings.TrimSpace(displayName)).Error; err != nil {
		return domain.User{}, err
	}
	user, ok := s.User(userID)
	if !ok {
		return domain.User{}, errors.New("用户不存在")
	}
	return user, nil
}

func (s *PostgresStore) ChangePassword(userID, currentPassword, newPassword string) (string, error) {
	if len(newPassword) < 10 {
		return "", ErrPasswordTooShort
	}
	var token string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user sqlUser
		if err := tx.Where("id = ? AND status = ?", userID, "active").First(&user).Error; err != nil {
			return ErrInvalidCredentials
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			return ErrPasswordMismatch
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := tx.Model(&sqlUser{}).Where("id = ?", userID).Update("password_hash", string(hash)).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&sqlSession{}).Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		token, err = createSession(tx, userID)
		return err
	})
	return token, err
}

func (s *PostgresStore) ListUsersPage(page, pageSize int) ([]domain.User, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	s.db.Model(&sqlUser{}).Count(&total)
	var rows []sqlUser
	s.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapUser(row))
	}
	return items, total
}

func (s *PostgresStore) CreateAPIKey(userID, name string, scopes []string) (domain.APIKey, string, error) {
	secret, err := randomToken("hm_", 32)
	if err != nil {
		return domain.APIKey{}, "", err
	}
	if len(scopes) == 0 {
		scopes = []string{"orders:read", "orders:write"}
	}
	prefix := secret
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	row := sqlAPIKey{ID: uuid.NewString(), UserID: userID, Name: strings.TrimSpace(name), Prefix: prefix, SecretHash: hashToken(secret), Scopes: scopes}
	if row.Name == "" {
		row.Name = "默认密钥"
	}
	if err := s.db.Create(&row).Error; err != nil {
		return domain.APIKey{}, "", err
	}
	return mapAPIKey(row), secret, nil
}

func (s *PostgresStore) ListAPIKeysPage(userID string, page, pageSize int) ([]domain.APIKey, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlAPIKey{}).Where("user_id = ?", userID)
	var total int64
	query.Count(&total)
	var rows []sqlAPIKey
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.APIKey, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAPIKey(row))
	}
	return items, total
}

func (s *PostgresStore) RevokeAPIKey(userID, keyID string) error {
	now := time.Now().UTC()
	result := s.db.Model(&sqlAPIKey{}).Where("id = ? AND user_id = ? AND revoked_at IS NULL", keyID, userID).Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

func (s *PostgresStore) ListWalletLedgersPage(userID string, page, pageSize int) ([]domain.WalletLedger, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlWalletLedger{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	query.Count(&total)
	var rows []sqlWalletLedger
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.WalletLedger, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.WalletLedger{ID: row.ID, Type: row.Type, Amount: float64(row.AmountCents) / 100, BalanceAfter: float64(row.BalanceAfterCents) / 100, OrderID: row.OrderID, PaymentID: row.PaymentOrderID, Description: row.Description, CreatedAt: row.CreatedAt})
	}
	return items, total
}

func (s *PostgresStore) AdjustBalance(actorID, userID string, amount float64, description, ip string) (domain.User, error) {
	amountCents := cents(amount)
	var result domain.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user sqlUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		if user.BalanceCents+amountCents < 0 {
			return ErrInsufficientBalance
		}
		user.BalanceCents += amountCents
		if err := tx.Model(&user).Update("balance_cents", user.BalanceCents).Error; err != nil {
			return err
		}
		ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, Type: "admin_adjustment", AmountCents: amountCents, BalanceAfterCents: user.BalanceCents, Description: description}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		audit := sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "wallet.adjust", ResourceType: "user", ResourceID: user.ID, Detail: description, IP: ip}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		result = mapUser(user)
		return nil
	})
	return result, err
}

func (s *PostgresStore) ListAuditLogsPage(page, pageSize int) ([]domain.AuditLog, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	s.db.Model(&sqlAuditLog{}).Count(&total)
	var rows []sqlAuditLog
	s.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.AuditLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.AuditLog{ID: row.ID, ActorID: row.ActorID, Action: row.Action, ResourceType: row.ResourceType, ResourceID: row.ResourceID, Detail: row.Detail, IP: row.IP, CreatedAt: row.CreatedAt})
	}
	return items, total
}

func (s *PostgresStore) WriteAudit(actorID, action, resourceType, resourceID, detail, ip string) error {
	return s.db.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Detail: detail, IP: ip}).Error
}

func createSession(tx *gorm.DB, userID string) (string, error) {
	token, err := randomToken("hs_", 32)
	if err != nil {
		return "", err
	}
	session := sqlSession{ID: uuid.NewString(), UserID: userID, TokenHash: hashToken(token), ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	if err := tx.Create(&session).Error; err != nil {
		return "", err
	}
	return token, nil
}

func randomToken(prefix string, size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func mapAPIKey(row sqlAPIKey) domain.APIKey {
	return domain.APIKey{ID: row.ID, Name: row.Name, Prefix: row.Prefix, Scopes: append([]string(nil), row.Scopes...), LastUsedAt: row.LastUsedAt, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt}
}
