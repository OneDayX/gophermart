// Package worker polls the accrual system about the orders whose reward is not
// known yet and records its answers.
package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OneDayX/gophermart/internal/accrual"
	"github.com/OneDayX/gophermart/internal/models"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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
// a batch of them, the ones checked longest ago first, and checks up to
// concurrency of them at once. When the accrual system answers 429, the worker
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
// the orders left from a previous run are picked up without a delay.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		w.Poll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Poll checks one batch of pending orders and returns when all of them are
// done, so the next batch never picks an order that is still being checked.
func (w *Worker) Poll(ctx context.Context) {
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

	// The only error a check returns is a 429. It cancels the group context,
	// and the rest of the batch is skipped: the accrual system would refuse it
	// anyway.
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(w.concurrency)

	for _, number := range numbers {
		if gctx.Err() != nil {
			break
		}
		g.Go(func() error { return w.check(gctx, number) })
	}

	_ = g.Wait()
}

// check asks the accrual system about one order and records the answer. It
// returns an error only when the accrual system asks to wait; any other
// failure is logged, and the order stays pending to be asked about again.
func (w *Worker) check(ctx context.Context, number string) error {
	// The batch may have been stopped while this order waited for its turn.
	if ctx.Err() != nil {
		return nil
	}

	status, reward, err := w.ask(ctx, number)

	var rateLimit *accrual.RateLimitError
	if errors.As(err, &rateLimit) {
		w.pause(rateLimit.RetryAfter)
		w.log.Warn("accrual system asks to slow down",
			zap.String("order", number),
			zap.Duration("retry_after", rateLimit.RetryAfter),
		)
		return err
	}

	if err == nil {
		err = w.orders.ApplyAccrual(ctx, number, status, reward)
	}
	if err != nil && ctx.Err() == nil {
		w.log.Error("failed to check order", zap.String("order", number), zap.Error(err))
	}

	return nil
}

// ask asks the accrual system about the order and translates the answer into
// the status of the order and the reward to credit.
func (w *Worker) ask(ctx context.Context, number string) (models.OrderStatus, *float64, error) {
	result, err := w.client.Order(ctx, number)

	switch {
	case errors.Is(err, accrual.ErrOrderNotRegistered):
		// The accrual system may register the order later, so it stays NEW;
		// it is still marked as checked to let the other orders go first.
		return models.OrderStatusNew, nil, nil
	case err != nil:
		return "", nil, err
	}

	switch result.Status {
	case accrual.StatusRegistered, accrual.StatusProcessing:
		return models.OrderStatusProcessing, nil, nil
	case accrual.StatusInvalid:
		return models.OrderStatusInvalid, nil, nil
	case accrual.StatusProcessed:
		// Only a processed order has a reward to credit.
		return models.OrderStatusProcessed, result.Accrual, nil
	default:
		return "", nil, fmt.Errorf("unknown accrual status %q", result.Status)
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
