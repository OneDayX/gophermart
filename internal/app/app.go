// Package app assembles gophermart out of its parts and runs it: the HTTP API
// and the worker that polls the accrual system.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OneDayX/gophermart/internal/accrual"
	"github.com/OneDayX/gophermart/internal/auth"
	"github.com/OneDayX/gophermart/internal/config"
	"github.com/OneDayX/gophermart/internal/database"
	"github.com/OneDayX/gophermart/internal/repository"
	"github.com/OneDayX/gophermart/internal/service"
	"github.com/OneDayX/gophermart/internal/worker"
	"go.uber.org/zap"
)

const (
	// shutdownTimeout is how long the requests in progress get to finish.
	shutdownTimeout = 10 * time.Second

	// readHeaderTimeout keeps a slow client from holding a connection open
	// without ever sending a request.
	readHeaderTimeout = 5 * time.Second

	// tokenTTL is how long a user stays signed in.
	tokenTTL = 24 * time.Hour
)

// App is a configured gophermart instance.
type App struct {
	log    *zap.Logger
	db     *database.DB
	server *http.Server
	worker *worker.Worker
}

// New connects to the database, brings its schema up to date and wires the
// components together. Close releases what it has opened.
func New(ctx context.Context, cfg config.Config, log *zap.Logger) (*App, error) {
	db, err := database.New(ctx, cfg.DatabaseURI)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return newApp(cfg, db, log), nil
}

// newApp wires the components around an open database.
func newApp(cfg config.Config, db *database.DB, log *zap.Logger) *App {
	key := cfg.AuthKey
	if key == "" {
		key = rand.Text()
		log.Warn("no auth key is set, tokens will not survive a restart")
	}
	tokens := auth.NewTokens(key, tokenTTL)

	users := repository.NewUserRepository(db.Pool())
	orders := repository.NewOrderRepository(db.Pool())
	balances := repository.NewBalanceRepository(db.Pool())

	router := newRouter(services{
		users:    service.NewUserService(users, tokens),
		orders:   service.NewOrderService(orders),
		balances: service.NewBalanceService(balances),
		tokens:   tokens,
	}, log)

	return &App{
		log: log,
		db:  db,
		server: &http.Server{
			Addr:              cfg.RunAddress,
			Handler:           router,
			ReadHeaderTimeout: readHeaderTimeout,
		},
		worker: worker.New(orders, accrual.NewClient(cfg.AccrualSystemAddress, nil), log),
	}
}

// Handler returns the HTTP API.
func (a *App) Handler() http.Handler {
	return a.server.Handler
}

// Run serves the API and polls the accrual system until ctx is cancelled or
// the server fails. Then it lets the requests in progress finish, stops the
// worker and returns the server error, if any.
func (a *App) Run(ctx context.Context) error {
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()

	var wg sync.WaitGroup
	wg.Go(func() { a.worker.Run(workerCtx) })

	// Buffered, so the goroutine can report the error and exit even when
	// nobody is waiting for it anymore.
	serveErr := make(chan error, 1)

	a.log.Info("starting server", zap.String("addr", a.server.Addr))
	go func() {
		// ErrServerClosed only means Shutdown was called, which is not a failure.
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
	case err := <-serveErr:
		runErr = fmt.Errorf("server error: %w", err)
		a.log.Error("server error", zap.Error(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		a.log.Error("failed to shut down the server", zap.Error(err))
	}

	// An order interrupted halfway keeps its old status and is checked again
	// on the next start.
	stopWorker()
	wg.Wait()

	return runErr
}

// Close releases the database connections. Call it after Run returns.
func (a *App) Close() {
	a.db.Close()
}
