package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ljunn/heromail/internal/domain"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresConfig struct {
	DSN           string
	RedisAddress  string
	RedisPassword string
	AdminEmail    string
	AdminPassword string
	EncryptionKey string
	SeedDemo      bool
}

type PostgresStore struct {
	db            *gorm.DB
	redis         *redis.Client
	encryptionKey []byte
}

const defaultServicesSeedKey = "target-services-v1"

const historicalMicrosoftIMAPReverifyKey = "microsoft-imap-oauth-reverify-v1"

const microsoftIMAPNoopReverifyKey = "microsoft-imap-noop-reverify-v1"

func shouldSeedDefaultServices(serviceCount int64, markerExists bool) bool {
	return !markerExists && serviceCount == 0
}

func NewPostgresStore(ctx context.Context, config PostgresConfig) (*PostgresStore, error) {
	if config.DSN == "" || config.RedisAddress == "" {
		return nil, errors.New("PostgreSQL 和 Redis 配置不能为空")
	}
	db, err := gorm.Open(postgres.Open(config.DSN), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败：%w", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: config.RedisAddress, Password: config.RedisPassword})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败：%w", err)
	}
	encryptionKey, err := hex.DecodeString(config.EncryptionKey)
	if err != nil || len(encryptionKey) != 32 {
		return nil, errors.New("HEROMAIL_ENCRYPTION_KEY 必须是 64 位十六进制字符串")
	}
	store := &PostgresStore{db: db, redis: redisClient, encryptionKey: encryptionKey}
	if err := store.migrate(); err != nil {
		return nil, err
	}
	if err := store.seed(config); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) migrate() error {
	if err := s.db.AutoMigrate(&sqlUser{}, &sqlSession{}, &sqlAPIKey{}, &sqlService{}, &sqlSeedState{}, &sqlSystemConfig{}, &sqlMailboxPool{}, &sqlMailbox{}, &sqlMailboxService{}, &sqlOrder{}, &sqlWalletLedger{}, &sqlPaymentProvider{}, &sqlPaymentOrder{}, &sqlMailEvent{}, &sqlWebhookEndpoint{}, &sqlWebhookDelivery{}, &sqlAuditLog{}); err != nil {
		return fmt.Errorf("执行数据库迁移失败：%w", err)
	}
	if err := s.migrateProviderPricing(); err != nil {
		return err
	}
	if err := s.migrateLegacyMicrosoftIMAPOAuth(); err != nil {
		return err
	}
	if err := s.migrateHistoricalMicrosoftIMAPOAuth(); err != nil {
		return err
	}
	return s.migrateMicrosoftIMAPNoopFailures()
}

// migrateLegacyMicrosoftIMAPOAuth 修复旧版本把源系统 IMAP OAuth 当成 Graph
// 凭据导入的问题。通过协议字段和 connection_method 做幂等标记，失败记录会
// 回到验证队列，由 VerificationWorker 使用正确的 IMAP scope 重新验证。
func (s *PostgresStore) migrateLegacyMicrosoftIMAPOAuth() error {
	var migrated int64
	var rows []sqlMailbox
	query := s.db.Where("provider IN ? AND connection_method = ? AND encrypted_credential <> ''", []string{
		domain.MailboxProviderOutlook,
		domain.MailboxProviderOutlookDE,
		domain.MailboxProviderHotmail,
	}, domain.MailboxConnectionAuto)
	result := query.FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
		for index := range rows {
			var credential map[string]string
			if err := s.decryptJSON(rows[index].EncryptedCredential, &credential); err != nil {
				continue
			}
			if !legacyMicrosoftIMAPOAuthCredential(credential) {
				continue
			}
			credential["oauth_protocol"] = "imap"
			encrypted, err := s.encryptJSON(credential)
			if err != nil {
				return err
			}
			updates := map[string]any{
				"encrypted_credential": encrypted,
				"connection_method":    domain.MailboxConnectionAuto,
				"verification_status":  domain.MailboxVerificationPending,
				"verification_error":   "",
				"updated_at":           time.Now().UTC(),
			}
			if rows[index].ActiveOrderID == "" {
				updates["state"] = domain.MailboxPending
			}
			result := tx.Model(&sqlMailbox{}).Where("id = ? AND connection_method = ?", rows[index].ID, domain.MailboxConnectionAuto).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			migrated += result.RowsAffected
		}
		return nil
	})
	if result.Error != nil {
		return fmt.Errorf("迁移旧 Microsoft IMAP OAuth 凭据失败：%w", result.Error)
	}
	if migrated == 0 {
		return nil
	}
	return s.db.Create(&sqlAuditLog{
		ID:           uuid.NewString(),
		ActorID:      "system",
		Action:       "mailbox.oauth.migrate",
		ResourceType: "mailbox",
		ResourceID:   "all",
		Detail:       fmt.Sprintf("修复旧版 Microsoft IMAP OAuth 导入记录 %d 条并重新验证", migrated),
	}).Error
}

func legacyMicrosoftIMAPOAuthCredential(credential map[string]string) bool {
	return strings.TrimSpace(credential["oauth_protocol"]) == "" &&
		strings.TrimSpace(credential["client_id"]) != "" &&
		strings.TrimSpace(credential["refresh_token"]) != ""
}

// migrateHistoricalMicrosoftIMAPOAuth 重新验证旧版本误走 Graph/Basic Auth 的
// Microsoft IMAP OAuth 记录。只执行一次，避免每次重启都重复触发大量 Token 请求。
func (s *PostgresStore) migrateHistoricalMicrosoftIMAPOAuth() error {
	var marker sqlSeedState
	markerErr := s.db.First(&marker, "key = ?", historicalMicrosoftIMAPReverifyKey).Error
	if markerErr == nil {
		return nil
	}
	if !errors.Is(markerErr, gorm.ErrRecordNotFound) {
		return markerErr
	}

	var requeued int64
	var rows []sqlMailbox
	providers := []string{domain.MailboxProviderOutlook, domain.MailboxProviderOutlookDE, domain.MailboxProviderHotmail}
	result := s.db.Where("provider IN ? AND verification_status = ? AND encrypted_credential <> ''", providers, domain.MailboxVerificationFailed).FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
		for index := range rows {
			var credential map[string]string
			if err := s.decryptJSON(rows[index].EncryptedCredential, &credential); err != nil {
				continue
			}
			if !legacyMicrosoftIMAPOAuthCredential(credential) && !microsoftIMAPOAuthCredential(credential) {
				continue
			}
			updates := map[string]any{
				"verification_status": domain.MailboxVerificationPending,
				"verification_error":  "",
				"updated_at":          time.Now().UTC(),
			}
			if legacyMicrosoftIMAPOAuthCredential(credential) {
				credential["oauth_protocol"] = "imap"
				encrypted, err := s.encryptJSON(credential)
				if err != nil {
					return err
				}
				updates["encrypted_credential"] = encrypted
			}
			if rows[index].ActiveOrderID == "" {
				updates["state"] = domain.MailboxPending
			}
			updated := tx.Model(&sqlMailbox{}).Where("id = ? AND verification_status = ?", rows[index].ID, domain.MailboxVerificationFailed).Updates(updates)
			if updated.Error != nil {
				return updated.Error
			}
			requeued += updated.RowsAffected
		}
		return nil
	})
	if result.Error != nil {
		return fmt.Errorf("重新验证历史 Microsoft IMAP OAuth 邮箱失败：%w", result.Error)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sqlSeedState{Key: historicalMicrosoftIMAPReverifyKey, CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		if requeued == 0 {
			return nil
		}
		return tx.Create(&sqlAuditLog{
			ID:           uuid.NewString(),
			ActorID:      "system",
			Action:       "mailbox.oauth.reverify",
			ResourceType: "mailbox",
			ResourceID:   "all",
			Detail:       fmt.Sprintf("重新验证历史 Microsoft IMAP OAuth 邮箱 %d 条", requeued),
		}).Error
	})
}

func microsoftIMAPOAuthCredential(credential map[string]string) bool {
	protocol := strings.ToLower(strings.TrimSpace(credential["oauth_protocol"]))
	return strings.TrimSpace(credential["client_id"]) != "" &&
		strings.TrimSpace(credential["refresh_token"]) != "" &&
		(protocol == "imap" || protocol == "imap_oauth" || protocol == "microsoft_imap" || protocol == "microsoft_imap_oauth")
}

// migrateMicrosoftIMAPNoopFailures 重新排队旧版 NOOP 健康检查产生的假失败。
// 仅匹配明确的 Microsoft IMAP 会话错误，避免覆盖真实授权失效记录。
func (s *PostgresStore) migrateMicrosoftIMAPNoopFailures() error {
	var marker sqlSeedState
	markerErr := s.db.First(&marker, "key = ?", microsoftIMAPNoopReverifyKey).Error
	if markerErr == nil {
		return nil
	}
	if !errors.Is(markerErr, gorm.ErrRecordNotFound) {
		return markerErr
	}
	result := s.db.Model(&sqlMailbox{}).
		Where("provider IN ? AND verification_status = ? AND verification_error ILIKE ?", []string{domain.MailboxProviderOutlook, domain.MailboxProviderOutlookDE, domain.MailboxProviderHotmail}, domain.MailboxVerificationFailed, "%User is authenticated but not connected%").
		Updates(map[string]any{
			"verification_status": domain.MailboxVerificationPending,
			"verification_error":  "",
			"state":               gorm.Expr("CASE WHEN active_order_id = '' THEN ? ELSE state END", domain.MailboxPending),
			"updated_at":          time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("重新验证 Microsoft IMAP NOOP 假失败邮箱失败：%w", result.Error)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sqlSeedState{Key: microsoftIMAPNoopReverifyKey, CreatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Create(&sqlAuditLog{
			ID:           uuid.NewString(),
			ActorID:      "system",
			Action:       "mailbox.oauth.reverify",
			ResourceType: "mailbox",
			ResourceID:   "all",
			Detail:       fmt.Sprintf("重新验证旧版 Microsoft IMAP NOOP 假失败邮箱 %d 条", result.RowsAffected),
		}).Error
	})
}

func (s *PostgresStore) migrateProviderPricing() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var services []sqlService
		if err := tx.Find(&services).Error; err != nil {
			return err
		}
		updatedServices := int64(0)
		for index := range services {
			prices := make(map[string]int64, len(services[index].AllowedProviders))
			for provider, price := range services[index].ProviderPricesCents {
				prices[provider] = price
			}
			changed := false
			for _, provider := range services[index].AllowedProviders {
				if _, exists := prices[provider]; exists {
					continue
				}
				prices[provider] = services[index].PriceCents
				changed = true
			}
			if !changed {
				continue
			}
			encodedPrices, err := json.Marshal(prices)
			if err != nil {
				return err
			}
			if err := tx.Model(&services[index]).Update("provider_prices_cents", gorm.Expr("?::jsonb", string(encodedPrices))).Error; err != nil {
				return err
			}
			updatedServices++
		}
		mailboxProvider := tx.Exec(`
UPDATE registration_orders AS orders
SET mailbox_provider = mailboxes.provider
FROM mailboxes
WHERE orders.mailbox_id = mailboxes.id
  AND (orders.mailbox_provider IS NULL OR orders.mailbox_provider = '')`)
		if mailboxProvider.Error != nil {
			return mailboxProvider.Error
		}
		requestedProviders := tx.Exec(`
UPDATE registration_orders
SET requested_providers = jsonb_build_array(mailbox_provider)
WHERE mailbox_provider <> ''
  AND (requested_providers IS NULL OR requested_providers = 'null'::jsonb OR requested_providers = '[]'::jsonb)`)
		if requestedProviders.Error != nil {
			return requestedProviders.Error
		}
		if updatedServices == 0 && mailboxProvider.RowsAffected == 0 && requestedProviders.RowsAffected == 0 {
			return nil
		}
		detail := fmt.Sprintf("迁移邮箱类型定价 %d 个平台，补齐订单邮箱类型 %d 条、请求类型 %d 条", updatedServices, mailboxProvider.RowsAffected, requestedProviders.RowsAffected)
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: "system", Action: "pricing.provider.migrate", ResourceType: "target_service", ResourceID: "all", Detail: detail}).Error
	})
}

func (s *PostgresStore) seed(config PostgresConfig) error {
	adminEmail := config.AdminEmail
	if adminEmail == "" {
		adminEmail = "admin@heromail.local"
	}
	adminPassword := config.AdminPassword
	if adminPassword == "" {
		return errors.New("必须配置 HEROMAIL_ADMIN_PASSWORD")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := sqlUser{ID: "admin-001", Email: strings.ToLower(adminEmail), PasswordHash: string(hash), Role: "admin", Status: "active", DisplayName: "系统管理员"}
	if err := s.db.Where("id = ?", admin.ID).FirstOrCreate(&admin).Error; err != nil {
		return err
	}
	if config.SeedDemo {
		userHash, hashErr := bcrypt.GenerateFromPassword([]byte("heromail-demo"), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		user := sqlUser{ID: "user-001", Email: "demo@example.com", PasswordHash: string(userHash), Role: "user", Status: "active", BalanceCents: 4860, DisplayName: "演示用户"}
		if err := s.db.Where("id = ?", user.ID).FirstOrCreate(&user).Error; err != nil {
			return err
		}
	}
	services := make([]sqlService, 0, len(defaultServices()))
	for _, service := range defaultServices() {
		services = append(services, sqlService{
			ID: service.ID, Code: service.Code, Name: service.Name, Description: service.Description,
			Enabled: service.Enabled, AllowedProviders: service.AllowedProviders, PriceCents: minimumProviderPriceCents(service.ProviderPrices), ProviderPricesCents: providerPricesCents(service.ProviderPrices),
			TTLSeconds: service.TTLSeconds, SenderDomains: service.SenderDomains, SubjectKeywords: service.SubjectKeywords, Regex: service.Regex,
		})
	}
	if err := s.ensureDefaultServices(services); err != nil {
		return err
	}
	if err := s.ensureDefaultMailboxPool(); err != nil {
		return err
	}
	if err := s.ensureMailboxServiceStates(); err != nil {
		return err
	}
	if config.SeedDemo {
		return s.seedDemoMailboxes(services)
	}
	return nil
}

func (s *PostgresStore) ensureDefaultServices(services []sqlService) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var marker sqlSeedState
		err := tx.First(&marker, "key = ?", defaultServicesSeedKey).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Model(&sqlService{}).Count(&count).Error; err != nil {
			return err
		}
		// 空库才创建内置平台；已有平台的库只登记初始化完成，绝不恢复管理员已删除的记录。
		if shouldSeedDefaultServices(count, false) {
			for index := range services {
				if err := tx.Create(&services[index]).Error; err != nil {
					return err
				}
			}
		}
		return tx.Create(&sqlSeedState{Key: defaultServicesSeedKey, CreatedAt: time.Now().UTC()}).Error
	})
}

func (s *PostgresStore) ensureDefaultMailboxPool() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var defaultPool sqlMailboxPool
		created := false
		if err := tx.Where("name = ?", domain.DefaultMailboxPoolName).First(&defaultPool).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			defaultPool = sqlMailboxPool{ID: "pool-default", Name: domain.DefaultMailboxPoolName, Provider: "mixed", Region: "global", Enabled: true, DailyLimit: 1000, CooldownSeconds: 0}
			if err := tx.Create(&defaultPool).Error; err != nil {
				return err
			}
			created = true
		} else if err != nil {
			return err
		}
		if err := tx.Model(&defaultPool).Updates(map[string]any{"provider": "mixed", "enabled": true}).Error; err != nil {
			return err
		}
		moved := tx.Model(&sqlMailbox{}).Where("pool <> ? OR pool IS NULL OR pool = ''", defaultPool.Name).Update("pool", defaultPool.Name)
		if moved.Error != nil {
			return moved.Error
		}
		legacy := tx.Model(&sqlMailboxPool{}).Where("id <> ? AND enabled = ?", defaultPool.ID, true).Update("enabled", false)
		if legacy.Error != nil {
			return legacy.Error
		}
		reclassified := tx.Model(&sqlMailbox{}).Where("lower(address) LIKE ? AND provider <> ?", "%@outlook.de", domain.MailboxProviderOutlookDE).Update("provider", domain.MailboxProviderOutlookDE)
		if reclassified.Error != nil {
			return reclassified.Error
		}
		var services []sqlService
		if err := tx.Find(&services).Error; err != nil {
			return err
		}
		updatedServices := int64(0)
		for index := range services {
			if contains(services[index].AllowedProviders, domain.MailboxProviderOutlook) && !contains(services[index].AllowedProviders, domain.MailboxProviderOutlookDE) {
				services[index].AllowedProviders = append(services[index].AllowedProviders, domain.MailboxProviderOutlookDE)
				providers, err := json.Marshal(services[index].AllowedProviders)
				if err != nil {
					return err
				}
				if err := tx.Model(&services[index]).Update("allowed_providers", gorm.Expr("CAST(? AS jsonb)", string(providers))).Error; err != nil {
					return err
				}
				updatedServices++
			}
		}
		if !created && moved.RowsAffected == 0 && legacy.RowsAffected == 0 && reclassified.RowsAffected == 0 && updatedServices == 0 {
			return nil
		}
		detail := fmt.Sprintf("统一邮箱池：迁移邮箱 %d 个，停用旧池 %d 个，重分类 Outlook.de %d 个，更新平台 %d 个", moved.RowsAffected, legacy.RowsAffected, reclassified.RowsAffected, updatedServices)
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: "system", Action: "mailbox_pool.consolidate", ResourceType: "mailbox_pool", ResourceID: defaultPool.ID, Detail: detail}).Error
	})
}

func (s *PostgresStore) ensureMailboxServiceStates() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`
INSERT INTO mailbox_service_states (mailbox_id, service_id, state, changed_at)
SELECT mailboxes.id, target_services.id, ?, ?
FROM mailboxes
CROSS JOIN target_services
ON CONFLICT (mailbox_id, service_id) DO NOTHING`, domain.ServiceAvailable, time.Now().UTC())
		if result.Error != nil {
			return fmt.Errorf("补齐邮箱平台状态失败：%w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		detail := fmt.Sprintf("补齐缺失的邮箱平台状态 %d 条", result.RowsAffected)
		return tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: "system", Action: "mailbox_service.backfill", ResourceType: "mailbox_service", ResourceID: "all", Detail: detail}).Error
	})
}

func (s *PostgresStore) seedDemoMailboxes(services []sqlService) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for index := 1; index <= 24; index++ {
			provider, domainName, pool := domain.MailboxProviderOutlook, "outlook.com", domain.DefaultMailboxPoolName
			if index%2 == 0 {
				provider, domainName = domain.MailboxProviderHotmail, "hotmail.com"
			}
			now := time.Now().UTC()
			mailbox := sqlMailbox{ID: fmt.Sprintf("mb-%03d", index), Address: fmt.Sprintf("hero_%02d@%s", index, domainName), Provider: provider, Pool: pool, State: string(domain.MailboxAvailable), HealthScore: 84 + index%16, OAuthValidUntil: now.Add(30 * 24 * time.Hour), ConnectionMethod: domain.MailboxConnectionMicrosoftOAuth, VerificationStatus: domain.MailboxVerificationVerified, LastVerifiedAt: &now}
			if err := tx.Where("id = ?", mailbox.ID).FirstOrCreate(&mailbox).Error; err != nil {
				return err
			}
			for _, service := range services {
				state := sqlMailboxService{MailboxID: mailbox.ID, ServiceID: service.ID, State: string(domain.ServiceAvailable), ChangedAt: time.Now()}
				if err := tx.Where("mailbox_id = ? AND service_id = ?", mailbox.ID, service.ID).FirstOrCreate(&state).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	database, err := s.db.DB()
	if err != nil {
		return err
	}
	if err := database.PingContext(ctx); err != nil {
		return err
	}
	return s.redis.Ping(ctx).Err()
}

func (s *PostgresStore) StorageName() string { return "postgres+redis" }

func (s *PostgresStore) User(id string) (domain.User, bool) {
	var user sqlUser
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return domain.User{}, false
	}
	return mapUser(user), true
}

func (s *PostgresStore) ListServices() []domain.Service {
	items, _ := s.ListServicesPage(1, 100)
	return items
}

func (s *PostgresStore) ListServicesPage(page, pageSize int) ([]domain.Service, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	var rows []sqlService
	s.db.Model(&sqlService{}).Count(&total)
	s.db.Order("name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.Service, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapService(row))
	}
	return items, total
}

func (s *PostgresStore) ListEnabledServicesPage(page, pageSize int) ([]domain.Service, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	var rows []sqlService
	query := s.db.Model(&sqlService{}).Where("enabled = ?", true)
	query.Count(&total)
	query.Order("name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.Service, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapService(row))
	}
	return items, total
}

func (s *PostgresStore) EnabledService(codeOrID string) (domain.Service, bool) {
	var row sqlService
	if err := s.db.Where("enabled = ? AND (id = ? OR code = ?)", true, codeOrID, strings.ToLower(codeOrID)).First(&row).Error; err != nil {
		return domain.Service{}, false
	}
	return mapService(row), true
}

func (s *PostgresStore) ServiceUsage(serviceIDs []string) map[string]ServiceUsage {
	type usageRow struct {
		ServiceID string
		State     string
		Count     int
	}
	var rows []usageRow
	if len(serviceIDs) > 0 {
		s.db.Table("mailbox_service_states AS mss").
			Select("mss.service_id, mss.state, count(*) AS count").
			Joins("JOIN mailboxes AS m ON m.id = mss.mailbox_id").
			Where("mss.service_id IN ? AND m.verification_status = ?", serviceIDs, domain.MailboxVerificationVerified).
			Group("mss.service_id, mss.state").Scan(&rows)
	}
	result := make(map[string]ServiceUsage, len(serviceIDs))
	for _, row := range rows {
		usage := result[row.ServiceID]
		switch domain.ServiceMailboxState(row.State) {
		case domain.ServiceAvailable:
			usage.Available = row.Count
		case domain.ServiceLeased:
			usage.Leased = row.Count
		case domain.ServiceConsumed:
			usage.Consumed = row.Count
		}
		result[row.ServiceID] = usage
	}
	return result
}

func (s *PostgresStore) ServiceAvailability(serviceIDs []string) map[string]int {
	byProvider := s.ServiceAvailabilityByProvider(serviceIDs)
	result := make(map[string]int, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		for _, count := range byProvider[serviceID] {
			result[serviceID] += count
		}
	}
	return result
}

func (s *PostgresStore) ServiceAvailabilityByProvider(serviceIDs []string) map[string]map[string]int {
	type availabilityRow struct {
		ServiceID string
		Provider  string
		Count     int
	}
	result := make(map[string]map[string]int, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		result[serviceID] = map[string]int{}
	}
	if len(serviceIDs) == 0 {
		return result
	}

	var rows []availabilityRow
	// 每日额度先按邮箱池聚合一次，避免对每个候选邮箱重复扫描整个 mailboxes 表。
	dailyUsage := s.db.Table("mailboxes AS daily_m").
		Select("daily_m.pool, COALESCE(SUM(CASE WHEN daily_m.last_received_at::date = CURRENT_DATE THEN daily_m.today_codes ELSE 0 END), 0) AS today_codes").
		Group("daily_m.pool")
	s.db.Table("mailbox_service_states AS mss").
		Select("mss.service_id, m.provider, COUNT(*) AS count").
		Joins("JOIN target_services AS ts ON ts.id = mss.service_id").
		Joins("JOIN mailboxes AS m ON m.id = mss.mailbox_id").
		Joins("JOIN mailbox_pools AS mp ON mp.name = m.pool AND mp.enabled = ?", true).
		Joins("LEFT JOIN (?) AS daily_usage ON daily_usage.pool = mp.name", dailyUsage).
		Where("mss.service_id IN ? AND ts.enabled = ?", serviceIDs, true).
		Where("m.state = ? AND m.active_order_id = '' AND m.health_score >= ? AND m.verification_status = ?", domain.MailboxAvailable, 60, domain.MailboxVerificationVerified).
		Where("(m.connection_method = ? OR m.oauth_valid_until > ?)", domain.MailboxConnectionIMAP, time.Now()).
		Where("mss.state = ?", domain.ServiceAvailable).
		Where("ts.allowed_providers @> jsonb_build_array(m.provider)").
		Where("m.last_received_at IS NULL OR m.last_received_at <= NOW() - (mp.cooldown_seconds * INTERVAL '1 second')").
		Where("mp.daily_limit <= 0 OR COALESCE(daily_usage.today_codes, 0) < mp.daily_limit").
		Group("mss.service_id, m.provider").
		Scan(&rows)
	for _, row := range rows {
		result[row.ServiceID][row.Provider] = row.Count
	}
	return result
}

func (s *PostgresStore) CreateOrder(userID, serviceID, requestID string, mailboxProviders []string) (domain.Order, error) {
	ctx := context.Background()
	lockKey := "heromail:allocate:" + serviceID
	lockValue := uuid.NewString()
	if err := s.acquireAllocationLock(ctx, lockKey, lockValue); err != nil {
		return domain.Order{}, err
	}
	defer s.releaseLock(ctx, lockKey, lockValue)

	var result domain.Order
	var err error
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var service sqlService
		if err := tx.Where("id = ? OR code = ?", serviceID, serviceID).First(&service).Error; err != nil {
			return ErrServiceNotFound
		}
		if !service.Enabled {
			return ErrServiceDisabled
		}
		requestedProviders, maximumPrice, validationErr := validateOrderProviders(mapService(service), mailboxProviders)
		if validationErr != nil {
			return validationErr
		}
		if requestID != "" {
			var existing sqlOrder
			if err := tx.Where("user_id = ? AND request_id = ?", userID, requestID).First(&existing).Error; err == nil {
				result = mapOrder(existing)
				return ErrDuplicateRequest
			}
		}
		var user sqlUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ? AND status = ?", userID, "active").Error; err != nil {
			return errors.New("user not found")
		}
		if user.BalanceCents < cents(maximumPrice) {
			return ErrInsufficientBalance
		}
		var mailbox sqlMailbox
		dailyUsage := tx.Table("mailboxes AS daily_m").
			Select("daily_m.pool, COALESCE(SUM(CASE WHEN daily_m.last_received_at::date = CURRENT_DATE THEN daily_m.today_codes ELSE 0 END), 0) AS today_codes").
			Group("daily_m.pool")
		query := tx.Table("mailboxes AS m").Select("m.*").
			Joins("JOIN mailbox_service_states AS mss ON mss.mailbox_id = m.id AND mss.service_id = ?", service.ID).
			Joins("JOIN mailbox_pools AS mp ON mp.name = m.pool AND mp.enabled = ?", true).
			Joins("LEFT JOIN (?) AS daily_usage ON daily_usage.pool = mp.name", dailyUsage).
			Where("m.state = ? AND m.active_order_id = '' AND m.health_score >= ? AND m.verification_status = ?", domain.MailboxAvailable, 60, domain.MailboxVerificationVerified).
			Where("(m.connection_method = ? OR m.oauth_valid_until > ?)", domain.MailboxConnectionIMAP, time.Now()).
			Where("mss.state = ?", domain.ServiceAvailable).
			Where("m.provider IN ?", requestedProviders).
			Where("m.last_received_at IS NULL OR m.last_received_at <= NOW() - (mp.cooldown_seconds * INTERVAL '1 second')").
			Where("mp.daily_limit <= 0 OR COALESCE(daily_usage.today_codes, 0) < mp.daily_limit").
			Order("m.health_score DESC, m.id ASC").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "m"}, Options: "SKIP LOCKED"}).
			Limit(1).Scan(&mailbox)
		if query.Error != nil || mailbox.ID == "" {
			return ErrNoMailboxAvailable
		}
		priceCents, priced := effectiveProviderPricesCents(service)[mailbox.Provider]
		if !priced {
			return ErrInvalidMailboxProviders
		}
		now := time.Now().UTC()
		orderID := "ORD" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:16])
		order := sqlOrder{ID: orderID, UserID: user.ID, ServiceID: service.ID, ServiceCode: service.Code, ServiceName: service.Name, MailboxID: mailbox.ID, MailboxAddress: mailbox.Address, MailboxProvider: mailbox.Provider, RequestedProviders: requestedProviders, Status: string(domain.OrderWaitingCode), PriceCents: priceCents, CreatedAt: now, AssignedAt: &now, SubmittedAt: &now, ExpiresAt: now.Add(time.Duration(effectiveOrderTTLSeconds(service.TTLSeconds)) * time.Second), RequestID: requestID}
		user.BalanceCents -= priceCents
		if err := tx.Model(&user).Update("balance_cents", user.BalanceCents).Error; err != nil {
			return err
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if err := tx.Model(&sqlMailbox{}).Where("id = ?", mailbox.ID).Updates(map[string]any{"state": domain.MailboxLeased, "active_order_id": order.ID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&sqlMailboxService{}).Where("mailbox_id = ? AND service_id = ?", mailbox.ID, service.ID).Updates(map[string]any{"state": domain.ServiceLeased, "changed_at": now}).Error; err != nil {
			return err
		}
		ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, OrderID: order.ID, Type: "order_charge", AmountCents: -priceCents, BalanceAfterCents: user.BalanceCents, Description: "注册订单扣费"}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := s.enqueueOrderWebhook(tx, order, "order.waiting_code"); err != nil {
			return err
		}
		result = mapOrder(order)
		return nil
	})
	return result, err
}

// acquireAllocationLock 短暂等待同一平台的并发分配完成，避免把锁竞争误报为库存耗尽。
func (s *PostgresStore) acquireAllocationLock(ctx context.Context, key, value string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		locked, err := s.redis.SetNX(ctx, key, value, 15*time.Second).Result()
		if err != nil {
			return err
		}
		if locked {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrAllocationBusy
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *PostgresStore) GetOrder(id string) (domain.Order, bool) {
	var order sqlOrder
	if err := s.db.First(&order, "id = ?", id).Error; err != nil {
		return domain.Order{}, false
	}
	return mapOrder(order), true
}

func (s *PostgresStore) ListOrders(userID string) []domain.Order {
	items, _ := s.ListOrdersPage(userID, 1, 100)
	return items
}

func (s *PostgresStore) ListOrdersPage(userID string, page, pageSize int) ([]domain.Order, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlOrder{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	var total int64
	query.Count(&total)
	var rows []sqlOrder
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapOrder(row))
	}
	return items, total
}

func (s *PostgresStore) ListUserOrdersPage(userID string, filter UserOrderFilter, page, pageSize int) ([]domain.Order, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlOrder{}).Where("user_id = ?", userID)
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Service != "" {
		query = query.Where("(service_id = ? OR service_code = ?)", filter.Service, filter.Service)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("id ILIKE ? OR service_name ILIKE ? OR mailbox_address ILIKE ?", pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0
	}
	var rows []sqlOrder
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0
	}
	items := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapOrder(row))
	}
	return items, total
}

func (s *PostgresStore) ListAdminOrdersPage(filter AdminOrderFilter, page, pageSize int) ([]domain.Order, int64) {
	page, pageSize = normalizePage(page, pageSize)
	query := s.db.Model(&sqlOrder{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Service != "" {
		query = query.Where("service_id = ? OR service_code = ?", filter.Service, filter.Service)
	}
	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("id ILIKE ? OR user_id ILIKE ? OR mailbox_address ILIKE ?", pattern, pattern, pattern)
	}
	var total int64
	query.Count(&total)
	var rows []sqlOrder
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	items := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapOrder(row))
	}
	return items, total
}

func (s *PostgresStore) ReceiveCodeValue(id, code string) (domain.Order, error) {
	var result domain.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order sqlOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", id).Error; err != nil {
			return ErrOrderNotFound
		}
		status := domain.OrderStatus(order.Status)
		if status != domain.OrderAssigned && status != domain.OrderWaitingCode {
			return ErrInvalidOrderState
		}
		now := time.Now().UTC()
		if !now.Before(order.ExpiresAt) {
			return ErrInvalidOrderState
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return ErrVerificationCodeRequired
		}
		order.Status, order.Code, order.CodeReceivedAt = string(domain.OrderCodeReceived), code, &now
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		todayCodes := gorm.Expr("CASE WHEN last_received_at::date = CURRENT_DATE THEN today_codes + 1 ELSE 1 END")
		if err := tx.Model(&sqlMailbox{}).Where("id = ?", order.MailboxID).Updates(map[string]any{"state": domain.MailboxAvailable, "active_order_id": "", "today_codes": todayCodes, "last_received_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&sqlMailboxService{}).Where("mailbox_id = ? AND service_id = ?", order.MailboxID, order.ServiceID).Updates(map[string]any{"state": domain.ServiceConsumed, "changed_at": now}).Error; err != nil {
			return err
		}
		if err := s.enqueueOrderWebhook(tx, order, "order.code_received"); err != nil {
			return err
		}
		result = mapOrder(order)
		return nil
	})
	return result, err
}

func (s *PostgresStore) CompleteOrder(id, userID string) (domain.Order, error) {
	return s.updateOrder(id, userID, []domain.OrderStatus{domain.OrderCodeReceived}, func(order *sqlOrder, now time.Time) {
		order.Status, order.CompletedAt = string(domain.OrderCompleted), &now
	})
}

func (s *PostgresStore) CancelOrder(id, userID string) (domain.Order, error) {
	var result domain.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order sqlOrder
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&order).Error; err != nil {
			return ErrOrderNotFound
		}
		status := domain.OrderStatus(order.Status)
		if status != domain.OrderAssigned && status != domain.OrderWaitingCode {
			return ErrInvalidOrderState
		}
		if err := s.refundOrder(tx, &order, domain.OrderCanceled); err != nil {
			return err
		}
		result = mapOrder(order)
		return nil
	})
	return result, err
}

func (s *PostgresStore) ReapExpired() int {
	var ids []string
	s.db.Model(&sqlOrder{}).Where("status IN ? AND expires_at < ?", []string{string(domain.OrderAssigned), string(domain.OrderWaitingCode)}, time.Now()).Pluck("id", &ids)
	count := 0
	for _, id := range ids {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var order sqlOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", id).Error; err != nil {
				return err
			}
			if order.ExpiresAt.After(time.Now()) || (domain.OrderStatus(order.Status) != domain.OrderAssigned && domain.OrderStatus(order.Status) != domain.OrderWaitingCode) {
				return nil
			}
			if err := s.refundOrder(tx, &order, domain.OrderExpiredRefunded); err != nil {
				return err
			}
			count++
			return nil
		})
		_ = err
	}
	return count
}

// ReconcileInvalidMailboxOrders 回收已绑定到未验证邮箱的存量订单。
// 部署修复版本后立即运行一次，之后由主循环定期运行，覆盖升级前已经产生的坏租约。
func (s *PostgresStore) ReconcileInvalidMailboxOrders() int {
	var ids []string
	s.db.Model(&sqlOrder{}).
		Joins("JOIN mailboxes AS m ON m.id = registration_orders.mailbox_id").
		Where("registration_orders.status IN ? AND COALESCE(m.verification_status, '') <> ?", []string{string(domain.OrderAssigned), string(domain.OrderWaitingCode)}, domain.MailboxVerificationVerified).
		Pluck("registration_orders.id", &ids)
	count := 0
	for _, id := range ids {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var orderRef sqlOrder
			if err := tx.Select("id", "mailbox_id").First(&orderRef, "id = ?", id).Error; err != nil {
				return err
			}
			var mailbox sqlMailbox
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&mailbox, "id = ?", orderRef.MailboxID).Error; err != nil {
				return err
			}
			if mailbox.VerificationStatus == domain.MailboxVerificationVerified {
				return nil
			}
			var order sqlOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", id).Error; err != nil {
				return err
			}
			if domain.OrderStatus(order.Status) != domain.OrderAssigned && domain.OrderStatus(order.Status) != domain.OrderWaitingCode {
				return nil
			}
			order.FailureReason = "邮箱未通过连接验证，订单已自动退款"
			if err := s.refundOrder(tx, &order, domain.OrderExpiredRefunded); err != nil {
				return err
			}
			updates := map[string]any{"active_order_id": "", "state": string(domain.MailboxError), "health_score": 0}
			if mailbox.State == string(domain.MailboxBlocked) {
				delete(updates, "state")
			}
			return tx.Model(&sqlMailbox{}).Where("id = ?", mailbox.ID).Updates(updates).Error
		})
		if err == nil {
			count++
		}
	}
	return count
}

func (s *PostgresStore) Overview() domain.Overview {
	var result domain.Overview
	var total, outlook, outlookDE, hotmail, gmail, icloud, mailcom, pending, verified, available, leased, authErrors, blocked int64
	s.db.Model(&sqlMailbox{}).Count(&total)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderOutlook).Count(&outlook)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderOutlookDE).Count(&outlookDE)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderHotmail).Count(&hotmail)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderGmail).Count(&gmail)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderICloud).Count(&icloud)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderMailCom).Count(&mailcom)
	s.db.Model(&sqlMailbox{}).Where("verification_status = ?", domain.MailboxVerificationPending).Count(&pending)
	s.db.Model(&sqlMailbox{}).Where("verification_status = ?", domain.MailboxVerificationVerified).Count(&verified)
	s.db.Model(&sqlMailbox{}).Where("state = ? AND verification_status = ?", domain.MailboxAvailable, domain.MailboxVerificationVerified).Count(&available)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxLeased).Count(&leased)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxError).Count(&authErrors)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxBlocked).Count(&blocked)
	result.AvailableMailboxes = int(available)
	result.TotalMailboxes = int(total)
	result.OutlookMailboxes = int(outlook)
	result.OutlookDEMailboxes = int(outlookDE)
	result.HotmailMailboxes = int(hotmail)
	result.GmailMailboxes = int(gmail)
	result.ICloudMailboxes = int(icloud)
	result.MailComMailboxes = int(mailcom)
	result.PendingMailboxes = int(pending)
	result.VerifiedMailboxes = int(verified)
	result.ActiveLeases = int(leased)
	result.AuthErrors = int(authErrors)
	result.BlockedMailboxes = int(blocked)
	start := time.Now().Truncate(24 * time.Hour)
	var orders []sqlOrder
	s.db.Where("created_at >= ?", start).Find(&orders)
	result.TodayOrders = len(orders)
	var codeSeconds float64
	var received int
	for _, order := range orders {
		if order.CodeReceivedAt != nil {
			received++
			if order.SubmittedAt != nil {
				codeSeconds += order.CodeReceivedAt.Sub(*order.SubmittedAt).Seconds()
			}
			result.TodayRevenue += float64(order.PriceCents) / 100
		}
	}
	if received > 0 {
		result.SuccessRate = float64(received) / float64(len(orders)) * 100
		result.AverageCodeSeconds = codeSeconds / float64(received)
	}
	return result
}

func (s *PostgresStore) Mailboxes() []domain.Mailbox {
	items, _ := s.MailboxesPage(1, 100)
	return items
}

func (s *PostgresStore) MailboxesPage(page, pageSize int) ([]domain.Mailbox, int64) {
	return s.ListMailboxesPage(MailboxFilter{}, page, pageSize)
}

func (s *PostgresStore) ListMailboxesPage(filter MailboxFilter, page, pageSize int) ([]domain.Mailbox, int64) {
	return s.mailboxesPage("", filter, page, pageSize)
}

func (s *PostgresStore) mailboxesPage(pool string, filter MailboxFilter, page, pageSize int) ([]domain.Mailbox, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	query := s.db.Model(&sqlMailbox{})
	if pool != "" {
		query = query.Where("pool = ?", pool)
	}
	if search := strings.TrimSpace(filter.Query); search != "" {
		query = query.Where("LOWER(address) LIKE ?", "%"+strings.ToLower(search)+"%")
	}
	query.Count(&total)
	var rows []sqlMailbox
	query.Order("address ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	mailboxIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		mailboxIDs = append(mailboxIDs, row.ID)
	}
	var states []sqlMailboxService
	if len(mailboxIDs) > 0 {
		s.db.Where("mailbox_id IN ?", mailboxIDs).Find(&states)
	}
	stateMap := make(map[string]map[string]domain.MailboxService)
	consumedServiceIDs := make(map[string]struct{})
	for _, state := range states {
		if stateMap[state.MailboxID] == nil {
			stateMap[state.MailboxID] = make(map[string]domain.MailboxService)
		}
		stateMap[state.MailboxID][state.ServiceID] = domain.MailboxService{ServiceID: state.ServiceID, State: domain.ServiceMailboxState(state.State), TimeoutCount: state.TimeoutCount, ChangedAt: state.ChangedAt}
		if domain.ServiceMailboxState(state.State) == domain.ServiceConsumed {
			consumedServiceIDs[state.ServiceID] = struct{}{}
		}
	}
	serviceCodes := make([]sqlService, 0, len(consumedServiceIDs))
	if len(consumedServiceIDs) > 0 {
		ids := make([]string, 0, len(consumedServiceIDs))
		for id := range consumedServiceIDs {
			ids = append(ids, id)
		}
		s.db.Select("id", "code", "name").Where("id IN ?", ids).Order("name ASC").Find(&serviceCodes)
	}
	items := make([]domain.Mailbox, 0, len(rows))
	for _, row := range rows {
		mailbox := mapMailbox(row, stateMap[row.ID])
		mailbox.RegisteredPlatforms = registeredPlatforms(stateMap[row.ID], serviceCodes)
		items = append(items, mailbox)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Address < items[j].Address })
	return items, total
}

func registeredPlatforms(states map[string]domain.MailboxService, services []sqlService) []string {
	result := make([]string, 0)
	for _, service := range services {
		if state, ok := states[service.ID]; ok && state.State == domain.ServiceConsumed {
			result = append(result, service.Code)
		}
	}
	return result
}

func (s *PostgresStore) updateOrder(id, userID string, allowed []domain.OrderStatus, mutate func(*sqlOrder, time.Time)) (domain.Order, error) {
	var result domain.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order sqlOrder
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if err := query.First(&order).Error; err != nil {
			return ErrOrderNotFound
		}
		valid := false
		for _, status := range allowed {
			if domain.OrderStatus(order.Status) == status {
				valid = true
				break
			}
		}
		if !valid {
			return ErrInvalidOrderState
		}
		mutate(&order, time.Now().UTC())
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		event := "order." + order.Status
		if err := s.enqueueOrderWebhook(tx, order, event); err != nil {
			return err
		}
		result = mapOrder(order)
		return nil
	})
	return result, err
}

func (s *PostgresStore) refundOrder(tx *gorm.DB, order *sqlOrder, status domain.OrderStatus) error {
	if order.Refunded {
		return nil
	}
	var user sqlUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", order.UserID).Error; err != nil {
		return err
	}
	user.BalanceCents += order.PriceCents
	if err := tx.Model(&user).Update("balance_cents", user.BalanceCents).Error; err != nil {
		return err
	}
	order.Status, order.Refunded = string(status), true
	if err := tx.Save(order).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := tx.Model(&sqlMailbox{}).Where("id = ?", order.MailboxID).Updates(map[string]any{"state": domain.MailboxAvailable, "active_order_id": ""}).Error; err != nil {
		return err
	}
	var mailboxService sqlMailboxService
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("mailbox_id = ? AND service_id = ?", order.MailboxID, order.ServiceID).First(&mailboxService).Error; err != nil {
		return err
	}
	if domain.ServiceMailboxState(mailboxService.State) == domain.ServiceLeased {
		mailboxService.State = string(domain.ServiceAvailable)
		mailboxService.ChangedAt = now
		if status == domain.OrderExpiredRefunded {
			nextCount, nextState := nextTimeoutState(mailboxService.TimeoutCount)
			mailboxService.TimeoutCount = nextCount
			mailboxService.State = string(nextState)
		}
		if err := tx.Save(&mailboxService).Error; err != nil {
			return err
		}
		if domain.ServiceMailboxState(mailboxService.State) == domain.ServiceConsumed {
			detail := fmt.Sprintf("累计 %d 次订单超时未收到验证码，停止向该平台分配", mailboxService.TimeoutCount)
			if err := tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: "system", Action: "mailbox.service.timeout_consume", ResourceType: "mailbox_service_state", ResourceID: order.MailboxID + ":" + order.ServiceID, Detail: detail}).Error; err != nil {
				return err
			}
		}
	}
	description := "管理员取消订单退款"
	if status == domain.OrderExpiredRefunded {
		description = "30 分钟未收到验证码自动退款"
		if strings.TrimSpace(order.FailureReason) != "" {
			description = order.FailureReason
		}
	}
	ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, OrderID: order.ID, Type: "order_refund", AmountCents: order.PriceCents, BalanceAfterCents: user.BalanceCents, Description: description}
	if err := tx.Create(&ledger).Error; err != nil {
		return err
	}
	if status == domain.OrderExpiredRefunded {
		action := "order.timeout_refund"
		if strings.TrimSpace(order.FailureReason) != "" {
			action = "order.mailbox_verification_refund"
		}
		if err := tx.Create(&sqlAuditLog{ID: uuid.NewString(), ActorID: "system", Action: action, ResourceType: "registration_order", ResourceID: order.ID, Detail: description}).Error; err != nil {
			return err
		}
	}
	return s.enqueueOrderWebhook(tx, *order, "order."+order.Status)
}

func (s *PostgresStore) releaseLock(ctx context.Context, key, value string) {
	script := redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
	_ = script.Run(ctx, s.redis, []string{key}, value).Err()
}

func mapUser(row sqlUser) domain.User {
	return domain.User{ID: row.ID, Email: row.Email, Balance: float64(row.BalanceCents) / 100, Role: row.Role, Status: row.Status, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt}
}

func mapService(row sqlService) domain.Service {
	prices := effectiveProviderPricesCents(row)
	providerPrices := make(map[string]float64, len(prices))
	for provider, price := range prices {
		providerPrices[provider] = float64(price) / 100
	}
	return domain.Service{ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description, Enabled: row.Enabled, AllowedProviders: append([]string(nil), row.AllowedProviders...), ProviderPrices: providerPrices, TTLSeconds: effectiveOrderTTLSeconds(row.TTLSeconds), SenderDomains: append([]string(nil), row.SenderDomains...), SubjectKeywords: append([]string(nil), row.SubjectKeywords...), Regex: row.Regex}
}

func mapMailbox(row sqlMailbox, states map[string]domain.MailboxService) domain.Mailbox {
	mailbox := domain.Mailbox{ID: row.ID, Address: row.Address, Provider: row.Provider, Pool: row.Pool, State: domain.MailboxState(row.State), HealthScore: row.HealthScore, OAuthValidUntil: row.OAuthValidUntil, ActiveOrderID: row.ActiveOrderID, TodayCodes: row.TodayCodes, ConnectionMethod: row.ConnectionMethod, VerificationStatus: row.VerificationStatus, VerificationError: compactStoredMailboxVerificationError(row.VerificationError), Services: states}
	if row.LastReceivedAt != nil {
		mailbox.LastReceivedAt = *row.LastReceivedAt
	}
	if row.LastVerifiedAt != nil {
		mailbox.LastVerifiedAt = *row.LastVerifiedAt
	}
	return mailbox
}

// compactStoredMailboxVerificationError 兼容压缩旧版本已经写入数据库的 Graph 原始错误。
func compactStoredMailboxVerificationError(message string) string {
	if strings.Contains(message, "AADSTS70000") || strings.Contains(strings.ToLower(message), "invalid_grant") {
		if marker := "；IMAP："; strings.Contains(message, marker) {
			imapDetail := strings.TrimSpace(message[strings.Index(message, marker)+len(marker):])
			if imapDetail == "" || imapDetail == "缺少 IMAP 凭证" {
				return "Graph：OAuth 授权已失效；未提供可用的 IMAP 密码或应用专用密码，无法自动切换"
			}
			if len(imapDetail) > 180 {
				imapDetail = imapDetail[:180]
			}
			return "Graph：OAuth 授权已失效；IMAP 自动切换失败：" + imapDetail
		}
		return "Graph：Microsoft OAuth 授权已失效（invalid_grant），请重新授权 Graph 或切换 IMAP"
	}
	if strings.Contains(message, "AADSTS65001") {
		return "Graph：Microsoft OAuth 尚未授予所需权限，请重新授权 Graph"
	}
	if len(message) > 240 {
		return message[:240]
	}
	return message
}

func mapOrder(row sqlOrder) domain.Order {
	order := domain.Order{ID: row.ID, UserID: row.UserID, ServiceID: row.ServiceID, ServiceCode: row.ServiceCode, ServiceName: row.ServiceName, MailboxID: row.MailboxID, MailboxAddress: row.MailboxAddress, MailboxProvider: row.MailboxProvider, RequestedProviders: append([]string(nil), row.RequestedProviders...), Status: domain.OrderStatus(row.Status), Code: row.Code, Price: float64(row.PriceCents) / 100, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Refunded: row.Refunded, RequestID: row.RequestID, FailureReason: row.FailureReason}
	if row.AssignedAt != nil {
		order.AssignedAt = *row.AssignedAt
	}
	if row.SubmittedAt != nil {
		order.SubmittedAt = *row.SubmittedAt
	}
	if row.CodeReceivedAt != nil {
		order.CodeReceivedAt = *row.CodeReceivedAt
	}
	if row.CompletedAt != nil {
		order.CompletedAt = *row.CompletedAt
	}
	return order
}

func cents(value float64) int64 { return int64(math.Round(value * 100)) }

func providerPricesCents(prices map[string]float64) map[string]int64 {
	result := make(map[string]int64, len(prices))
	for provider, price := range prices {
		result[provider] = cents(price)
	}
	return result
}

func effectiveProviderPricesCents(service sqlService) map[string]int64 {
	result := make(map[string]int64, len(service.AllowedProviders))
	for provider, price := range service.ProviderPricesCents {
		result[provider] = price
	}
	for _, provider := range service.AllowedProviders {
		if _, exists := result[provider]; !exists {
			result[provider] = service.PriceCents
		}
	}
	return result
}

func minimumProviderPriceCents(prices map[string]float64) int64 {
	minimum := int64(0)
	first := true
	for _, price := range prices {
		value := cents(price)
		if first || value < minimum {
			minimum, first = value, false
		}
	}
	return minimum
}
