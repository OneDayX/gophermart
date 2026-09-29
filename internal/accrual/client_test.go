package accrual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Order(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    OrderAccrual
		wantErr error
	}{
		{
			name:   "processed",
			status: http.StatusOK,
			body:   `{"order":"12345678903","status":"PROCESSED","accrual":500}`,
			want:   OrderAccrual{Order: "12345678903", Status: StatusProcessed, Accrual: new(500.0)},
		},
		{
			name:    "not registered",
			status:  http.StatusNoContent,
			wantErr: ErrOrderNotRegistered,
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			wantErr: ErrUnexpectedResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/orders/12345678903", r.URL.Path)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := NewClient(srv.URL, nil).Order(context.Background(), "12345678903")
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_OrderRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "No more than 10 requests per minute allowed", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, nil).Order(context.Background(), "12345678903")

	var rateLimit *RateLimitError
	require.ErrorAs(t, err, &rateLimit)
	assert.Equal(t, 30*time.Second, rateLimit.RetryAfter)
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)

	tests := []struct {
		header string
		want   time.Duration
	}{
		{header: "60", want: 60 * time.Second},
		{header: now.Add(90 * time.Second).Format(http.TimeFormat), want: 90 * time.Second},
		{header: "", want: defaultRetryAfter},
		{header: "0", want: defaultRetryAfter},
		{header: "soon", want: defaultRetryAfter},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, parseRetryAfter(tt.header, now), tt.header)
	}
}
