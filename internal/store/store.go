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
	ErrServiceNotFound          = errors.New("target service not found")
	ErrServiceDisabled          = errors.New("target service is disabled")
	ErrNoMailboxAvailable       = errors.New("no mailbox available for this service")
	ErrOrderNotFound            = errors.New("order not found")
	ErrInvalidOrderState        = errors.New("order is not in a mutable state")
	ErrVerificationCodeRequired = errors.New("verification code is required")
	ErrInsufficientBalance      = errors.New("insufficient balance")
	ErrDuplicateRequest         = errors.New("request_id already exists")
	ErrInvalidMailboxProviders  = errors.New("mailbox_providers 必须是目标平台允许且已定价的邮箱类型")
	ErrMailboxServiceLeased     = errors.New("邮箱在该目标平台存在进行中的订单")
	ErrMailboxServiceNotFound   = errors.New("邮箱目标平台状态不存在")
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

	for _, service := range defaultServices() {
		serviceCopy := service
		s.services[service.ID] = &serviceCopy
	}

	providers := []string{domain.MailboxProviderOutlook, domain.MailboxProviderHotmail}
	for i := 1; i <= 24; i++ {
		provider := providers[(i-1)%len(providers)]
		mailDomain := "outlook.com"
		pool := domain.DefaultMailboxPoolName
		if provider == "hotmail" {
			mailDomain = "hotmail.com"
		}
		id := fmt.Sprintf("mb-%03d", i)
		states := make(map[string]domain.MailboxService)
		for serviceID := range s.services {
			states[serviceID] = domain.MailboxService{ServiceID: serviceID, State: domain.ServiceAvailable, ChangedAt: time.Now()}
		}
		s.mailboxes[id] = &domain.Mailbox{ID: id, Address: fmt.Sprintf("hero_%02d@%s", i, mailDomain), Provider: provider, Pool: pool, State: domain.MailboxAvailable, HealthScore: 84 + i%16, OAuthValidUntil: time.Now().Add(30 * 24 * time.Hour), ConnectionMethod: domain.MailboxConnectionMicrosoftOAuth, VerificationStatus: domain.MailboxVerificationVerified, LastVerifiedAt: time.Now(), RegisteredPlatforms: []string{}, Services: states}
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
		out = append(out, cloneService(*service))
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

func (s *Store) EnabledService(codeOrID string) (domain.Service, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, service := range s.services {
		if service.Enabled && (service.ID == codeOrID || service.Code == codeOrID) {
			return cloneService(*service), true
		}
	}
	return domain.Service{}, false
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

func (s *Store) ServiceAvailability(serviceIDs []string) map[string]int {
	byProvider := s.ServiceAvailabilityByProvider(serviceIDs)
	result := make(map[string]int, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		for _, count := range byProvider[serviceID] {
			result[serviceID] += count
		}
	}
	return result
}

func (s *Store) ServiceAvailabilityByProvider(serviceIDs []string) map[string]map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	result := make(map[string]map[string]int, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		result[serviceID] = map[string]int{}
		service, exists := s.services[serviceID]
		if !exists || !service.Enabled {
			continue
		}
		for _, mailbox := range s.mailboxes {
			if mailbox.State != domain.MailboxAvailable || mailbox.ActiveOrderID != "" || mailbox.HealthScore < 60 || !mailboxConnectionValid(mailbox, now) {
				continue
			}
			if !contains(service.AllowedProviders, mailbox.Provider) {
				continue
			}
			if state, ok := mailbox.Services[serviceID]; ok && state.State == domain.ServiceAvailable {
				result[serviceID][mailbox.Provider]++
			}
		}
	}
	return result
}

func (s *Store) CreateOrder(userID, serviceID, requestID string, mailboxProviders []string) (domain.Order, error) {
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
	requestedProviders, maximumPrice, err := validateOrderProviders(*service, mailboxProviders)
	if err != nil {
		return domain.Order{}, err
	}
	if requestID != "" {
		for _, existing := range s.orders {
			if existing.UserID == userID && existing.RequestID == requestID {
				return cloneOrder(*existing), ErrDuplicateRequest
			}
		}
	}
	if user.Balance < maximumPrice {
		return domain.Order{}, ErrInsufficientBalance
	}

	now := time.Now()
	var selected *domain.Mailbox
	for _, mailbox := range s.mailboxes {
		if mailbox.State != domain.MailboxAvailable || mailbox.ActiveOrderID != "" || mailbox.HealthScore < 60 || !mailboxConnectionValid(mailbox, now) {
			continue
		}
		if !contains(requestedProviders, mailbox.Provider) {
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
	price := service.ProviderPrices[selected.Provider]
	s.seq++
	order := &domain.Order{ID: fmt.Sprintf("ORD%06d", s.seq), UserID: userID, ServiceID: service.ID, ServiceCode: service.Code, ServiceName: service.Name, MailboxID: selected.ID, MailboxAddress: selected.Address, MailboxProvider: selected.Provider, RequestedProviders: requestedProviders, Status: domain.OrderWaitingCode, Price: price, CreatedAt: now, AssignedAt: now, SubmittedAt: now, ExpiresAt: now.Add(time.Duration(effectiveOrderTTLSeconds(service.TTLSeconds)) * time.Second), RequestID: requestID}
	user.Balance -= price
	selected.State, selected.ActiveOrderID = domain.MailboxLeased, order.ID
	state := selected.Services[service.ID]
	state.State, state.ChangedAt = domain.ServiceLeased, now
	selected.Services[service.ID] = state
	s.orders[order.ID] = order
	return cloneOrder(*order), nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func mailboxConnectionValid(mailbox *domain.Mailbox, now time.Time) bool {
	return mailbox.ConnectionMethod == domain.MailboxConnectionIMAP || mailbox.OAuthValidUntil.After(now)
}

func validateOrderProviders(service domain.Service, requested []string) ([]string, float64, error) {
	providers := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	maximumPrice := 0.0
	for _, raw := range requested {
		provider := strings.ToLower(strings.TrimSpace(raw))
		if _, exists := seen[provider]; exists {
			continue
		}
		price, priced := service.ProviderPrices[provider]
		if provider == "" || !domain.IsSupportedMailboxProvider(provider) || !contains(service.AllowedProviders, provider) || !priced || price < 0 {
			return nil, 0, ErrInvalidMailboxProviders
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
		if price > maximumPrice {
			maximumPrice = price
		}
	}
	if len(providers) == 0 {
		return nil, 0, ErrInvalidMailboxProviders
	}
	return providers, maximumPrice, nil
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

func (s *Store) ListUserOrdersPage(userID string, filter UserOrderFilter, page, pageSize int) ([]domain.Order, int64) {
	items := s.ListOrders(userID)
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	filtered := make([]domain.Order, 0, len(items))
	for _, order := range items {
		if filter.Status != "" && string(order.Status) != filter.Status {
			continue
		}
		if filter.Service != "" && order.ServiceID != filter.Service && order.ServiceCode != filter.Service {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(order.ID+" "+order.ServiceName+" "+order.MailboxAddress), query) {
			continue
		}
		filtered = append(filtered, order)
	}
	page, pageSize = normalizePage(page, pageSize)
	return paginate(filtered, page, pageSize), int64(len(filtered))
}

func (s *Store) ListAdminOrdersPage(filter AdminOrderFilter, page, pageSize int) ([]domain.Order, int64) {
	items := s.ListOrders("")
	filtered := make([]domain.Order, 0, len(items))
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	for _, order := range items {
		if filter.Status != "" && string(order.Status) != filter.Status {
			continue
		}
		if filter.Service != "" && order.ServiceID != filter.Service && order.ServiceCode != filter.Service {
			continue
		}
		if filter.UserID != "" && order.UserID != filter.UserID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(order.ID+" "+order.UserID+" "+order.MailboxAddress), query) {
			continue
		}
		filtered = append(filtered, order)
	}
	page, pageSize = normalizePage(page, pageSize)
	return paginate(filtered, page, pageSize), int64(len(filtered))
}

func (s *Store) ReceiveCodeValue(id, code string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok {
		return domain.Order{}, ErrOrderNotFound
	}
	if order.Status != domain.OrderAssigned && order.Status != domain.OrderWaitingCode {
		return domain.Order{}, ErrInvalidOrderState
	}
	now := time.Now()
	if !order.ExpiresAt.IsZero() && !now.Before(order.ExpiresAt) {
		return domain.Order{}, ErrInvalidOrderState
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return domain.Order{}, ErrVerificationCodeRequired
	}
	order.Status, order.Code, order.CodeReceivedAt = domain.OrderCodeReceived, code, now
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
	s.refundAndReleaseLocked(order, false)
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
			s.refundAndReleaseLocked(order, true)
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
		result.TotalMailboxes++
		if mailbox.Provider == domain.MailboxProviderOutlook {
			result.OutlookMailboxes++
		}
		if mailbox.Provider == domain.MailboxProviderOutlookDE {
			result.OutlookDEMailboxes++
		}
		if mailbox.Provider == domain.MailboxProviderHotmail {
			result.HotmailMailboxes++
		}
		if mailbox.Provider == domain.MailboxProviderGmail {
			result.GmailMailboxes++
		}
		if mailbox.Provider == domain.MailboxProviderICloud {
			result.ICloudMailboxes++
		}
		if mailbox.Provider == domain.MailboxProviderMailCom {
			result.MailComMailboxes++
		}
		if mailbox.VerificationStatus == domain.MailboxVerificationPending {
			result.PendingMailboxes++
		}
		if mailbox.VerificationStatus == domain.MailboxVerificationVerified {
			result.VerifiedMailboxes++
		}
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
		item := cloneMailbox(*mailbox)
		item.RegisteredPlatforms = make([]string, 0)
		for _, service := range s.services {
			if state, ok := mailbox.Services[service.ID]; ok && state.State == domain.ServiceConsumed {
				item.RegisteredPlatforms = append(item.RegisteredPlatforms, service.Code)
			}
		}
		sort.Strings(item.RegisteredPlatforms)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Address < out[j].Address })
	return out
}

func (s *Store) MailboxesPage(page, pageSize int) ([]domain.Mailbox, int64) {
	return s.ListMailboxesPage(MailboxFilter{}, page, pageSize)
}

func (s *Store) ListMailboxesPage(filter MailboxFilter, page, pageSize int) ([]domain.Mailbox, int64) {
	items := s.Mailboxes()
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	if query != "" {
		filtered := make([]domain.Mailbox, 0, len(items))
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Address), query) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return paginate(items, page, pageSize), int64(len(items))
}

func (s *Store) MarkMailboxServiceConsumed(mailboxID, serviceID string, changedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mailbox, ok := s.mailboxes[mailboxID]
	if !ok {
		return errors.New("mailbox not found")
	}
	state, ok := mailbox.Services[serviceID]
	if !ok || state.State != domain.ServiceAvailable {
		return nil
	}
	if changedAt.IsZero() {
		changedAt = time.Now()
	}
	state.State = domain.ServiceConsumed
	state.ChangedAt = changedAt
	mailbox.Services[serviceID] = state
	return nil
}

func (s *Store) MarkMailboxServiceRegistered(_, mailboxID, serviceID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mailbox, ok := s.mailboxes[mailboxID]
	if !ok {
		return ErrMailboxNotFound
	}
	if _, ok := s.services[serviceID]; !ok {
		return ErrServiceNotFound
	}
	state, ok := mailbox.Services[serviceID]
	if !ok {
		return ErrMailboxServiceNotFound
	}
	if state.State == domain.ServiceLeased {
		return ErrMailboxServiceLeased
	}
	if state.State == domain.ServiceConsumed {
		return nil
	}
	state.State = domain.ServiceConsumed
	state.ChangedAt = time.Now().UTC()
	mailbox.Services[serviceID] = state
	return nil
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) StorageName() string        { return "memory" }

func (s *Store) refundAndReleaseLocked(order *domain.Order, timedOut bool) {
	if user := s.users[order.UserID]; user != nil && order.Refunded {
		user.Balance += order.Price
	}
	if mailbox := s.mailboxes[order.MailboxID]; mailbox != nil {
		mailbox.ActiveOrderID = ""
		mailbox.State = domain.MailboxAvailable
		state := mailbox.Services[order.ServiceID]
		if state.State == domain.ServiceLeased {
			state.State, state.ChangedAt = domain.ServiceAvailable, time.Now()
			if timedOut {
				state.TimeoutCount, state.State = nextTimeoutState(state.TimeoutCount)
			}
			mailbox.Services[order.ServiceID] = state
		}
	}
}

func cloneService(service domain.Service) domain.Service {
	service.AllowedProviders = append([]string(nil), service.AllowedProviders...)
	service.ProviderPrices = cloneProviderPrices(service.ProviderPrices)
	service.SenderDomains = append([]string(nil), service.SenderDomains...)
	service.SubjectKeywords = append([]string(nil), service.SubjectKeywords...)
	return service
}

func cloneProviderPrices(prices map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(prices))
	for provider, price := range prices {
		result[provider] = price
	}
	return result
}

func cloneOrder(order domain.Order) domain.Order {
	order.RequestedProviders = append([]string(nil), order.RequestedProviders...)
	return order
}
func cloneMailbox(mailbox domain.Mailbox) domain.Mailbox {
	services := make(map[string]domain.MailboxService, len(mailbox.Services))
	for k, v := range mailbox.Services {
		services[k] = v
	}
	mailbox.Services = services
	mailbox.RegisteredPlatforms = append([]string(nil), mailbox.RegisteredPlatforms...)
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
