package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OneDayX/gophermart/internal/auth"
	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubServices answers every call successfully.
type stubServices struct{}

func (stubServices) Register(ctx context.Context, login, password string) (string, error) {
	return "token", nil
}

func (stubServices) Login(ctx context.Context, login, password string) (string, error) {
	return "token", nil
}

func (stubServices) Upload(ctx context.Context, userID int64, number string) error {
	return nil
}

func (stubServices) List(ctx context.Context, userID int64) ([]models.Order, error) {
	return nil, nil
}

func (stubServices) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return models.Balance{}, nil
}

func (stubServices) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return nil
}

func (stubServices) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return nil, nil
}

func TestRouter(t *testing.T) {
	tokens := auth.NewTokens("secret", time.Hour)
	token, err := tokens.Issue(1)
	require.NoError(t, err)

	svc := stubServices{}
	router := newRouter(services{users: svc, orders: svc, balances: svc, tokens: tokens}, zap.NewNop())

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		token    string
		wantCode int
	}{
		{name: "register", method: http.MethodPost, path: "/api/user/register", body: `{"login":"a","password":"b"}`, wantCode: http.StatusOK},
		{name: "balance", method: http.MethodGet, path: "/api/user/balance", token: token, wantCode: http.StatusOK},
		{name: "orders", method: http.MethodGet, path: "/api/user/orders", token: token, wantCode: http.StatusNoContent},
		{name: "withdrawals", method: http.MethodGet, path: "/api/user/withdrawals", token: token, wantCode: http.StatusNoContent},
		{name: "orders without token", method: http.MethodGet, path: "/api/user/orders", wantCode: http.StatusUnauthorized},
		{name: "unknown path", method: http.MethodGet, path: "/api/user/unknown", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.token != "" {
				req.Header.Set("Authorization", auth.BearerHeader(tt.token))
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)
		})
	}
}
