package app

import (
	"context"
	"net/http"

	"github.com/OneDayX/gophermart/internal/handler"
	"github.com/OneDayX/gophermart/internal/models"
	"github.com/OneDayX/gophermart/internal/server/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
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

	r := chi.NewRouter()
	r.Use(middleware.Logger(log))
	// Inside the logger, so a panic is logged as the 500 it turns into.
	r.Use(chimw.Recoverer)
	r.Use(middleware.Gzip)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", h.Register(svc.users)) // POST /api/user/register
		r.Post("/login", h.Login(svc.users))       // POST /api/user/login

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(svc.tokens, log))

			r.Post("/orders", h.UploadOrder(svc.orders))          // POST /api/user/orders
			r.Get("/orders", h.ListOrders(svc.orders))            // GET /api/user/orders
			r.Get("/balance", h.Balance(svc.balances))            // GET /api/user/balance
			r.Post("/balance/withdraw", h.Withdraw(svc.balances)) // POST /api/user/balance/withdraw
			r.Get("/withdrawals", h.Withdrawals(svc.balances))    // GET /api/user/withdrawals
		})
	})

	return r
}
