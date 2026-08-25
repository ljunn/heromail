package mail

import (
	"context"
	"sync"
	"time"

	"github.com/ljunn/heromail/internal/store"
)

type pendingVerificationRepository interface {
	PendingMailboxVerificationIDs(limit int) ([]string, error)
}

type VerificationWorker struct {
	repository         pendingVerificationRepository
	queue              store.MailboxVerificationQueue
	verifier           *MailboxVerifier
	concurrency        int
	historyConcurrency int
	inFlight           sync.Map
}

func NewVerificationWorker(repository pendingVerificationRepository, queue store.MailboxVerificationQueue, verifier *MailboxVerifier) *VerificationWorker {
	return NewVerificationWorkerWithConcurrency(repository, queue, verifier, 16, 4)
}

func NewVerificationWorkerWithConcurrency(repository pendingVerificationRepository, queue store.MailboxVerificationQueue, verifier *MailboxVerifier, concurrency, historyConcurrency int) *VerificationWorker {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 128 {
		concurrency = 128
	}
	if historyConcurrency < 1 {
		historyConcurrency = 1
	}
	if historyConcurrency > 32 {
		historyConcurrency = 32
	}
	return &VerificationWorker{repository: repository, queue: queue, verifier: verifier, concurrency: concurrency, historyConcurrency: historyConcurrency}
}

func (w *VerificationWorker) Run(ctx context.Context) {
	if w.verifier == nil || w.queue == nil {
		return
	}
	historyJobs := make(chan string, w.historyConcurrency*2)
	var verifyGroup sync.WaitGroup
	var historyGroup sync.WaitGroup
	for index := 0; index < w.historyConcurrency; index++ {
		historyGroup.Add(1)
		go func() {
			defer historyGroup.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case mailboxID, ok := <-historyJobs:
					if !ok {
						return
					}
					historyContext, cancel := context.WithTimeout(ctx, 60*time.Second)
					_, _ = w.verifier.ScanMailboxHistory(historyContext, "system", mailboxID, "")
					cancel()
				}
			}
		}()
	}
	for index := 0; index < w.concurrency; index++ {
		verifyGroup.Add(1)
		go func() {
			defer verifyGroup.Done()
			w.consume(ctx, historyJobs)
		}()
	}
	w.reconcile(ctx)
	reconcileTicker := time.NewTicker(30 * time.Second)
	defer reconcileTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			verifyGroup.Wait()
			close(historyJobs)
			historyGroup.Wait()
			return
		case <-reconcileTicker.C:
			w.reconcile(ctx)
		}
	}
}

func (w *VerificationWorker) consume(ctx context.Context, historyJobs chan<- string) {
	for {
		mailboxID, err := w.queue.DequeueMailboxVerification(ctx, 5*time.Second)
		if ctx.Err() != nil {
			return
		}
		if err != nil || mailboxID == "" {
			continue
		}
		if _, loaded := w.inFlight.LoadOrStore(mailboxID, struct{}{}); loaded {
			continue
		}
		verifyContext, cancel := context.WithTimeout(ctx, 45*time.Second)
		_, verifyErr := w.verifier.Verify(verifyContext, "system", mailboxID, "")
		cancel()
		w.inFlight.Delete(mailboxID)
		if verifyErr != nil {
			continue
		}
		select {
		case historyJobs <- mailboxID:
		case <-ctx.Done():
			return
		}
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
