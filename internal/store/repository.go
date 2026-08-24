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
	CreateOrder(userID, serviceID, requestID string) (domain.Order, error)
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
