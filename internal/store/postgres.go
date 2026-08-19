package store

import (
	"context"
	"encoding/hex"
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
	if err := s.db.AutoMigrate(&sqlUser{}, &sqlSession{}, &sqlAPIKey{}, &sqlService{}, &sqlMailboxPool{}, &sqlMailbox{}, &sqlMailboxService{}, &sqlOrder{}, &sqlWalletLedger{}, &sqlPaymentProvider{}, &sqlPaymentOrder{}, &sqlMailEvent{}, &sqlWebhookEndpoint{}, &sqlWebhookDelivery{}, &sqlAuditLog{}); err != nil {
		return fmt.Errorf("执行数据库迁移失败：%w", err)
	}
	return nil
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
	services := []sqlService{
		{ID: "svc-github", Code: "github", Name: "GitHub", Description: "开发者平台", Enabled: true, AllowedProviders: append([]string(nil), domain.SupportedMailboxProviders...), PriceCents: 35, TTLSeconds: 600, SenderDomains: []string{"github.com"}, SubjectKeywords: []string{"verification", "验证码"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-openai", Code: "openai", Name: "OpenAI", Description: "人工智能平台", Enabled: true, AllowedProviders: append([]string(nil), domain.SupportedMailboxProviders...), PriceCents: 60, TTLSeconds: 600, SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-discord", Code: "discord", Name: "Discord", Description: "社区平台", Enabled: true, AllowedProviders: append([]string(nil), domain.SupportedMailboxProviders...), PriceCents: 30, TTLSeconds: 600, SenderDomains: []string{"discord.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-telegram", Code: "telegram", Name: "Telegram", Description: "通讯平台", Enabled: true, AllowedProviders: append([]string(nil), domain.SupportedMailboxProviders...), PriceCents: 25, TTLSeconds: 600, SenderDomains: []string{"telegram.org"}, SubjectKeywords: []string{"login code", "code"}, Regex: `\b(\d{5})\b`},
	}
	for _, service := range services {
		if err := s.db.Where("id = ?", service.ID).FirstOrCreate(&service).Error; err != nil {
			return err
		}
	}
	if err := s.ensureDefaultMailboxPool(); err != nil {
		return err
	}
	if config.SeedDemo {
		return s.seedDemoMailboxes(services)
	}
	return nil
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
				if err := tx.Model(&services[index]).Update("allowed_providers", services[index].AllowedProviders).Error; err != nil {
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
		s.db.Model(&sqlMailboxService{}).Select("service_id, state, count(*) AS count").Where("service_id IN ?", serviceIDs).Group("service_id, state").Scan(&rows)
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
	type availabilityRow struct {
		ServiceID string
		Count     int
	}
	result := make(map[string]int, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		result[serviceID] = 0
	}
	if len(serviceIDs) == 0 {
		return result
	}

	var rows []availabilityRow
	s.db.Table("mailbox_service_states AS mss").
		Select("mss.service_id, COUNT(*) AS count").
		Joins("JOIN target_services AS ts ON ts.id = mss.service_id").
		Joins("JOIN mailboxes AS m ON m.id = mss.mailbox_id").
		Joins("JOIN mailbox_pools AS mp ON mp.name = m.pool AND mp.enabled = ?", true).
		Where("mss.service_id IN ? AND ts.enabled = ?", serviceIDs, true).
		Where("m.state = ? AND m.active_order_id = '' AND m.health_score >= ? AND m.verification_status = ?", domain.MailboxAvailable, 60, domain.MailboxVerificationVerified).
		Where("(m.connection_method = ? OR m.oauth_valid_until > ?)", domain.MailboxConnectionIMAP, time.Now()).
		Where("mss.state = ?", domain.ServiceAvailable).
		Where("ts.allowed_providers @> jsonb_build_array(m.provider)").
		Where("m.last_received_at IS NULL OR m.last_received_at <= NOW() - (mp.cooldown_seconds * INTERVAL '1 second')").
		Where("mp.daily_limit <= 0 OR (SELECT COALESCE(SUM(CASE WHEN inner_m.last_received_at::date = CURRENT_DATE THEN inner_m.today_codes ELSE 0 END), 0) FROM mailboxes AS inner_m WHERE inner_m.pool = mp.name) < mp.daily_limit").
		Group("mss.service_id").
		Scan(&rows)
	for _, row := range rows {
		result[row.ServiceID] = row.Count
	}
	return result
}

func (s *PostgresStore) CreateOrder(userID, serviceID, requestID string) (domain.Order, error) {
	ctx := context.Background()
	lockKey := "heromail:allocate:" + serviceID
	lockValue := uuid.NewString()
	locked, err := s.redis.SetNX(ctx, lockKey, lockValue, 15*time.Second).Result()
	if err != nil {
		return domain.Order{}, err
	}
	if !locked {
		return domain.Order{}, ErrNoMailboxAvailable
	}
	defer s.releaseLock(ctx, lockKey, lockValue)

	var result domain.Order
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var service sqlService
		if err := tx.Where("id = ? OR code = ?", serviceID, serviceID).First(&service).Error; err != nil {
			return ErrServiceNotFound
		}
		if !service.Enabled {
			return ErrServiceDisabled
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
		if user.BalanceCents < service.PriceCents {
			return ErrInsufficientBalance
		}
		var mailbox sqlMailbox
		query := tx.Table("mailboxes AS m").Select("m.*").
			Joins("JOIN mailbox_service_states AS mss ON mss.mailbox_id = m.id AND mss.service_id = ?", service.ID).
			Joins("JOIN mailbox_pools AS mp ON mp.name = m.pool AND mp.enabled = ?", true).
			Where("m.state = ? AND m.active_order_id = '' AND m.health_score >= ? AND m.verification_status = ?", domain.MailboxAvailable, 60, domain.MailboxVerificationVerified).
			Where("(m.connection_method = ? OR m.oauth_valid_until > ?)", domain.MailboxConnectionIMAP, time.Now()).
			Where("mss.state = ?", domain.ServiceAvailable).
			Where("m.provider IN ?", service.AllowedProviders).
			Where("m.last_received_at IS NULL OR m.last_received_at <= NOW() - (mp.cooldown_seconds * INTERVAL '1 second')").
			Where("mp.daily_limit <= 0 OR (SELECT COALESCE(SUM(CASE WHEN inner_m.last_received_at::date = CURRENT_DATE THEN inner_m.today_codes ELSE 0 END), 0) FROM mailboxes AS inner_m WHERE inner_m.pool = mp.name) < mp.daily_limit").
			Order("m.health_score DESC, m.id ASC").
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "m"}, Options: "SKIP LOCKED"}).
			Limit(1).Scan(&mailbox)
		if query.Error != nil || mailbox.ID == "" {
			return ErrNoMailboxAvailable
		}
		now := time.Now().UTC()
		orderID := "ORD" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:16])
		order := sqlOrder{ID: orderID, UserID: user.ID, ServiceID: service.ID, ServiceCode: service.Code, ServiceName: service.Name, MailboxID: mailbox.ID, MailboxAddress: mailbox.Address, Status: string(domain.OrderAssigned), PriceCents: service.PriceCents, CreatedAt: now, AssignedAt: &now, ExpiresAt: now.Add(time.Duration(service.TTLSeconds) * time.Second), RequestID: requestID}
		user.BalanceCents -= service.PriceCents
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
		ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, OrderID: order.ID, Type: "order_reserve", AmountCents: -service.PriceCents, BalanceAfterCents: user.BalanceCents, Description: "注册订单预扣"}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		if err := s.enqueueOrderWebhook(tx, order, "order.assigned"); err != nil {
			return err
		}
		result = mapOrder(order)
		return nil
	})
	return result, err
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

func (s *PostgresStore) SubmitOrder(id, userID string) (domain.Order, error) {
	return s.updateOrder(id, userID, []domain.OrderStatus{domain.OrderAssigned}, func(order *sqlOrder, now time.Time) {
		order.Status, order.SubmittedAt = string(domain.OrderWaitingCode), &now
	})
}

func (s *PostgresStore) ReceiveCode(id string) (domain.Order, error) {
	return s.ReceiveCodeValue(id, "")
}

func (s *PostgresStore) ReceiveCodeValue(id, code string) (domain.Order, error) {
	var result domain.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order sqlOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, "id = ?", id).Error; err != nil {
			return ErrOrderNotFound
		}
		if domain.OrderStatus(order.Status) != domain.OrderWaitingCode {
			return ErrInvalidOrderState
		}
		if code == "" {
			code = codeFor(order.ServiceCode)
		}
		now := time.Now().UTC()
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

func (s *PostgresStore) Overview() domain.Overview {
	var result domain.Overview
	var total, outlook, outlookDE, hotmail, pending, verified, available, leased, authErrors, blocked int64
	s.db.Model(&sqlMailbox{}).Count(&total)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderOutlook).Count(&outlook)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderOutlookDE).Count(&outlookDE)
	s.db.Model(&sqlMailbox{}).Where("provider = ?", domain.MailboxProviderHotmail).Count(&hotmail)
	s.db.Model(&sqlMailbox{}).Where("verification_status = ?", domain.MailboxVerificationPending).Count(&pending)
	s.db.Model(&sqlMailbox{}).Where("verification_status = ?", domain.MailboxVerificationVerified).Count(&verified)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxAvailable).Count(&available)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxLeased).Count(&leased)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxError).Count(&authErrors)
	s.db.Model(&sqlMailbox{}).Where("state = ?", domain.MailboxBlocked).Count(&blocked)
	result.AvailableMailboxes = int(available)
	result.TotalMailboxes = int(total)
	result.OutlookMailboxes = int(outlook)
	result.OutlookDEMailboxes = int(outlookDE)
	result.HotmailMailboxes = int(hotmail)
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
	return s.mailboxesPage("", page, pageSize)
}

func (s *PostgresStore) mailboxesPage(pool string, page, pageSize int) ([]domain.Mailbox, int64) {
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	query := s.db.Model(&sqlMailbox{})
	if pool != "" {
		query = query.Where("pool = ?", pool)
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
		stateMap[state.MailboxID][state.ServiceID] = domain.MailboxService{ServiceID: state.ServiceID, State: domain.ServiceMailboxState(state.State), ChangedAt: state.ChangedAt}
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
	if err := tx.Model(&sqlMailboxService{}).Where("mailbox_id = ? AND service_id = ? AND state = ?", order.MailboxID, order.ServiceID, domain.ServiceLeased).Updates(map[string]any{"state": domain.ServiceAvailable, "changed_at": now}).Error; err != nil {
		return err
	}
	ledger := sqlWalletLedger{ID: uuid.NewString(), UserID: user.ID, OrderID: order.ID, Type: "order_refund", AmountCents: order.PriceCents, BalanceAfterCents: user.BalanceCents, Description: "注册订单退款"}
	if err := tx.Create(&ledger).Error; err != nil {
		return err
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
	return domain.Service{ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description, Enabled: row.Enabled, AllowedProviders: append([]string(nil), row.AllowedProviders...), Price: float64(row.PriceCents) / 100, TTLSeconds: row.TTLSeconds, SenderDomains: append([]string(nil), row.SenderDomains...), SubjectKeywords: append([]string(nil), row.SubjectKeywords...), Regex: row.Regex}
}

func mapMailbox(row sqlMailbox, states map[string]domain.MailboxService) domain.Mailbox {
	mailbox := domain.Mailbox{ID: row.ID, Address: row.Address, Provider: row.Provider, Pool: row.Pool, State: domain.MailboxState(row.State), HealthScore: row.HealthScore, OAuthValidUntil: row.OAuthValidUntil, ActiveOrderID: row.ActiveOrderID, TodayCodes: row.TodayCodes, ConnectionMethod: row.ConnectionMethod, VerificationStatus: row.VerificationStatus, VerificationError: row.VerificationError, Services: states}
	if row.LastReceivedAt != nil {
		mailbox.LastReceivedAt = *row.LastReceivedAt
	}
	if row.LastVerifiedAt != nil {
		mailbox.LastVerifiedAt = *row.LastVerifiedAt
	}
	return mailbox
}

func mapOrder(row sqlOrder) domain.Order {
	order := domain.Order{ID: row.ID, UserID: row.UserID, ServiceID: row.ServiceID, ServiceCode: row.ServiceCode, ServiceName: row.ServiceName, MailboxID: row.MailboxID, MailboxAddress: row.MailboxAddress, Status: domain.OrderStatus(row.Status), Code: row.Code, Price: float64(row.PriceCents) / 100, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Refunded: row.Refunded, RequestID: row.RequestID, FailureReason: row.FailureReason}
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
