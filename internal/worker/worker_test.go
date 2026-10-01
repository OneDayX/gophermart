package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/OneDayX/gophermart/internal/accrual"
	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
)

// fakeStore hands out the pending orders and remembers what the worker saves.
type fakeStore struct {
	pending []string

	mu       sync.Mutex
	statuses map[string]models.OrderStatus
	accruals map[string]float64
}

func newFakeStore(pending ...string) *fakeStore {
	return &fakeStore{
		pending:  pending,
		statuses: make(map[string]models.OrderStatus),
		accruals: make(map[string]float64),
	}
}

func (s *fakeStore) Pending(ctx context.Context, limit int) ([]string, error) {
	return s.pending, nil
}

func (s *fakeStore) ApplyAccrual(ctx context.Context, number string, status models.OrderStatus, accrual *float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.statuses[number] = status
	if accrual != nil {
		s.accruals[number] = *accrual
	}
	return nil
}

// fakeClient answers from a map; unknown orders are not registered.
type fakeClient struct {
	answers map[string]accrual.OrderAccrual
	err     error
	calls   atomic.Int32
}

func (c *fakeClient) Order(ctx context.Context, number string) (accrual.OrderAccrual, error) {
	c.calls.Add(1)

	if c.err != nil {
		return accrual.OrderAccrual{}, c.err
	}
	answer, ok := c.answers[number]
	if !ok {
		return accrual.OrderAccrual{}, accrual.ErrOrderNotRegistered
	}
	return answer, nil
}

func TestWorker_Poll(t *testing.T) {
	store := newFakeStore("1", "2", "3", "4")
	client := &fakeClient{answers: map[string]accrual.OrderAccrual{
		"1": {Order: "1", Status: accrual.StatusRegistered},
		"2": {Order: "2", Status: accrual.StatusInvalid},
		"3": {Order: "3", Status: accrual.StatusProcessed, Accrual: new(500.0)},
	}}

	New(store, client, nil).Poll(context.Background())

	assert.Equal(t, map[string]models.OrderStatus{
		"1": models.OrderStatusProcessing,
		"2": models.OrderStatusInvalid,
		"3": models.OrderStatusProcessed,
		"4": models.OrderStatusNew,
	}, store.statuses)
	assert.Equal(t, map[string]float64{"3": 500}, store.accruals)
}

func TestWorker_PausesOnRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := newFakeStore("1", "2", "3")
		client := &fakeClient{err: &accrual.RateLimitError{RetryAfter: time.Minute}}

		w := New(store, client, nil)
		w.concurrency = 1
		w.Poll(t.Context())

		assert.Equal(t, int32(1), client.calls.Load(), "after 429 the rest of the batch is skipped")
		assert.True(t, w.paused())
		assert.Empty(t, store.statuses)

		time.Sleep(time.Minute)
		assert.False(t, w.paused())
	})
}

func TestWorker_SkipsErrors(t *testing.T) {
	store := newFakeStore("1", "2")
	client := &fakeClient{answers: map[string]accrual.OrderAccrual{
		"1": {Order: "1", Status: "SOMETHING"},
	}}

	New(store, client, nil).Poll(context.Background())
	assert.Equal(t, map[string]models.OrderStatus{"2": models.OrderStatusNew}, store.statuses,
		"an order with an unknown status stays as it was")

	store = newFakeStore("1")
	New(store, &fakeClient{err: errors.New("connection refused")}, nil).Poll(context.Background())
	assert.Empty(t, store.statuses)
}

func TestWorker_Run(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &fakeClient{answers: map[string]accrual.OrderAccrual{
			"1": {Order: "1", Status: accrual.StatusProcessing},
		}}

		ctx, cancel := context.WithTimeout(t.Context(), 3500*time.Millisecond)
		defer cancel()

		New(newFakeStore("1"), client, nil).Run(ctx)

		// Once at the start and then on every tick of the one-second interval.
		assert.Equal(t, int32(4), client.calls.Load())
	})
}
