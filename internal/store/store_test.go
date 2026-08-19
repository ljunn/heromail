package store

import (
	"math"
	"testing"

	"github.com/ljunn/heromail/internal/domain"
)

func TestRegistrationConsumesMailboxForOneServiceOnly(t *testing.T) {
	s := New()
	first, err := s.CreateOrder("user-001", "svc-github", "test-1")
	if err != nil {
		t.Fatalf("create first order: %v", err)
	}
	if _, err := s.SubmitOrder(first.ID, "user-001"); err != nil {
		t.Fatalf("submit first order: %v", err)
	}
	if _, err := s.ReceiveCode(first.ID); err != nil {
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

	second, err := s.CreateOrder("user-001", "svc-github", "test-2")
	if err != nil {
		t.Fatalf("create second order: %v", err)
	}
	if second.MailboxAddress == first.MailboxAddress {
		t.Fatalf("reused %s for the same target platform", first.MailboxAddress)
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

func findMailbox(mailboxes []domain.Mailbox, address string) (domain.Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if mailbox.Address == address {
			return mailbox, true
		}
	}
	return domain.Mailbox{}, false
}
