package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/OneDayX/gophermart/internal/auth"
	"github.com/OneDayX/gophermart/internal/models"
)

type stubUsers struct {
	err error
}

func (s stubUsers) Register(ctx context.Context, login, password string) (string, error) {
	return "the-token", s.err
}

func (s stubUsers) Login(ctx context.Context, login, password string) (string, error) {
	return "the-token", s.err
}

type stubOrders struct {
	orders []models.Order
	err    error
}

func (s stubOrders) Upload(ctx context.Context, userID int64, number string) error {
	return s.err
}

func (s stubOrders) List(ctx context.Context, userID int64) ([]models.Order, error) {
	return s.orders, s.err
}

type stubBalances struct {
	balance     models.Balance
	withdrawals []models.Withdrawal
	err         error
}

func (s stubBalances) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return s.balance, s.err
}

func (s stubBalances) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return s.err
}

func (s stubBalances) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return s.withdrawals, s.err
}

// newRequest builds a request as if the auth middleware had let user 1 through.
func newRequest(method, target, contentType, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r.WithContext(auth.WithUserID(r.Context(), 1))
}
