package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *PostgresStore) ListMailboxPoolsPage(page, pageSize int) ([]domain.MailboxPool, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	s.db.Model(&sqlMailboxPool{}).Where("name = ?", domain.DefaultMailboxPoolName).Count(&total)
	type poolRow struct {
		ID              string
		Name            string
		Provider        string
		Region          string
		Enabled         bool
		DailyLimit      int
		CooldownSeconds int
		CreatedAt       time.Time
		MailboxCount    int64
	}
	var rows []poolRow
	s.db.Table("mailbox_pools AS p").Select("p.id, p.name, p.provider, p.region, p.enabled, p.daily_limit, p.cooldown_seconds, p.created_at, count(m.id) AS mailbox_count").Joins("LEFT JOIN mailboxes AS m ON m.pool = p.name").Where("p.name = ?", domain.DefaultMailboxPoolName).Group("p.id").Order("p.created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows)
	items := make([]domain.MailboxPool, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.MailboxPool{ID: row.ID, Name: row.Name, Provider: row.Provider, Region: row.Region, Enabled: row.Enabled, DailyLimit: row.DailyLimit, CooldownSecond: row.CooldownSeconds, MailboxCount: row.MailboxCount, CreatedAt: row.CreatedAt})
	}
	return items, total
}

func (s *PostgresStore) MailboxPoolByName(name string) (domain.MailboxPool, bool) {
	var row sqlMailboxPool
	if err := s.db.First(&row, "name = ?", strings.TrimSpace(name)).Error; err != nil {
		return domain.MailboxPool{}, false
	}
	return domain.MailboxPool{ID: row.ID, Name: row.Name, Provider: row.Provider, Region: row.Region, Enabled: row.Enabled, DailyLimit: row.DailyLimit, CooldownSecond: row.CooldownSeconds, CreatedAt: row.CreatedAt}, true
}

func (s *PostgresStore) SaveMailbox(actorID string, mailbox domain.Mailbox, credential map[string]string, ip string) (domain.Mailbox, error) {
	mailbox.Address = strings.ToLower(strings.TrimSpace(mailbox.Address))
	provider, supported := domain.DetectMailboxProvider(mailbox.Address)
	if !supported {
		return domain.Mailbox{}, errors.New("不支持该邮箱类型")
	}
	mailbox.Provider = provider
	mailbox.Pool = strings.TrimSpace(mailbox.Pool)
	if mailbox.Pool == "" {
		mailbox.Pool = domain.DefaultMailboxPoolName
	}
	encrypted := ""
	var err error
	if len(credential) > 0 {
		encrypted, err = s.encryptJSON(credential)
		if err != nil {
			return domain.Mailbox{}, err
		}
	}
	var row sqlMailbox
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing sqlMailbox
		query := tx.Where("address = ?", mailbox.Address)
		if mailbox.ID != "" {
			query = tx.Where("id = ? OR address = ?", mailbox.ID, mailbox.Address)
		}
		findErr := query.First(&existing).Error
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if findErr == nil {
			row = existing
			row.Address = mailbox.Address
			row.Provider = mailbox.Provider
			row.Pool = mailbox.Pool
			if mailbox.HealthScore > 0 {
				row.HealthScore = mailbox.HealthScore
			}
			if !mailbox.OAuthValidUntil.IsZero() {
				row.OAuthValidUntil = mailbox.OAuthValidUntil
			}
			if mailbox.ConnectionMethod != "" {
				row.ConnectionMethod = mailbox.ConnectionMethod
			}
			if mailbox.VerificationStatus != "" {
				row.VerificationStatus = mailbox.VerificationStatus
				row.VerificationError = mailbox.VerificationError
			}
			if mailbox.State != "" && row.ActiveOrderID == "" {
				row.State = string(mailbox.State)
			}
			if !mailbox.LastVerifiedAt.IsZero() {
				verifiedAt := mailbox.LastVerifiedAt
				row.LastVerifiedAt = &verifiedAt
			}
			if row.ActiveOrderID == "" && row.State == string(domain.MailboxError) && mailbox.VerificationStatus == domain.MailboxVerificationVerified {
				row.State = string(domain.MailboxAvailable)
			}
		} else {
			mailboxID := mailbox.ID
			if mailboxID == "" {
				mailboxID = uuid.NewString()
			}
			row = sqlMailbox{
				ID:                 mailboxID,
				Address:            mailbox.Address,
				Provider:           mailbox.Provider,
				Pool:               mailbox.Pool,
				State:              string(mailbox.State),
				HealthScore:        mailbox.HealthScore,
				OAuthValidUntil:    mailbox.OAuthValidUntil,
				ConnectionMethod:   mailbox.ConnectionMethod,
				VerificationStatus: mailbox.VerificationStatus,
				VerificationError:  mailbox.VerificationError,
			}
			if row.State == "" {
				row.State = string(domain.MailboxAvailable)
			}
			if row.HealthScore == 0 && row.State == string(domain.MailboxAvailable) {
				row.HealthScore = 100
			}
			if !mailbox.LastVerifiedAt.IsZero() {
				verifiedAt := mailbox.LastVerifiedAt
				row.LastVerifiedAt = &verifiedAt
			}
		}
		if encrypted != "" {
			row.EncryptedCredential = encrypted
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

func (s *PostgresStore) ListMailboxCredentialsPage(afterID string, limit int) ([]MailboxCredential, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	var rows []sqlMailbox
	query := s.db.Where("encrypted_credential <> '' AND state NOT IN ?", []string{string(domain.MailboxBlocked), string(domain.MailboxError)})
	if afterID != "" {
		query = query.Where("id > ?", afterID)
	}
	if err := query.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
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

func (s *PostgresStore) UpdateMailboxCredential(actorID, mailboxID string, credential map[string]string, validUntil time.Time, ip string) error {
	encrypted, err := s.encryptJSON(credential)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&sqlMailbox{}).Where("id = ?", mailboxID).Updates(map[string]any{"encrypted_credential": encrypted, "oauth_valid_until": validUntil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrMailboxNotFound
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox.credential.refresh", ResourceType: "mailbox", ResourceID: mailboxID, Detail: "刷新邮箱 OAuth 凭证", IP: ip}).Error
	})
}

// UpdateMailboxVerification 只更新连接健康状态，不触碰邮箱凭证。
func (s *PostgresStore) UpdateMailboxVerification(actorID, mailboxID, method, status, verificationError string, verifiedAt time.Time, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var mailbox sqlMailbox
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&mailbox, "id = ?", mailboxID).Error; err != nil {
			return ErrMailboxNotFound
		}
		mailbox.ConnectionMethod = method
		mailbox.VerificationStatus = status
		mailbox.VerificationError = verificationError
		if !verifiedAt.IsZero() {
			mailbox.LastVerifiedAt = &verifiedAt
		}
		if status == domain.MailboxVerificationVerified {
			mailbox.HealthScore = 100
			if (mailbox.State == string(domain.MailboxError) || mailbox.State == string(domain.MailboxPending)) && mailbox.ActiveOrderID == "" {
				mailbox.State = string(domain.MailboxAvailable)
			}
		} else if status == domain.MailboxVerificationFailed && mailbox.ActiveOrderID == "" && mailbox.State != string(domain.MailboxBlocked) {
			mailbox.State = string(domain.MailboxError)
			mailbox.HealthScore = 0
		}
		if err := tx.Save(&mailbox).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "mailbox.verify", ResourceType: "mailbox", ResourceID: mailboxID, Detail: "邮箱连接验证结果：" + status, IP: ip}).Error
	})
}

func (s *PostgresStore) PendingMailboxVerificationIDs(limit int) ([]string, error) {
	if limit < 1 || limit > 5000 {
		limit = 1000
	}
	var ids []string
	err := s.db.Model(&sqlMailbox{}).
		Where("verification_status = ? OR (verification_status = ? AND connection_method IN ? AND oauth_valid_until <= ?)",
			domain.MailboxVerificationPending,
			domain.MailboxVerificationVerified,
			[]string{domain.MailboxConnectionMicrosoftGraph, domain.MailboxConnectionMicrosoftOAuth},
			time.Now()).
		Order("updated_at ASC").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

const (
	mailboxVerificationQueueKey = "heromail:mailbox:verification:queue"
	mailboxVerificationSetKey   = "heromail:mailbox:verification:queued"
)

func (s *PostgresStore) EnqueueMailboxVerification(ctx context.Context, mailboxID string) error {
	script := redis.NewScript(`
if redis.call("SADD", KEYS[1], ARGV[1]) == 1 then
  redis.call("LPUSH", KEYS[2], ARGV[1])
end
return 1`)
	return script.Run(ctx, s.redis, []string{mailboxVerificationSetKey, mailboxVerificationQueueKey}, mailboxID).Err()
}

func (s *PostgresStore) DequeueMailboxVerification(ctx context.Context, timeout time.Duration) (string, error) {
	values, err := s.redis.BRPop(ctx, timeout, mailboxVerificationQueueKey).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(values) != 2 {
		return "", nil
	}
	mailboxID := values[1]
	if err := s.redis.SRem(ctx, mailboxVerificationSetKey, mailboxID).Err(); err != nil {
		return "", err
	}
	return mailboxID, nil
}

func (s *PostgresStore) SaveService(actorID string, service domain.Service, ip string) (domain.Service, error) {
	if service.ID == "" {
		service.ID = uuid.NewString()
	}
	row := sqlService{ID: service.ID, Code: strings.ToLower(service.Code), Name: service.Name, Description: service.Description, Enabled: service.Enabled, AllowedProviders: service.AllowedProviders, PriceCents: minimumProviderPriceCents(service.ProviderPrices), ProviderPricesCents: providerPricesCents(service.ProviderPrices), TTLSeconds: service.TTLSeconds, SenderDomains: service.SenderDomains, SubjectKeywords: service.SubjectKeywords, Regex: service.Regex}
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
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: actorID, Action: "target_service.save", ResourceType: "target_service", ResourceID: row.ID, Detail: "保存目标平台配置", IP: ip}).Error
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
	s.db.Where("mailbox_id = ? AND status IN ? AND expires_at > ?", mailboxID, []domain.OrderStatus{domain.OrderAssigned, domain.OrderWaitingCode}, time.Now()).Order("created_at ASC").Find(&rows)
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

func (s *PostgresStore) MarkMailboxServiceConsumed(mailboxID, serviceID string, changedAt time.Time) error {
	if changedAt.IsZero() {
		changedAt = time.Now().UTC()
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&sqlMailboxService{}).
			Where("mailbox_id = ? AND service_id = ? AND state = ?", mailboxID, serviceID, domain.ServiceAvailable).
			Updates(map[string]any{"state": string(domain.ServiceConsumed), "changed_at": changedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Create(&sqlAuditLog{
			ID:           uuid.NewString(),
			ActorID:      "mail-worker",
			Action:       "mailbox.service.consume",
			ResourceType: "mailbox_service",
			ResourceID:   mailboxID + ":" + serviceID,
			Detail:       "历史收件匹配目标平台，标记邮箱已注册",
		}).Error
	})
}

func (s *PostgresStore) MarkMailboxServiceRegistered(actorID, mailboxID, serviceID, ip string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var state sqlMailboxService
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&state, "mailbox_id = ? AND service_id = ?", mailboxID, serviceID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMailboxServiceNotFound
			}
			return err
		}
		if state.State == string(domain.ServiceLeased) {
			return ErrMailboxServiceLeased
		}
		if state.State == string(domain.ServiceConsumed) {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&state).Updates(map[string]any{
			"state":      string(domain.ServiceConsumed),
			"changed_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&sqlAuditLog{
			ID:           uuid.NewString(),
			ActorID:      actorID,
			Action:       "mailbox.service.manual_register",
			ResourceType: "mailbox_service",
			ResourceID:   mailboxID + ":" + serviceID,
			Detail:       "管理员手工标记邮箱已完成目标平台注册",
			IP:           ip,
		}).Error
	})
}
