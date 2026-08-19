package mail

import (
	"context"
	"testing"
	"time"
)

type pendingVerificationRepositoryStub struct {
	ids []string
}

func (s pendingVerificationRepositoryStub) PendingMailboxVerificationIDs(int) ([]string, error) {
	return append([]string(nil), s.ids...), nil
}

type verificationQueueStub struct {
	enqueued []string
}

func (s *verificationQueueStub) EnqueueMailboxVerification(_ context.Context, mailboxID string) error {
	s.enqueued = append(s.enqueued, mailboxID)
	return nil
}

func (*verificationQueueStub) DequeueMailboxVerification(context.Context, time.Duration) (string, error) {
	return "", nil
}

func TestVerificationWorkerReconcilesPendingMailboxes(t *testing.T) {
	queue := &verificationQueueStub{}
	worker := NewVerificationWorker(pendingVerificationRepositoryStub{ids: []string{"mailbox-1", "mailbox-2"}}, queue, nil)
	worker.reconcile(context.Background())
	if len(queue.enqueued) != 2 || queue.enqueued[0] != "mailbox-1" || queue.enqueued[1] != "mailbox-2" {
		t.Fatalf("待验证邮箱补偿入队错误：%v", queue.enqueued)
	}
}
