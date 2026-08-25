package store

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
)

var testOrderProviders = []string{domain.MailboxProviderOutlook, domain.MailboxProviderHotmail}

func TestRegistrationConsumesMailboxForOneServiceOnly(t *testing.T) {
	s := New()
	first, err := s.CreateOrder("user-001", "svc-adobe", "test-1", testOrderProviders)
	if err != nil {
		t.Fatalf("create first order: %v", err)
	}
	if _, err := s.ReceiveCodeValue(first.ID, "628419"); err != nil {
		t.Fatalf("receive first code: %v", err)
	}

	mailbox, ok := findMailbox(s.Mailboxes(), first.MailboxAddress)
	if !ok {
		t.Fatalf("mailbox %s not found", first.MailboxAddress)
	}
	if got := mailbox.Services["svc-adobe"].State; got != domain.ServiceConsumed {
		t.Fatalf("adobe state = %s, want %s", got, domain.ServiceConsumed)
	}
	if got := mailbox.Services["svc-openai"].State; got != domain.ServiceAvailable {
		t.Fatalf("openai state = %s, want %s", got, domain.ServiceAvailable)
	}
	if len(mailbox.RegisteredPlatforms) != 1 || mailbox.RegisteredPlatforms[0] != "adobe" {
		t.Fatalf("registered platforms = %#v, want [adobe]", mailbox.RegisteredPlatforms)
	}

	second, err := s.CreateOrder("user-001", "svc-adobe", "test-2", testOrderProviders)
	if err != nil {
		t.Fatalf("create second order: %v", err)
	}
	if second.MailboxAddress == first.MailboxAddress {
		t.Fatalf("reused %s for the same target platform", first.MailboxAddress)
	}
}

func TestReceiveCodeRejectsEmptyValueAndAcceptsAssignedOrder(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "real-code-only", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if _, err := s.ReceiveCodeValue(order.ID, ""); !errors.Is(err, ErrVerificationCodeRequired) {
		t.Fatalf("空验证码返回 %v，期望 %v", err, ErrVerificationCodeRequired)
	}
	received, err := s.ReceiveCodeValue(order.ID, "314159")
	if err != nil {
		t.Fatalf("assigned 订单写入真实验证码失败：%v", err)
	}
	if received.Status != domain.OrderCodeReceived || received.Code != "314159" {
		t.Fatalf("真实验证码写入结果错误：%+v", received)
	}
}

func TestReceiveCodeRejectsExpiredOrder(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "expired-code", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	s.orders[order.ID].ExpiresAt = time.Now().Add(-time.Second)
	if _, err := s.ReceiveCodeValue(order.ID, "314159"); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("过期订单写入验证码返回 %v，期望 %v", err, ErrInvalidOrderState)
	}
}

func TestCancelRefundsAndReleasesLease(t *testing.T) {
	s := New()
	before, _ := s.User("user-001")
	order, err := s.CreateOrder("user-001", "svc-adobe", "cancel-1", testOrderProviders)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	afterCharge, _ := s.User("user-001")
	if math.Abs((before.Balance-afterCharge.Balance)-order.Price) > 0.000001 {
		t.Fatalf("balance charge = %.2f, want %.2f", before.Balance-afterCharge.Balance, order.Price)
	}
	if _, err := s.CancelOrder(order.ID, "user-001"); err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	afterRefund, _ := s.User("user-001")
	if math.Abs(afterRefund.Balance-before.Balance) > 0.000001 {
		t.Fatalf("balance after refund = %.2f, want %.2f", afterRefund.Balance, before.Balance)
	}
	mailbox, _ := findMailbox(s.Mailboxes(), order.MailboxAddress)
	if mailbox.State != domain.MailboxAvailable || mailbox.Services[order.ServiceID].State != domain.ServiceAvailable {
		t.Fatalf("lease was not released: mailbox=%s service=%s", mailbox.State, mailbox.Services[order.ServiceID].State)
	}
}

func TestCreateOrderStartsListeningForAtLeastThirtyMinutes(t *testing.T) {
	s := New()
	before, _ := s.User("user-001")
	order, err := s.CreateOrder("user-001", "svc-openai", "automatic-listening", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if order.Status != domain.OrderWaitingCode || order.SubmittedAt.IsZero() {
		t.Fatalf("订单没有自动进入收码状态：%+v", order)
	}
	if order.ExpiresAt.Sub(order.CreatedAt) < 30*time.Minute {
		t.Fatalf("订单有效期 = %s，期望至少 30 分钟", order.ExpiresAt.Sub(order.CreatedAt))
	}
	after, _ := s.User("user-001")
	if math.Abs((before.Balance-after.Balance)-order.Price) > 0.000001 {
		t.Fatalf("下单扣费 = %.2f，期望 %.2f", before.Balance-after.Balance, order.Price)
	}
}

func TestFiveNoCodeTimeoutsConsumeMailboxService(t *testing.T) {
	s := New()
	const mailboxAddress = "hero_01@outlook.com"
	for _, mailbox := range s.mailboxes {
		if mailbox.Address != mailboxAddress {
			mailbox.State = domain.MailboxBlocked
		}
	}
	initialBalance, _ := s.User("user-001")
	for attempt := 1; attempt <= 5; attempt++ {
		order, err := s.CreateOrder("user-001", "svc-adobe", fmt.Sprintf("timeout-%d", attempt), testOrderProviders)
		if err != nil {
			t.Fatalf("第 %d 次创建订单失败：%v", attempt, err)
		}
		if order.MailboxAddress != mailboxAddress {
			t.Fatalf("第 %d 次分配邮箱 = %s，期望 %s", attempt, order.MailboxAddress, mailboxAddress)
		}
		s.orders[order.ID].ExpiresAt = time.Now().Add(-time.Second)
		if reaped := s.ReapExpired(); reaped != 1 {
			t.Fatalf("第 %d 次回收数 = %d，期望 1", attempt, reaped)
		}
		mailbox, _ := findMailbox(s.Mailboxes(), mailboxAddress)
		state := mailbox.Services["svc-adobe"]
		if state.TimeoutCount != attempt {
			t.Fatalf("第 %d 次超时计数 = %d", attempt, state.TimeoutCount)
		}
		wantState := domain.ServiceAvailable
		if attempt == 5 {
			wantState = domain.ServiceConsumed
		}
		if state.State != wantState {
			t.Fatalf("第 %d 次超时状态 = %s，期望 %s", attempt, state.State, wantState)
		}
		balance, _ := s.User("user-001")
		if math.Abs(balance.Balance-initialBalance.Balance) > 0.000001 {
			t.Fatalf("第 %d 次超时退款后余额 = %.2f，期望 %.2f", attempt, balance.Balance, initialBalance.Balance)
		}
	}
	if _, err := s.CreateOrder("user-001", "svc-adobe", "timeout-six", testOrderProviders); !errors.Is(err, ErrNoMailboxAvailable) {
		t.Fatalf("第 5 次超时后仍可分配，返回 %v", err)
	}
}

func TestManualCancelDoesNotCountAsNoCodeTimeout(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "manual-cancel", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if _, err := s.CancelOrder(order.ID, ""); err != nil {
		t.Fatalf("后台取消订单失败：%v", err)
	}
	mailbox, _ := findMailbox(s.Mailboxes(), order.MailboxAddress)
	if got := mailbox.Services[order.ServiceID].TimeoutCount; got != 0 {
		t.Fatalf("主动取消累计了未收码次数：%d", got)
	}
}

func TestReceivedCodeCannotBeCanceledOrRefunded(t *testing.T) {
	s := New()
	before, _ := s.User("user-001")
	order, err := s.CreateOrder("user-001", "svc-openai", "received-no-cancel", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if _, err := s.ReceiveCodeValue(order.ID, "314159"); err != nil {
		t.Fatalf("写入验证码失败：%v", err)
	}
	if _, err := s.CancelOrder(order.ID, ""); !errors.Is(err, ErrInvalidOrderState) {
		t.Fatalf("已收码订单取消返回 %v，期望 %v", err, ErrInvalidOrderState)
	}
	after, _ := s.User("user-001")
	if math.Abs((before.Balance-after.Balance)-order.Price) > 0.000001 {
		t.Fatalf("已收码订单被错误退款：下单前 %.2f，下单后 %.2f", before.Balance, after.Balance)
	}
}

func TestServiceAvailabilityTracksAllocatableMailboxes(t *testing.T) {
	s := New()
	before := s.ServiceAvailability([]string{"svc-adobe"})["svc-adobe"]
	if before <= 0 {
		t.Fatalf("初始 Adobe 余量 = %d，期望大于 0", before)
	}

	order, err := s.CreateOrder("user-001", "svc-adobe", "availability-1", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	afterAllocation := s.ServiceAvailability([]string{"svc-adobe"})["svc-adobe"]
	if afterAllocation != before-1 {
		t.Fatalf("分配后余量 = %d，期望 %d", afterAllocation, before-1)
	}

	if _, err := s.CancelOrder(order.ID, "user-001"); err != nil {
		t.Fatalf("取消订单失败：%v", err)
	}
	afterCancel := s.ServiceAvailability([]string{"svc-adobe"})["svc-adobe"]
	if afterCancel != before {
		t.Fatalf("取消后余量 = %d，期望恢复为 %d", afterCancel, before)
	}
}

func TestServiceAvailabilityGroupsMailboxProviders(t *testing.T) {
	s := New()
	availability := s.ServiceAvailabilityByProvider([]string{"svc-adobe"})["svc-adobe"]
	if availability[domain.MailboxProviderOutlook] <= 0 || availability[domain.MailboxProviderHotmail] <= 0 {
		t.Fatalf("邮箱类型库存没有正确分组：%+v", availability)
	}
	if got, want := availability[domain.MailboxProviderOutlook]+availability[domain.MailboxProviderHotmail], s.ServiceAvailability([]string{"svc-adobe"})["svc-adobe"]; got != want {
		t.Fatalf("分组库存合计 = %d，平台总库存 = %d", got, want)
	}
}

func TestCreateOrderChargesAllocatedMailboxProviderPrice(t *testing.T) {
	s := New()
	s.services["svc-adobe"].ProviderPrices[domain.MailboxProviderOutlook] = 0.37
	s.services["svc-adobe"].ProviderPrices[domain.MailboxProviderHotmail] = 0.81
	before, _ := s.User("user-001")

	order, err := s.CreateOrder("user-001", "svc-adobe", "provider-price", []string{domain.MailboxProviderHotmail})
	if err != nil {
		t.Fatalf("按邮箱类型创建订单失败：%v", err)
	}
	after, _ := s.User("user-001")
	if order.MailboxProvider != domain.MailboxProviderHotmail || len(order.RequestedProviders) != 1 || order.RequestedProviders[0] != domain.MailboxProviderHotmail {
		t.Fatalf("订单邮箱类型记录不正确：%+v", order)
	}
	if math.Abs(order.Price-0.81) > 0.000001 || math.Abs((before.Balance-after.Balance)-0.81) > 0.000001 {
		t.Fatalf("订单费用 %.2f，余额变化 %.2f，期望均为 0.81", order.Price, before.Balance-after.Balance)
	}
}

func TestCreateOrderAllowsVerifiedDirectIMAPMailbox(t *testing.T) {
	s := New()
	for _, mailbox := range s.mailboxes {
		if mailbox.ID != "mb-001" {
			mailbox.State = domain.MailboxBlocked
			continue
		}
		mailbox.Address = "hero@gmail.com"
		mailbox.Provider = domain.MailboxProviderGmail
		mailbox.ConnectionMethod = domain.MailboxConnectionIMAP
		mailbox.OAuthValidUntil = time.Time{}
	}

	order, err := s.CreateOrder("user-001", "svc-adobe", "direct-imap", []string{domain.MailboxProviderGmail})
	if err != nil {
		t.Fatalf("有效 IMAP 邮箱不能参与分配：%v", err)
	}
	if order.MailboxProvider != domain.MailboxProviderGmail {
		t.Fatalf("实际邮箱类型 = %s，期望 %s", order.MailboxProvider, domain.MailboxProviderGmail)
	}
}

func TestCreateOrderRequiresBalanceForHighestSelectedProviderPrice(t *testing.T) {
	s := New()
	s.services["svc-adobe"].ProviderPrices[domain.MailboxProviderOutlook] = 0.10
	s.services["svc-adobe"].ProviderPrices[domain.MailboxProviderHotmail] = 100

	_, err := s.CreateOrder("user-001", "svc-adobe", "provider-price-balance", []string{domain.MailboxProviderOutlook, domain.MailboxProviderHotmail})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("余额不能覆盖所选最高价时返回 %v，期望 %v", err, ErrInsufficientBalance)
	}
}

func TestUserOrderFilterIsServerSideSemantics(t *testing.T) {
	s := New()
	adobe, err := s.CreateOrder("user-001", "svc-adobe", "filter-adobe", testOrderProviders)
	if err != nil {
		t.Fatalf("创建 Adobe 订单失败：%v", err)
	}
	if _, err := s.CancelOrder(adobe.ID, "user-001"); err != nil {
		t.Fatalf("取消 Adobe 订单失败：%v", err)
	}
	if _, err := s.CreateOrder("user-001", "svc-openai", "filter-openai", testOrderProviders); err != nil {
		t.Fatalf("创建 OpenAI 订单失败：%v", err)
	}
	items, total := s.ListUserOrdersPage("user-001", UserOrderFilter{Status: string(domain.OrderCanceled), Service: "adobe", Query: "ORD"}, 1, 20)
	if total != 1 || len(items) != 1 || items[0].ID != adobe.ID {
		t.Fatalf("用户订单筛选结果不正确：total=%d items=%+v", total, items)
	}
}

func TestDefaultServiceSeedDoesNotRestoreDeletedConfiguration(t *testing.T) {
	if !shouldSeedDefaultServices(0, false) {
		t.Fatal("空库首次初始化应创建默认平台")
	}
	if shouldSeedDefaultServices(0, true) {
		t.Fatal("已有初始化标记时不得重新创建平台")
	}
	if shouldSeedDefaultServices(3, false) {
		t.Fatal("已有平台的数据库不得补种缺失平台")
	}
}

func TestLegacyServicePriceBackfillsEveryAllowedProvider(t *testing.T) {
	row := sqlService{
		AllowedProviders: []string{domain.MailboxProviderOutlook, domain.MailboxProviderHotmail},
		PriceCents:       73,
	}
	service := mapService(row)
	if service.ProviderPrices[domain.MailboxProviderOutlook] != 0.73 || service.ProviderPrices[domain.MailboxProviderHotmail] != 0.73 {
		t.Fatalf("旧平台价格没有按允许类型兼容：%+v", service.ProviderPrices)
	}
}

func TestLegacyOrderMapsMailboxProviderMigrationFields(t *testing.T) {
	row := sqlOrder{MailboxProvider: domain.MailboxProviderHotmail, RequestedProviders: []string{domain.MailboxProviderHotmail}, PriceCents: 81}
	order := mapOrder(row)
	if order.MailboxProvider != domain.MailboxProviderHotmail || len(order.RequestedProviders) != 1 || order.RequestedProviders[0] != domain.MailboxProviderHotmail || order.Price != 0.81 {
		t.Fatalf("订单邮箱类型迁移字段映射不正确：%+v", order)
	}
}

func TestDefaultServicesMatchSupportedRegistrationPlatforms(t *testing.T) {
	services := defaultServices()
	got := make(map[string]domain.Service, len(services))
	for _, service := range services {
		got[service.Code] = service
	}
	for _, code := range []string{"adobe", "imagine", "krea", "leonardo", "openai", "runway", "grok"} {
		if _, ok := got[code]; !ok {
			t.Fatalf("默认平台缺少 %s", code)
		}
	}
	for _, removed := range []string{"github", "discord", "telegram"} {
		if _, ok := got[removed]; ok {
			t.Fatalf("已移除平台 %s 仍存在于默认配置", removed)
		}
	}
	if grok := got["grok"]; len(grok.SenderDomains) != 1 || grok.SenderDomains[0] != "x.ai" || len(grok.SubjectKeywords) != 1 || grok.SubjectKeywords[0] != "validate your email" {
		t.Fatalf("Grok 邮件规则不正确：%+v", grok)
	}
	if openai := got["openai"]; len(openai.SubjectKeywords) != 3 {
		t.Fatalf("OpenAI 邮件标题关键词不正确：%+v", openai.SubjectKeywords)
	}
}

func TestMarkMailboxServiceConsumedUpdatesRegisteredPlatforms(t *testing.T) {
	s := New()
	mailbox, ok := findMailbox(s.Mailboxes(), "hero_01@outlook.com")
	if !ok {
		t.Fatal("未找到测试邮箱")
	}
	changedAt := time.Now().UTC().Add(-time.Minute)
	if err := s.MarkMailboxServiceConsumed(mailbox.ID, "svc-adobe", changedAt); err != nil {
		t.Fatalf("标记平台注册状态失败：%v", err)
	}
	updated, ok := findMailbox(s.Mailboxes(), mailbox.Address)
	if !ok || len(updated.RegisteredPlatforms) != 1 || updated.RegisteredPlatforms[0] != "adobe" {
		t.Fatalf("已注册平台 = %#v，期望 [adobe]", updated.RegisteredPlatforms)
	}
	if err := s.MarkMailboxServiceConsumed(mailbox.ID, "svc-adobe", changedAt); err != nil {
		t.Fatalf("重复标记平台注册状态失败：%v", err)
	}
}

func TestAdminCanMarkMailboxRegisteredForService(t *testing.T) {
	s := New()
	mailbox, ok := findMailbox(s.Mailboxes(), "hero_01@outlook.com")
	if !ok {
		t.Fatal("未找到测试邮箱")
	}
	if err := s.MarkMailboxServiceRegistered("admin-001", mailbox.ID, "svc-openai", "127.0.0.1"); err != nil {
		t.Fatalf("管理员手工标记失败：%v", err)
	}
	updated, _ := findMailbox(s.Mailboxes(), mailbox.Address)
	if len(updated.RegisteredPlatforms) != 1 || updated.RegisteredPlatforms[0] != "openai" {
		t.Fatalf("已注册平台 = %#v，期望 [openai]", updated.RegisteredPlatforms)
	}
	if err := s.MarkMailboxServiceRegistered("admin-001", mailbox.ID, "svc-openai", "127.0.0.1"); err != nil {
		t.Fatalf("重复手工标记应当幂等：%v", err)
	}
}

func TestAdminCannotMarkLeasedMailboxServiceRegistered(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "manual-register-leased", testOrderProviders)
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	if err := s.MarkMailboxServiceRegistered("admin-001", order.MailboxID, order.ServiceID, "127.0.0.1"); !errors.Is(err, ErrMailboxServiceLeased) {
		t.Fatalf("租用中的邮箱平台标记返回 %v，期望 %v", err, ErrMailboxServiceLeased)
	}
}

func findMailbox(mailboxes []domain.Mailbox, address string) (domain.Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if mailbox.Address == address {
			return mailbox, true
		}
	}
	return domain.Mailbox{}, false
}

func TestListMailboxesPageFiltersByAddress(t *testing.T) {
	items, total := New().ListMailboxesPage(MailboxFilter{Query: "HERO_01@OUTLOOK"}, 1, 20)
	if total != 1 || len(items) != 1 || items[0].Address != "hero_01@outlook.com" {
		t.Fatalf("邮箱搜索结果错误：total=%d items=%+v", total, items)
	}
	items, total = New().ListMailboxesPage(MailboxFilter{Query: "不存在"}, 1, 20)
	if total != 0 || len(items) != 0 {
		t.Fatalf("无结果搜索错误：total=%d items=%+v", total, items)
	}
}
