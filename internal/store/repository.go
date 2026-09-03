package store

import (
	"context"

	"github.com/ljunn/heromail/internal/domain"
)

type Repository interface {
	User(id string) (domain.User, bool)
	ListServices() []domain.Service
	ListServicesPage(page, pageSize int) ([]domain.Service, int64)
	ListEnabledServicesPage(page, pageSize int) ([]domain.Service, int64)
	EnabledService(codeOrID string) (domain.Service, bool)
	ServiceUsage(serviceIDs []string) map[string]ServiceUsage
	ServiceAvailability(serviceIDs []string) map[string]int
	ServiceAvailabilityByProvider(serviceIDs []string) map[string]map[string]int
	CreateOrder(userID, serviceID, requestID string, mailboxProviders []string) (domain.Order, error)
	GetOrder(id string) (domain.Order, bool)
	ListOrders(userID string) []domain.Order
	ListOrdersPage(userID string, page, pageSize int) ([]domain.Order, int64)
	CompleteOrder(id, userID string) (domain.Order, error)
	CancelOrder(id, userID string) (domain.Order, error)
	ReapExpired() int
	Overview() domain.Overview
	Mailboxes() []domain.Mailbox
	MailboxesPage(page, pageSize int) ([]domain.Mailbox, int64)
}

// InvalidMailboxOrderRepository 用于回收已经绑定到未验证邮箱的存量订单。
// 这是可选接口，避免影响只实现基础订单能力的测试适配器。
type InvalidMailboxOrderRepository interface {
	ReconcileInvalidMailboxOrders() int
}

// MailboxFilter 是管理员邮箱资源列表的服务端筛选条件。
type MailboxFilter struct {
	Query string
	// Status 只接受管理员列表使用的有限筛选值，避免把内部凭证或规则
	// 暴露成任意查询条件。
	Status string
}

// MailboxSearchRepository 为邮箱列表提供服务端搜索，避免前端只在当前页筛选。
type MailboxSearchRepository interface {
	ListMailboxesPage(filter MailboxFilter, page, pageSize int) ([]domain.Mailbox, int64)
}

// InvalidMailboxSummary 描述认证失败邮箱的可操作范围。
// Deletable 只包含没有活跃订单、同时满足 failed/auth_error 的邮箱；
// 其余状态始终不会被批量删除。
type InvalidMailboxSummary struct {
	AuthErrors      int64 `json:"auth_errors"`
	Deletable       int64 `json:"deletable"`
	ProtectedActive int64 `json:"protected_active"`
	Pending         int64 `json:"pending"`
	Verified        int64 `json:"verified"`
}

// MailboxMaintenanceRepository 提供带预览和数量校验的失效邮箱维护能力。
// 这是可选接口，避免影响只实现基础邮箱资源的测试适配器。
type MailboxMaintenanceRepository interface {
	InvalidMailboxSummary() (InvalidMailboxSummary, error)
	DeleteInvalidMailboxes(actorID, ip string, expected int64) (int64, error)
}

// MailboxHistoryScanResult 是批量历史扫描入队结果。扫描由后台 Worker
// 异步执行，接口只负责把符合条件的邮箱安全地提交到去重队列。
type MailboxHistoryScanResult struct {
	ServiceCode string `json:"service_code"`
	Eligible    int64  `json:"eligible"`
	Queued      int64  `json:"queued"`
	Async       bool   `json:"async"`
}

// MailboxHistoryScanAdminRepository 用于按目标平台批量提交历史扫描。
type MailboxHistoryScanAdminRepository interface {
	QueueMailboxHistoryScans(ctx context.Context, serviceCode string) (MailboxHistoryScanResult, error)
}

type ServiceUsage struct {
	Available int
	Leased    int
	Consumed  int
}

type AdminOrderFilter struct {
	Status  string
	Service string
	UserID  string
	Query   string
}

type UserOrderFilter struct {
	Status  string
	Service string
	Query   string
}

type UserOrderFilterRepository interface {
	ListUserOrdersPage(userID string, filter UserOrderFilter, page, pageSize int) ([]domain.Order, int64)
}

type AdminOrderRepository interface {
	ListAdminOrdersPage(filter AdminOrderFilter, page, pageSize int) ([]domain.Order, int64)
}

type HealthRepository interface {
	Ping(context.Context) error
	StorageName() string
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func effectiveOrderTTLSeconds(_ int) int {
	return domain.MinimumOrderTTLSeconds
}

func nextTimeoutState(current int) (int, domain.ServiceMailboxState) {
	next := current + 1
	if next >= domain.MailboxServiceTimeoutLimit {
		return next, domain.ServiceConsumed
	}
	return next, domain.ServiceAvailable
}
