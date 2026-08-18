package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ljunn/heromail/internal/domain"
)

var (
	ErrServiceNotFound     = errors.New("target service not found")
	ErrServiceDisabled     = errors.New("target service is disabled")
	ErrNoMailboxAvailable  = errors.New("no mailbox available for this service")
	ErrOrderNotFound       = errors.New("order not found")
	ErrInvalidOrderState   = errors.New("order is not in a mutable state")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrDuplicateRequest    = errors.New("request_id already exists")
)

type Store struct {
	mu        sync.RWMutex
	users     map[string]*domain.User
	services  map[string]*domain.Service
	mailboxes map[string]*domain.Mailbox
	orders    map[string]*domain.Order
	seq       int64
}

func New() *Store {
	s := &Store{
		users: make(map[string]*domain.User), services: make(map[string]*domain.Service),
		mailboxes: make(map[string]*domain.Mailbox), orders: make(map[string]*domain.Order),
		seq: 1000,
	}
	s.seed()
	return s
}

func (s *Store) seed() {
	s.users["user-001"] = &domain.User{ID: "user-001", Email: "demo@example.com", Balance: 48.60, Role: "user"}
	s.users["admin-001"] = &domain.User{ID: "admin-001", Email: "admin@heromail.local", Balance: 0, Role: "admin"}

	services := []*domain.Service{
		{ID: "svc-github", Code: "github", Name: "GitHub", Description: "开发者平台", Enabled: true, AllowedProviders: []string{"outlook", "hotmail"}, Price: 0.35, TTLSeconds: 600, SenderDomains: []string{"github.com"}, SubjectKeywords: []string{"verification", "验证码"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-openai", Code: "openai", Name: "OpenAI", Description: "人工智能平台", Enabled: true, AllowedProviders: []string{"outlook", "hotmail"}, Price: 0.60, TTLSeconds: 600, SenderDomains: []string{"openai.com"}, SubjectKeywords: []string{"verification", "code"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-discord", Code: "discord", Name: "Discord", Description: "社区平台", Enabled: true, AllowedProviders: []string{"outlook", "hotmail"}, Price: 0.30, TTLSeconds: 600, SenderDomains: []string{"discord.com"}, SubjectKeywords: []string{"verification"}, Regex: `\b(\d{6})\b`},
		{ID: "svc-telegram", Code: "telegram", Name: "Telegram", Description: "通讯平台", Enabled: true, AllowedProviders: []string{"outlook", "hotmail"}, Price: 0.25, TTLSeconds: 600, SenderDomains: []string{"telegram.org"}, SubjectKeywords: []string{"login code", "code"}, Regex: `\b(\d{5})\b`},
	}
	for _, service := range services {
		s.services[service.ID] = service
	}

	providers := []string{"outlook", "hotmail"}
	for i := 1; i <= 24; i++ {
		provider := providers[(i-1)%len(providers)]
		mailDomain := "outlook.com"
		pool := "Outlook Pool A"
		if provider == "hotmail" {
			mailDomain, pool = "hotmail.com", "Hotmail Pool A"
		}
		id := fmt.Sprintf("mb-%03d", i)
		states := make(map[string]domain.MailboxService)
		for serviceID := range s.services {
			states[serviceID] = domain.MailboxService{ServiceID: serviceID, State: domain.ServiceAvailable, ChangedAt: time.Now()}
		}
		s.mailboxes[id] = &domain.Mailbox{ID: id, Address: fmt.Sprintf("hero_%02d@%s", i, mailDomain), Provider: provider, Pool: pool, State: domain.MailboxAvailable, HealthScore: 84 + i%16, OAuthValidUntil: time.Now().Add(30 * 24 * time.Hour), Services: states}
	}
}

func (s *Store) User(id string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, false
	}
	return *u, true
}

func (s *Store) ListServices() []domain.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Service, 0, len(s.services))
	for _, service := range s.services {
		copy := *service
		copy.AllowedProviders = append([]string(nil), service.AllowedProviders...)
		out = append(out, copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) ListServicesPage(page, pageSize int) ([]domain.Service, int64) {
	items := s.ListServices()
	return paginate(items, page, pageSize), int64(len(items))
}

func (s *Store) ListEnabledServicesPage(page, pageSize int) ([]domain.Service, int64) {
	services := s.ListServices()
	items := make([]domain.Service, 0, len(services))
	for _, service := range services {
		if service.Enabled {
			items = append(items, service)
		}
	}
	return paginate(items, page, pageSize), int64(len(items))
}

func (s *Store) ServiceUsage(serviceIDs []string) map[string]ServiceUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]ServiceUsage, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		usage := ServiceUsage{}
		for _, mailbox := range s.mailboxes {
			switch mailbox.Services[serviceID].State {
			case domain.ServiceAvailable:
				usage.Available++
			case domain.ServiceLeased:
				usage.Leased++
			case domain.ServiceConsumed:
				usage.Consumed++
			}
		}
		result[serviceID] = usage
	}
	return result
}

func (s *Store) CreateOrder(userID, serviceID, requestID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok {
		return domain.Order{}, errors.New("user not found")
	}
	service, ok := s.services[serviceID]
	if !ok {
		return domain.Order{}, ErrServiceNotFound
	}
	if !service.Enabled {
		return domain.Order{}, ErrServiceDisabled
	}
	if requestID != "" {
		for _, existing := range s.orders {
			if existing.UserID == userID && existing.RequestID == requestID {
				return cloneOrder(*existing), ErrDuplicateRequest
			}
		}
	}
	if user.Balance < service.Price {
		return domain.Order{}, ErrInsufficientBalance
	}

	var selected *domain.Mailbox
	for _, mailbox := range s.mailboxes {
		if mailbox.State != domain.MailboxAvailable || mailbox.ActiveOrderID != "" || mailbox.HealthScore < 60 {
			continue
		}
		allowed := false
		for _, provider := range service.AllowedProviders {
			if provider == mailbox.Provider {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		state, exists := mailbox.Services[service.ID]
		if !exists || state.State != domain.ServiceAvailable {
			continue
		}
		if selected == nil || mailbox.HealthScore > selected.HealthScore {
			selected = mailbox
		}
	}
	if selected == nil {
		return domain.Order{}, ErrNoMailboxAvailable
	}
	now := time.Now()
	s.seq++
	order := &domain.Order{ID: fmt.Sprintf("ORD%06d", s.seq), UserID: userID, ServiceID: service.ID, ServiceCode: service.Code, ServiceName: service.Name, MailboxID: selected.ID, MailboxAddress: selected.Address, Status: domain.OrderAssigned, Price: service.Price, CreatedAt: now, AssignedAt: now, ExpiresAt: now.Add(time.Duration(service.TTLSeconds) * time.Second), RequestID: requestID}
	user.Balance -= service.Price
	selected.State, selected.ActiveOrderID = domain.MailboxLeased, order.ID
	state := selected.Services[service.ID]
	state.State, state.ChangedAt = domain.ServiceLeased, now
	selected.Services[service.ID] = state
	s.orders[order.ID] = order
	return cloneOrder(*order), nil
}

func (s *Store) GetOrder(id string) (domain.Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order, ok := s.orders[id]
	if !ok {
		return domain.Order{}, false
	}
	return cloneOrder(*order), true
}

func (s *Store) ListOrders(userID string) []domain.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Order, 0)
	for _, order := range s.orders {
		if userID == "" || order.UserID == userID {
			out = append(out, cloneOrder(*order))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) ListOrdersPage(userID string, page, pageSize int) ([]domain.Order, int64) {
	items := s.ListOrders(userID)
	return paginate(items, page, pageSize), int64(len(items))
}

func (s *Store) SubmitOrder(id, userID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || (userID != "" && order.UserID != userID) {
		return domain.Order{}, ErrOrderNotFound
	}
	if order.Status != domain.OrderAssigned {
		return domain.Order{}, ErrInvalidOrderState
	}
	order.Status, order.SubmittedAt = domain.OrderWaitingCode, time.Now()
	return cloneOrder(*order), nil
}

func (s *Store) ReceiveCode(id string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok {
		return domain.Order{}, ErrOrderNotFound
	}
	if order.Status != domain.OrderWaitingCode {
		return domain.Order{}, ErrInvalidOrderState
	}
	now := time.Now()
	order.Status, order.Code, order.CodeReceivedAt = domain.OrderCodeReceived, codeFor(order.ServiceCode), now
	if mailbox := s.mailboxes[order.MailboxID]; mailbox != nil {
		mailbox.TodayCodes++
		mailbox.LastReceivedAt = now
		mailbox.ActiveOrderID = ""
		mailbox.State = domain.MailboxAvailable
		state := mailbox.Services[order.ServiceID]
		state.State, state.ChangedAt = domain.ServiceConsumed, now
		mailbox.Services[order.ServiceID] = state
	}
	return cloneOrder(*order), nil
}

func (s *Store) CompleteOrder(id, userID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || (userID != "" && order.UserID != userID) {
		return domain.Order{}, ErrOrderNotFound
	}
	if order.Status != domain.OrderCodeReceived {
		return domain.Order{}, ErrInvalidOrderState
	}
	order.Status, order.CompletedAt = domain.OrderCompleted, time.Now()
	return cloneOrder(*order), nil
}

func (s *Store) CancelOrder(id, userID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || (userID != "" && order.UserID != userID) {
		return domain.Order{}, ErrOrderNotFound
	}
	if order.Status != domain.OrderAssigned && order.Status != domain.OrderWaitingCode {
		return domain.Order{}, ErrInvalidOrderState
	}
	order.Status, order.Refunded = domain.OrderCanceled, true
	s.refundAndReleaseLocked(order)
	return cloneOrder(*order), nil
}

func (s *Store) ReapExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	count := 0
	for _, order := range s.orders {
		if (order.Status == domain.OrderAssigned || order.Status == domain.OrderWaitingCode) && now.After(order.ExpiresAt) {
			order.Status, order.Refunded = domain.OrderExpiredRefunded, true
			s.refundAndReleaseLocked(order)
			count++
		}
	}
	return count
}

func (s *Store) Overview() domain.Overview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result domain.Overview
	var codeTotal, codeSeconds float64
	now := time.Now()
	for _, mailbox := range s.mailboxes {
		if mailbox.State == domain.MailboxAvailable {
			result.AvailableMailboxes++
		}
		if mailbox.State == domain.MailboxLeased {
			result.ActiveLeases++
		}
		if mailbox.State == domain.MailboxError {
			result.AuthErrors++
		}
		if mailbox.State == domain.MailboxBlocked {
			result.BlockedMailboxes++
		}
	}
	for _, order := range s.orders {
		if order.CreatedAt.Year() == now.Year() && order.CreatedAt.YearDay() == now.YearDay() {
			result.TodayOrders++
			if order.Status == domain.OrderCompleted || order.Status == domain.OrderCodeReceived {
				result.TodayRevenue += order.Price
			}
			if !order.CodeReceivedAt.IsZero() {
				codeTotal++
				codeSeconds += order.CodeReceivedAt.Sub(order.SubmittedAt).Seconds()
			}
		}
	}
	if codeTotal > 0 {
		result.AverageCodeSeconds = codeSeconds / codeTotal
		result.SuccessRate = codeTotal / float64(result.TodayOrders) * 100
	} else {
		result.AverageCodeSeconds = 23.5
		result.SuccessRate = 98.65
	}
	return result
}

func (s *Store) Mailboxes() []domain.Mailbox {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Mailbox, 0, len(s.mailboxes))
	for _, mailbox := range s.mailboxes {
		out = append(out, cloneMailbox(*mailbox))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

func (s *Store) MailboxesPage(page, pageSize int) ([]domain.Mailbox, int64) {
	items := s.Mailboxes()
	return paginate(items, page, pageSize), int64(len(items))
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) StorageName() string        { return "memory" }

func (s *Store) refundAndReleaseLocked(order *domain.Order) {
	if user := s.users[order.UserID]; user != nil && order.Refunded {
		user.Balance += order.Price
	}
	if mailbox := s.mailboxes[order.MailboxID]; mailbox != nil {
		mailbox.ActiveOrderID = ""
		mailbox.State = domain.MailboxAvailable
		state := mailbox.Services[order.ServiceID]
		if state.State == domain.ServiceLeased {
			state.State, state.ChangedAt = domain.ServiceAvailable, time.Now()
			mailbox.Services[order.ServiceID] = state
		}
	}
}

func codeFor(service string) string {
	if service == "svc-telegram" {
		return "84271"
	}
	return "842729"
}

func cloneOrder(order domain.Order) domain.Order { return order }
func cloneMailbox(mailbox domain.Mailbox) domain.Mailbox {
	services := make(map[string]domain.MailboxService, len(mailbox.Services))
	for k, v := range mailbox.Services {
		services[k] = v
	}
	mailbox.Services = services
	return mailbox
}

func IsUserVisible(status domain.OrderStatus) bool {
	return !strings.HasSuffix(string(status), "internal")
}

func paginate[T any](items []T, page, pageSize int) []T {
	page, pageSize = normalizePage(page, pageSize)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
