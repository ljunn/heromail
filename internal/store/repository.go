package store

import (
	"context"

	"github.com/ljunn/heromail/internal/domain"
)

type Repository interface {
	User(id string) (domain.User, bool)
	ListServices() []domain.Service
	ListServicesPage(page, pageSize int) ([]domain.Service, int64)
	ServiceUsage(serviceIDs []string) map[string]ServiceUsage
	CreateOrder(userID, serviceID, requestID string) (domain.Order, error)
	GetOrder(id string) (domain.Order, bool)
	ListOrders(userID string) []domain.Order
	ListOrdersPage(userID string, page, pageSize int) ([]domain.Order, int64)
	SubmitOrder(id, userID string) (domain.Order, error)
	ReceiveCode(id string) (domain.Order, error)
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
