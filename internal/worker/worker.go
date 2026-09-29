// Package worker polls the accrual system about the orders whose reward is not
// known yet and records its answers.
package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/OneDayX/gophermart/internal/accrual"
	"github.com/OneDayX/gophermart/internal/models"
	"go.uber.org/zap"
)

const (
	// defaultInterval is how often the worker looks for pending orders.
	defaultInterval = time.Second

	// defaultConcurrency is how many requests to the accrual system may be in
	// flight at once.
	defaultConcurrency = 4

	// defaultBatchSize is how many pending orders one poll takes.
	defaultBatchSize = 100
)

type orderStore interface {
	Pending(ctx context.Context, limit int) ([]string, error)
	ApplyAccrual(ctx context.Context, number string, status models.OrderStatus, accrual *float64) error
}

type accrualClient interface {
	Order(ctx context.Context, number string) (accrual.OrderAccrual, error)
}

// Worker checks pending orders with the accrual system. On every tick it takes
// a batch of them, the ones checked longest ago first, and hands them to a
// pool of goroutines. When the accrual system answers 429, the whole pool
// stops asking for as long as the system says.
type Worker struct {
	orders orderStore
	client accrualClient
	log    *zap.Logger

	interval    time.Duration
	concurrency int
	batchSize   int

	mu          sync.Mutex
	pausedUntil time.Time
}

// New returns a Worker that takes orders from the store and asks the client
// about them. It uses a nop logger if none is provided.
func New(orders orderStore, client accrualClient, log *zap.Logger) *Worker {
	if log == nil {
		log = zap.NewNop()
	}

	return &Worker{
		orders:      orders,
		client:      client,
		log:         log,
		interval:    defaultInterval,
		concurrency: defaultConcurrency,
		batchSize:   defaultBatchSize,
	}
}

// Run polls the accrual system until ctx is cancelled, starting at once, so
// the orders left from a previous run are picked up without a delay. It
// returns when every goroutine it started has stopped.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		w.poll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll checks one batch of pending orders and waits until all of them are
// done, so the next batch never picks an order that is still being checked.
func (w *Worker) poll(ctx context.Context) {
	if w.paused() {
		return
	}

	numbers, err := w.orders.Pending(ctx, w.batchSize)
	if err != nil {
		if ctx.Err() == nil {
			w.log.Error("failed to fetch pending orders", zap.Error(err))
		}
		return
	}
	if len(numbers) == 0 {
		return
	}

	jobs := make(chan string)

	var wg sync.WaitGroup
	for range min(w.concurrency, len(numbers)) {
		wg.Go(func() {
			for number := range jobs {
				w.check(ctx, number)
			}
		})
	}

	w.dispatch(ctx, numbers, jobs)
	close(jobs)
	wg.Wait()
}

// dispatch hands the numbers to the pool one by one. It stops early when the
// accrual system asks to wait, since it would refuse the rest anyway.
func (w *Worker) dispatch(ctx context.Context, numbers []string, jobs chan<- string) {
	for _, number := range numbers {
		if w.paused() {
			return
		}

		select {
		case jobs <- number:
		case <-ctx.Done():
			return
		}
	}
}

// check asks the accrual system about one order and records the answer. A
// failure is only logged: the order stays pending and is asked about again.
func (w *Worker) check(ctx context.Context, number string) {
	// Another goroutine may have run into the rate limit while this order
	// was waiting for its turn.
	if w.paused() {
		return
	}

	status, reward, ok := w.ask(ctx, number)
	if !ok {
		return
	}

	if err := w.orders.ApplyAccrual(ctx, number, status, reward); err != nil {
		if ctx.Err() == nil {
			w.log.Error("failed to record accrual", zap.String("order", number), zap.Error(err))
		}
		return
	}

	w.log.Debug("order checked", zap.String("order", number), zap.String("status", string(status)))
}

// ask asks the accrual system about the order and translates the answer into
// the status of the order and the reward to credit. The flag is false when
// there is no answer to record.
func (w *Worker) ask(ctx context.Context, number string) (models.OrderStatus, *float64, bool) {
	result, err := w.client.Order(ctx, number)

	var rateLimit *accrual.RateLimitError
	switch {
	case errors.As(err, &rateLimit):
		w.pause(rateLimit.RetryAfter)
		w.log.Warn("accrual system asks to slow down",
			zap.String("order", number),
			zap.Duration("retry_after", rateLimit.RetryAfter),
		)
		return "", nil, false

	case errors.Is(err, accrual.ErrOrderNotRegistered):
		// The accrual system may register the order later, so it stays NEW;
		// it is still marked as checked to let the other orders go first.
		return models.OrderStatusNew, nil, true

	case err != nil:
		if ctx.Err() == nil {
			w.log.Error("failed to ask the accrual system", zap.String("order", number), zap.Error(err))
		}
		return "", nil, false
	}

	switch result.Status {
	case accrual.StatusRegistered, accrual.StatusProcessing:
		return models.OrderStatusProcessing, nil, true
	case accrual.StatusInvalid:
		return models.OrderStatusInvalid, nil, true
	case accrual.StatusProcessed:
		// Only a processed order has a reward to credit.
		return models.OrderStatusProcessed, result.Accrual, true
	default:
		w.log.Error("accrual system reported an unknown status",
			zap.String("order", number),
			zap.String("status", string(result.Status)),
		)
		return "", nil, false
	}
}

// pause stops the requests for d. A shorter pause never cuts a longer one.
func (w *Worker) pause(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if until := time.Now().Add(d); until.After(w.pausedUntil) {
		w.pausedUntil = until
	}
}

// paused reports whether the accrual system has asked to wait and the time is
// not up yet.
func (w *Worker) paused() bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return time.Now().Before(w.pausedUntil)
}
