package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *PostgresStore) ListPaymentProvidersPage(page, pageSize int) ([]domain.PaymentProvider, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	s.db.Model(&sqlPaymentProvider{}).Count(&total)
	var rows []sqlPaymentProvider
	s.db.Order("priority ASC, created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.PaymentProvider, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapPaymentProvider(row))
	}
	return items, total
}

func (s *PostgresStore) ListEnabledPaymentProviders(method string) ([]PaymentProviderSecret, error) {
	var rows []sqlPaymentProvider
	if err := s.db.Where("enabled = ?", true).Order("priority ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]PaymentProviderSecret, 0)
	for _, row := range rows {
		if !containsString(row.Methods, method) {
			continue
		}
		config := make(map[string]string)
		if err := s.decryptJSON(row.EncryptedConfig, &config); err != nil {
			return nil, err
		}
		items = append(items, PaymentProviderSecret{Provider: mapPaymentProvider(row), Config: config})
	}
	return items, nil
}

func (s *PostgresStore) GetPaymentProviderSecret(id string) (PaymentProviderSecret, error) {
	var row sqlPaymentProvider
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return PaymentProviderSecret{}, ErrPaymentProviderNotFound
	}
	config := make(map[string]string)
	if err := s.decryptJSON(row.EncryptedConfig, &config); err != nil {
		return PaymentProviderSecret{}, err
	}
	return PaymentProviderSecret{Provider: mapPaymentProvider(row), Config: config}, nil
}

func (s *PostgresStore) SavePaymentProvider(actorID string, provider domain.PaymentProvider, config map[string]string, ip string) (domain.PaymentProvider, error) {
	encrypted, err := s.encryptJSON(config)
	if err != nil {
		return domain.PaymentProvider{}, err
	}
	if provider.ID == "" {
		provider.ID = uuid.NewString()
	}
	row := sqlPaymentProvider{ID: provider.ID, Name: provider.Name, Type: provider.Type, Methods: provider.Methods, Enabled: provider.Enabled, Priority: provider.Priority, EncryptedConfig: encrypted}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "payment_provider.save", ResourceType: "payment_provider", ResourceID: row.ID, Detail: "保存支付服务商配置", IP: ip}).Error
	})
	return mapPaymentProvider(row), err
}

func (s *PostgresStore) DeletePaymentProvider(actorID, providerID, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&sqlPaymentProvider{}, "id = ?", providerID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrPaymentProviderNotFound
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "payment_provider.delete", ResourceType: "payment_provider", ResourceID: providerID, Detail: "删除支付服务商", IP: ip}).Error
	})
}

func (s *PostgresStore) CreatePaymentOrder(userID, providerID, method string, amount float64) (domain.PaymentOrder, error) {
	amountCents := cents(amount)
	if amountCents < 100 || amountCents > 10000000 {
		return domain.PaymentOrder{}, errors.New("充值金额必须在 1 到 100000 元之间")
	}
	var provider sqlPaymentProvider
	if err := s.db.First(&provider, "id = ? AND enabled = ?", providerID, true).Error; err != nil || !containsString(provider.Methods, method) {
		return domain.PaymentOrder{}, ErrPaymentProviderNotFound
	}
	now := time.Now().UTC()
	row := sqlPaymentOrder{ID: "PAY" + time.Now().UTC().Format("20060102150405") + uuid.NewString()[:8], UserID: userID, ProviderID: provider.ID, ProviderName: provider.Name, Method: method, Status: "pending", AmountCents: amountCents, CreatedAt: now, ExpiresAt: now.Add(30 * time.Minute)}
	if err := s.db.Create(&row).Error; err != nil {
		return domain.PaymentOrder{}, err
	}
	return mapPaymentOrder(row), nil
}

func (s *PostgresStore) SetPaymentOrderURL(orderID, payURL string) error {
	return s.db.Model(&sqlPaymentOrder{}).Where("id = ? AND status = ?", orderID, "pending").Update("pay_url", payURL).Error
}

func (s *PostgresStore) GetPaymentOrder(userID, orderID string) (domain.PaymentOrder, bool) {
	var row sqlPaymentOrder
	query := s.db.Where("id = ?", orderID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&row).Error; err != nil {
		return domain.PaymentOrder{}, false
	}
	return mapPaymentOrder(row), true
}

func (s *PostgresStore) ListPaymentOrdersPage(userID string, page, pageSize int) ([]domain.PaymentOrder, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlPaymentOrder{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	query.Count(&total)
	var rows []sqlPaymentOrder
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.PaymentOrder, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapPaymentOrder(row))
	}
	return items, total
}

func (s *PostgresStore) CancelPaymentOrder(userID, orderID string) (domain.PaymentOrder, error) {
	var row sqlPaymentOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ? AND user_id = ?", orderID, userID).Error; err != nil {
			return ErrPaymentOrderNotFound
		}
		if row.Status != "pending" {
			return errors.New("当前支付订单不能取消")
		}
		now := time.Now().UTC()
		row.Status, row.CanceledAt = "canceled", &now
		return tx.Save(&row).Error
	})
	return mapPaymentOrder(row), err
}

func (s *PostgresStore) CompletePaymentOrder(orderID, providerTradeNo string, paidAmount float64) (domain.PaymentOrder, error) {
	var result domain.PaymentOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order sqlPaymentOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", orderID).Error; err != nil {
			return ErrPaymentOrderNotFound
		}
		if order.Status == "completed" {
			result = mapPaymentOrder(order)
			return nil
		}
		if order.Status != "pending" && order.Status != "paid" {
			return errors.New("支付订单状态不允许入账")
		}
		if cents(paidAmount) != order.AmountCents {
			return ErrPaymentAmountMismatch
		}
		var user sqlUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", order.UserID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		user.BalanceCents += order.AmountCents
		if err := tx.Model(&user).Update("balance_cents", user.BalanceCents).Error; err != nil {
			return err
		}
		order.Status, order.ProviderTradeNo, order.PaidAt, order.CompletedAt = "completed", providerTradeNo, &now, &now
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, PaymentOrderID: order.ID, Type: "payment_topup", AmountCents: order.AmountCents, BalanceAfterCents: user.BalanceCents, Description: "在线支付充值"}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		audit := sqlAuditLog{ID: uuid.NewString(), ActorID: "payment-webhook", Action: "payment.complete", ResourceType: "payment_order", ResourceID: order.ID, Detail: "支付回调验签通过并完成余额入账"}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		result = mapPaymentOrder(order)
		return nil
	})
	return result, err
}

func (s *PostgresStore) ReapExpiredPaymentOrders() int {
	result := s.db.Model(&sqlPaymentOrder{}).Where("status = ? AND expires_at < ?", "pending", time.Now().UTC()).Updates(map[string]any{"status": "expired", "updated_at": time.Now().UTC()})
	return int(result.RowsAffected)
}

func (s *PostgresStore) encryptJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, data, nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (s *PostgresStore) decryptJSON(value string, target any) error {
	sealed, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(sealed) < gcm.NonceSize() {
		return errors.New("加密配置格式无效")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, target)
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func mapPaymentProvider(row sqlPaymentProvider) domain.PaymentProvider {
	return domain.PaymentProvider{ID: row.ID, Name: row.Name, Type: row.Type, Methods: append([]string(nil), row.Methods...), Enabled: row.Enabled, Priority: row.Priority, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func mapPaymentOrder(row sqlPaymentOrder) domain.PaymentOrder {
	return domain.PaymentOrder{ID: row.ID, UserID: row.UserID, ProviderID: row.ProviderID, ProviderName: row.ProviderName, Method: row.Method, Status: row.Status, Amount: float64(row.AmountCents) / 100, ProviderTradeNo: row.ProviderTradeNo, PayURL: row.PayURL, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, PaidAt: row.PaidAt, CompletedAt: row.CompletedAt, CanceledAt: row.CanceledAt}
}
