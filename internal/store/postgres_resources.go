package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"gorm.io/gorm"
)

func (s *PostgresStore) ListMailboxPoolsPage(page, pageSize int) ([]domain.MailboxPool, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	s.db.Model(&sqlMailboxPool{}).Count(&total)
	type poolRow struct {
		sqlMailboxPool
		MailboxCount int64
	}
	var rows []poolRow
	s.db.Table("mailbox_pools AS p").Select("p.*, count(m.id) AS mailbox_count").Joins("LEFT JOIN mailboxes AS m ON m.pool = p.name").Group("p.id").Order("p.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
	items := make([]domain.MailboxPool, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.MailboxPool{ID: row.ID, Name: row.Name, Provider: row.Provider, Region: row.Region, Enabled: row.Enabled, DailyLimit: row.DailyLimit, CooldownSecond: row.CooldownSeconds, MailboxCount: row.MailboxCount, CreatedAt: row.CreatedAt})
	}
	return items, total
}

func (s *PostgresStore) SaveMailboxPool(actorID string, pool domain.MailboxPool, ip string) (domain.MailboxPool, error) {
	if pool.ID == "" {
		pool.ID = uuid.NewString()
	}
	row := sqlMailboxPool{ID: pool.ID, Name: strings.TrimSpace(pool.Name), Provider: strings.ToLower(pool.Provider), Region: pool.Region, Enabled: pool.Enabled, DailyLimit: pool.DailyLimit, CooldownSeconds: pool.CooldownSecond}
	if row.DailyLimit <= 0 {
		row.DailyLimit = 100
	}
	if row.CooldownSeconds < 0 {
		row.CooldownSeconds = 0
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox_pool.save", ResourceType: "mailbox_pool", ResourceID: row.ID, Detail: "保存邮箱池", IP: ip}).Error
	})
	pool = domain.MailboxPool{ID: row.ID, Name: row.Name, Provider: row.Provider, Region: row.Region, Enabled: row.Enabled, DailyLimit: row.DailyLimit, CooldownSecond: row.CooldownSeconds, CreatedAt: row.CreatedAt}
	return pool, err
}

func (s *PostgresStore) DeleteMailboxPool(actorID, poolID, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var pool sqlMailboxPool
		if err := tx.First(&pool, "id = ?", poolID).Error; err != nil {
			return ErrMailboxPoolNotFound
		}
		var count int64
		tx.Model(&sqlMailbox{}).Where("pool = ?", pool.Name).Count(&count)
		if count > 0 {
			return errors.New("邮箱池仍包含邮箱，不能删除")
		}
		if err := tx.Delete(&pool).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox_pool.delete", ResourceType: "mailbox_pool", ResourceID: pool.ID, Detail: "删除邮箱池", IP: ip}).Error
	})
}

func (s *PostgresStore) SaveMailbox(actorID string, mailbox domain.Mailbox, credential map[string]string, ip string) (domain.Mailbox, error) {
	if mailbox.ID == "" {
		mailbox.ID = uuid.NewString()
	}
	encrypted := ""
	var err error
	if len(credential) > 0 {
		encrypted, err = s.encryptJSON(credential)
		if err != nil {
			return domain.Mailbox{}, err
		}
	}
	row := sqlMailbox{ID: mailbox.ID, Address: strings.ToLower(strings.TrimSpace(mailbox.Address)), Provider: strings.ToLower(mailbox.Provider), Pool: mailbox.Pool, State: string(mailbox.State), HealthScore: mailbox.HealthScore, OAuthValidUntil: mailbox.OAuthValidUntil, EncryptedCredential: encrypted}
	if row.State == "" {
		row.State = string(domain.MailboxAvailable)
	}
	if row.HealthScore == 0 {
		row.HealthScore = 100
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing sqlMailbox
		if findErr := tx.First(&existing, "id = ?", row.ID).Error; findErr == nil && encrypted == "" {
			row.EncryptedCredential = existing.EncryptedCredential
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		var services []sqlService
		if err := tx.Find(&services).Error; err != nil {
			return err
		}
		for _, service := range services {
			state := sqlMailboxService{MailboxID: row.ID, ServiceID: service.ID, State: string(domain.ServiceAvailable), ChangedAt: time.Now().UTC()}
			if err := tx.Where("mailbox_id = ? AND service_id = ?", row.ID, service.ID).FirstOrCreate(&state).Error; err != nil {
				return err
			}
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox.save", ResourceType: "mailbox", ResourceID: row.ID, Detail: "保存邮箱资源或 OAuth 授权", IP: ip}).Error
	})
	return mapMailbox(row, nil), err
}

func (s *PostgresStore) DeleteMailbox(actorID, mailboxID, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var mailbox sqlMailbox
		if err := tx.First(&mailbox, "id = ?", mailboxID).Error; err != nil {
			return ErrMailboxNotFound
		}
		if mailbox.ActiveOrderID != "" {
			return errors.New("邮箱存在活跃订单，不能删除")
		}
		if err := tx.Delete(&sqlMailboxService{}, "mailbox_id = ?", mailboxID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&mailbox).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox.delete", ResourceType: "mailbox", ResourceID: mailboxID, Detail: "删除邮箱资源", IP: ip}).Error
	})
}

func (s *PostgresStore) GetMailboxCredential(mailboxID string) (MailboxCredential, error) {
	var row sqlMailbox
	if err := s.db.First(&row, "id = ?", mailboxID).Error; err != nil {
		return MailboxCredential{}, ErrMailboxNotFound
	}
	config := make(map[string]string)
	if row.EncryptedCredential != "" {
		if err := s.decryptJSON(row.EncryptedCredential, &config); err != nil {
			return MailboxCredential{}, err
		}
	}
	return MailboxCredential{Mailbox: mapMailbox(row, nil), Config: config}, nil
}

func (s *PostgresStore) ListMailboxCredentials(limit int) ([]MailboxCredential, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	var rows []sqlMailbox
	if err := s.db.Where("encrypted_credential <> '' AND state NOT IN ?", []string{string(domain.MailboxBlocked), string(domain.MailboxError)}).Order("updated_at ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]MailboxCredential, 0, len(rows))
	for _, row := range rows {
		config := make(map[string]string)
		if err := s.decryptJSON(row.EncryptedCredential, &config); err != nil {
			continue
		}
		items = append(items, MailboxCredential{Mailbox: mapMailbox(row, nil), Config: config})
	}
	return items, nil
}

func (s *PostgresStore) UpdateMailboxCredential(mailboxID string, credential map[string]string, validUntil time.Time) error {
	encrypted, err := s.encryptJSON(credential)
	if err != nil {
		return err
	}
	return s.db.Model(&sqlMailbox{}).Where("id = ?", mailboxID).Updates(map[string]any{"encrypted_credential": encrypted, "oauth_valid_until": validUntil, "state": domain.MailboxAvailable}).Error
}

func (s *PostgresStore) SaveService(actorID string, service domain.Service, ip string) (domain.Service, error) {
	if service.ID == "" {
		service.ID = uuid.NewString()
	}
	row := sqlService{ID: service.ID, Code: strings.ToLower(service.Code), Name: service.Name, Description: service.Description, Enabled: service.Enabled, AllowedProviders: service.AllowedProviders, PriceCents: cents(service.Price), TTLSeconds: service.TTLSeconds, SenderDomains: service.SenderDomains, SubjectKeywords: service.SubjectKeywords, Regex: service.Regex}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		var mailboxIDs []string
		if err := tx.Model(&sqlMailbox{}).Pluck("id", &mailboxIDs).Error; err != nil {
			return err
		}
		for _, mailboxID := range mailboxIDs {
			state := sqlMailboxService{MailboxID: mailboxID, ServiceID: row.ID, State: string(domain.ServiceAvailable), ChangedAt: time.Now().UTC()}
			if err := tx.Where("mailbox_id = ? AND service_id = ?", mailboxID, row.ID).FirstOrCreate(&state).Error; err != nil {
				return err
			}
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "target_service.save", ResourceType: "target_service", ResourceID: row.ID, Detail: "保存目标平台与收码规则", IP: ip}).Error
	})
	return mapService(row), err
}

func (s *PostgresStore) DeleteService(actorID, serviceID, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&sqlOrder{}).Where("service_id = ?", serviceID).Count(&count)
		if count > 0 {
			return errors.New("目标平台已有订单，只能停用不能删除")
		}
		if err := tx.Delete(&sqlMailboxService{}, "service_id = ?", serviceID).Error; err != nil {
			return err
		}
		result := tx.Delete(&sqlService{}, "id = ?", serviceID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrServiceNotFound
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "target_service.delete", ResourceType: "target_service", ResourceID: serviceID, Detail: "删除目标平台", IP: ip}).Error
	})
}

func (s *PostgresStore) CreateOAuthState(state string, value OAuthState, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.redis.Set(context.Background(), "heromail:oauth:"+state, data, ttl).Err()
}

func (s *PostgresStore) ConsumeOAuthState(state string) (OAuthState, error) {
	key := "heromail:oauth:" + state
	data, err := s.redis.GetDel(context.Background(), key).Bytes()
	if err != nil {
		return OAuthState{}, err
	}
	var value OAuthState
	err = json.Unmarshal(data, &value)
	return value, err
}

func (s *PostgresStore) WaitingOrdersForMailbox(mailboxID string) []domain.Order {
	var rows []sqlOrder
	s.db.Where("mailbox_id = ? AND status = ? AND expires_at > ?", mailboxID, domain.OrderWaitingCode, time.Now()).Order("created_at ASC").Find(&rows)
	items := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapOrder(row))
	}
	return items
}

func (s *PostgresStore) ServiceByID(serviceID string) (domain.Service, bool) {
	var row sqlService
	if err := s.db.First(&row, "id = ?", serviceID).Error; err != nil {
		return domain.Service{}, false
	}
	return mapService(row), true
}

func (s *PostgresStore) MarkMailEvent(mailboxID, messageID, sender, subject string, receivedAt time.Time) (bool, error) {
	row := sqlMailEvent{ID: uuid.NewString(), MailboxID: mailboxID, MessageID: messageID, Sender: sender, Subject: subject, ReceivedAt: receivedAt}
	if err := s.db.Create(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
