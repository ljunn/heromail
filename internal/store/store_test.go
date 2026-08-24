package store

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/ljunn/heromail/internal/domain"
)

func TestRegistrationConsumesMailboxForOneServiceOnly(t *testing.T) {
	s := New()
	first, err := s.CreateOrder("user-001", "svc-github", "test-1")
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
	if got := mailbox.Services["svc-github"].State; got != domain.ServiceConsumed {
		t.Fatalf("github state = %s, want %s", got, domain.ServiceConsumed)
	}
	if got := mailbox.Services["svc-openai"].State; got != domain.ServiceAvailable {
		t.Fatalf("openai state = %s, want %s", got, domain.ServiceAvailable)
	}
	if len(mailbox.RegisteredPlatforms) != 1 || mailbox.RegisteredPlatforms[0] != "github" {
		t.Fatalf("registered platforms = %#v, want [github]", mailbox.RegisteredPlatforms)
	}

	second, err := s.CreateOrder("user-001", "svc-github", "test-2")
	if err != nil {
		t.Fatalf("create second order: %v", err)
	}
	if second.MailboxAddress == first.MailboxAddress {
		t.Fatalf("reused %s for the same target platform", first.MailboxAddress)
	}
}

func TestReceiveCodeRejectsEmptyValueAndAcceptsAssignedOrder(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "real-code-only")
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
	order, err := s.CreateOrder("user-001", "svc-openai", "expired-code")
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
	order, err := s.CreateOrder("user-001", "svc-github", "cancel-1")
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
	order, err := s.CreateOrder("user-001", "svc-openai", "automatic-listening")
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
		order, err := s.CreateOrder("user-001", "svc-github", fmt.Sprintf("timeout-%d", attempt))
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
		state := mailbox.Services["svc-github"]
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
	if _, err := s.CreateOrder("user-001", "svc-github", "timeout-six"); !errors.Is(err, ErrNoMailboxAvailable) {
		t.Fatalf("第 5 次超时后仍可分配，返回 %v", err)
	}
}

func TestManualCancelDoesNotCountAsNoCodeTimeout(t *testing.T) {
	s := New()
	order, err := s.CreateOrder("user-001", "svc-openai", "manual-cancel")
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
	order, err := s.CreateOrder("user-001", "svc-openai", "received-no-cancel")
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
	before := s.ServiceAvailability([]string{"svc-github"})["svc-github"]
	if before <= 0 {
		t.Fatalf("初始 GitHub 余量 = %d，期望大于 0", before)
	}

	order, err := s.CreateOrder("user-001", "svc-github", "availability-1")
	if err != nil {
		t.Fatalf("创建订单失败：%v", err)
	}
	afterAllocation := s.ServiceAvailability([]string{"svc-github"})["svc-github"]
	if afterAllocation != before-1 {
		t.Fatalf("分配后余量 = %d，期望 %d", afterAllocation, before-1)
	}

	if _, err := s.CancelOrder(order.ID, "user-001"); err != nil {
		t.Fatalf("取消订单失败：%v", err)
	}
	afterCancel := s.ServiceAvailability([]string{"svc-github"})["svc-github"]
	if afterCancel != before {
		t.Fatalf("取消后余量 = %d，期望恢复为 %d", afterCancel, before)
	}
}

func TestUserOrderFilterIsServerSideSemantics(t *testing.T) {
	s := New()
	github, err := s.CreateOrder("user-001", "svc-github", "filter-github")
	if err != nil {
		t.Fatalf("创建 GitHub 订单失败：%v", err)
	}
	if _, err := s.CancelOrder(github.ID, "user-001"); err != nil {
		t.Fatalf("取消 GitHub 订单失败：%v", err)
	}
	if _, err := s.CreateOrder("user-001", "svc-openai", "filter-openai"); err != nil {
		t.Fatalf("创建 OpenAI 订单失败：%v", err)
	}
	items, total := s.ListUserOrdersPage("user-001", UserOrderFilter{Status: string(domain.OrderCanceled), Service: "github", Query: "ORD"}, 1, 20)
	if total != 1 || len(items) != 1 || items[0].ID != github.ID {
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

func TestMarkMailboxServiceConsumedUpdatesRegisteredPlatforms(t *testing.T) {
	s := New()
	mailbox, ok := findMailbox(s.Mailboxes(), "hero_01@outlook.com")
	if !ok {
		t.Fatal("未找到测试邮箱")
	}
	changedAt := time.Now().UTC().Add(-time.Minute)
	if err := s.MarkMailboxServiceConsumed(mailbox.ID, "svc-github", changedAt); err != nil {
		t.Fatalf("标记平台注册状态失败：%v", err)
	}
	updated, ok := findMailbox(s.Mailboxes(), mailbox.Address)
	if !ok || len(updated.RegisteredPlatforms) != 1 || updated.RegisteredPlatforms[0] != "github" {
		t.Fatalf("已注册平台 = %#v，期望 [github]", updated.RegisteredPlatforms)
	}
	if err := s.MarkMailboxServiceConsumed(mailbox.ID, "svc-github", changedAt); err != nil {
		t.Fatalf("重复标记平台注册状态失败：%v", err)
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
