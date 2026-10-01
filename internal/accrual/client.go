// Package accrual is a client of the accrual system: the external service that
// decides how many loyalty points an order earns.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// defaultTimeout bounds one request when the caller gives no client.
	defaultTimeout = 10 * time.Second

	// defaultRetryAfter is how long to wait after a 429 that does not say.
	defaultRetryAfter = 60 * time.Second

	// maxRetryAfter caps the wait after a 429. The accrual system limits
	// requests per minute, so a much longer wait is a mistake, and the worker
	// must not stall until a restart because of it.
	maxRetryAfter = time.Hour
)

// Status is the stage of the calculation as the accrual system reports it.
type Status string

const (
	// StatusRegistered means the order is registered, but the reward is not
	// calculated yet.
	StatusRegistered Status = "REGISTERED"
	// StatusInvalid means the order is not accepted for calculation and will
	// earn nothing. The status is final.
	StatusInvalid Status = "INVALID"
	// StatusProcessing means the reward is being calculated.
	StatusProcessing Status = "PROCESSING"
	// StatusProcessed means the reward is calculated. The status is final.
	StatusProcessed Status = "PROCESSED"
)

var (
	// ErrOrderNotRegistered means the accrual system does not know the order.
	// It may still register it later.
	ErrOrderNotRegistered = errors.New("order is not registered in the accrual system")

	// ErrUnexpectedResponse means the accrual system answered with a status
	// code its protocol does not define.
	ErrUnexpectedResponse = errors.New("unexpected response from the accrual system")
)

// RateLimitError means the accrual system refused the request because the
// client makes too many of them.
type RateLimitError struct {
	// RetryAfter is how long the accrual system asks to wait before the next
	// request.
	RetryAfter time.Duration
}

// Error describes the refusal.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("accrual system rate limit exceeded, retry after %s", e.RetryAfter)
}

// OrderAccrual is what the accrual system knows about an order.
type OrderAccrual struct {
	// Order is the order number.
	Order string `json:"order"`
	// Status is the stage of the calculation.
	Status Status `json:"status"`
	// Accrual is the calculated reward; nil when there is none.
	Accrual *float64 `json:"accrual,omitempty"`
}

// Client asks the accrual system about orders.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient returns a client of the accrual system at baseURL, which has a
// scheme and no trailing slash. A nil client means one with a default timeout.
func NewClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{baseURL: baseURL, client: client}
}

// Order asks the accrual system about the order with GET /api/orders/{number}.
// It returns ErrOrderNotRegistered if the system does not know the order, and
// a *RateLimitError if the system asks to slow down.
func (c *Client) Order(ctx context.Context, number string) (OrderAccrual, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+url.PathEscape(number), nil)
	if err != nil {
		return OrderAccrual{}, fmt.Errorf("build accrual request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return OrderAccrual{}, fmt.Errorf("request accrual: %w", err)
	}
	defer func() {
		// Draining the body lets the transport reuse the connection.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var result OrderAccrual
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return OrderAccrual{}, fmt.Errorf("decode accrual response: %w", err)
		}
		return result, nil

	case http.StatusNoContent:
		return OrderAccrual{}, fmt.Errorf("%w: %s", ErrOrderNotRegistered, number)

	case http.StatusTooManyRequests:
		return OrderAccrual{}, &RateLimitError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}

	default:
		return OrderAccrual{}, fmt.Errorf("%w: status %d", ErrUnexpectedResponse, resp.StatusCode)
	}
}

// parseRetryAfter reads a Retry-After header, which holds either a number of
// seconds or an HTTP date. A missing or unusable value means the default, and
// a wait longer than maxRetryAfter is cut down to it.
func parseRetryAfter(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)

	// A number too large for an int still means "wait long": Atoi reports it
	// with ErrRange and returns the largest int of the same sign.
	if seconds, err := strconv.Atoi(header); err == nil || errors.Is(err, strconv.ErrRange) {
		// The cap is checked before multiplying: enough seconds overflow the
		// nanoseconds a time.Duration counts in.
		switch {
		case seconds <= 0:
			return defaultRetryAfter
		case seconds > int(maxRetryAfter/time.Second):
			return maxRetryAfter
		default:
			return time.Duration(seconds) * time.Second
		}
	}

	if date, err := http.ParseTime(header); err == nil {
		// Sub saturates instead of overflowing, so only the cap is needed.
		if wait := date.Sub(now); wait > 0 {
			return min(wait, maxRetryAfter)
		}
	}

	return defaultRetryAfter
}
