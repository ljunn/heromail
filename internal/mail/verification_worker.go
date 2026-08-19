package mail

import (
	"context"
	"time"

	"github.com/ljunn/heromail/internal/store"
)

type pendingVerificationRepository interface {
	PendingMailboxVerificationIDs(limit int) ([]string, error)
}

type VerificationWorker struct {
	repository pendingVerificationRepository
	queue      store.MailboxVerificationQueue
	verifier   *MailboxVerifier
}

func NewVerificationWorker(repository pendingVerificationRepository, queue store.MailboxVerificationQueue, verifier *MailboxVerifier) *VerificationWorker {
	return &VerificationWorker{repository: repository, queue: queue, verifier: verifier}
}

func (w *VerificationWorker) Run(ctx context.Context) {
	nextReconcile := time.Time{}
	for ctx.Err() == nil {
		if time.Now().After(nextReconcile) {
			w.reconcile(ctx)
			nextReconcile = time.Now().Add(30 * time.Second)
		}
		mailboxID, err := w.queue.DequeueMailboxVerification(ctx, 5*time.Second)
		if err != nil || mailboxID == "" {
			continue
		}
		_, _ = w.verifier.Verify(ctx, "system", mailboxID, "")
	}
}

func (w *VerificationWorker) reconcile(ctx context.Context) {
	ids, err := w.repository.PendingMailboxVerificationIDs(1000)
	if err != nil {
		return
	}
	for _, id := range ids {
		_ = w.queue.EnqueueMailboxVerification(ctx, id)
	}
}
