package store

import (
	"errors"

	"github.com/ljunn/heromail/internal/domain"
)

type PaymentProviderSecret struct {
	Provider domain.PaymentProvider
	Config   map[string]string
}

type PaymentRepository interface {
	ListPaymentProvidersPage(page, pageSize int) ([]domain.PaymentProvider, int64)
	ListEnabledPaymentProviders(method string) ([]PaymentProviderSecret, error)
	GetPaymentProviderSecret(id string) (PaymentProviderSecret, error)
	SavePaymentProvider(actorID string, provider domain.PaymentProvider, config map[string]string, ip string) (domain.PaymentProvider, error)
	DeletePaymentProvider(actorID, providerID, ip string) error
	CreatePaymentOrder(userID, providerID, method string, amount float64) (domain.PaymentOrder, error)
	SetPaymentOrderURL(orderID, payURL string) error
	GetPaymentOrder(userID, orderID string) (domain.PaymentOrder, bool)
	ListPaymentOrdersPage(userID string, page, pageSize int) ([]domain.PaymentOrder, int64)
	CancelPaymentOrder(userID, orderID string) (domain.PaymentOrder, error)
	CompletePaymentOrder(orderID, providerTradeNo string, paidAmount float64) (domain.PaymentOrder, error)
	ReapExpiredPaymentOrders() int
}

var (
	ErrPaymentProviderNotFound = errors.New("支付服务商不存在")
	ErrPaymentOrderNotFound    = errors.New("支付订单不存在")
	ErrPaymentAmountMismatch   = errors.New("支付金额不一致")
)
