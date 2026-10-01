package app

import (
	"context"
	"net/http"

	"github.com/OneDayX/gophermart/internal/handler"
	"github.com/OneDayX/gophermart/internal/models"
	"github.com/OneDayX/gophermart/internal/server/middleware"
	"go.uber.org/zap"
)

type userService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

type orderService interface {
	Upload(ctx context.Context, userID int64, number string) error
	List(ctx context.Context, userID int64) ([]models.Order, error)
}

type balanceService interface {
	Balance(ctx context.Context, userID int64) (models.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

type tokenParser interface {
	Parse(token string) (int64, error)
}

// services is everything the router hands requests to.
type services struct {
	users    userService
	orders   orderService
	balances balanceService
	tokens   tokenParser
}

// newRouter maps the API onto the handlers. Registration and login are open,
// everything else needs a token.
func newRouter(svc services, log *zap.Logger) http.Handler {
	h := handler.NewHandler(log)
	authed := middleware.Auth(svc.tokens, log)

	mux := http.NewServeMux()
	mux.Handle("POST /api/user/register", h.Register(svc.users))
	mux.Handle("POST /api/user/login", h.Login(svc.users))
	mux.Handle("POST /api/user/orders", authed(h.UploadOrder(svc.orders)))
	mux.Handle("GET /api/user/orders", authed(h.ListOrders(svc.orders)))
	mux.Handle("GET /api/user/balance", authed(h.Balance(svc.balances)))
	mux.Handle("POST /api/user/balance/withdraw", authed(h.Withdraw(svc.balances)))
	mux.Handle("GET /api/user/withdrawals", authed(h.Withdrawals(svc.balances)))

	return middleware.Logger(log)(middleware.Gzip(mux))
}
