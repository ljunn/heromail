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
	if ids, err := w.repository.PendingMailboxVerificationIDs(1000); err == nil {
		for _, id := range ids {
			_ = w.queue.EnqueueMailboxVerification(ctx, id)
		}
	}
	for ctx.Err() == nil {
		mailboxID, err := w.queue.DequeueMailboxVerification(ctx, 5*time.Second)
		if err != nil || mailboxID == "" {
			continue
		}
		_, _ = w.verifier.Verify(ctx, "system", mailboxID, "")
	}
}
