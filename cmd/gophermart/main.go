// Command gophermart runs loyalty system: an HTTP API where
// users upload their orders and spend loyalty points, and a worker that asks
// the accrual system how many points each order earns.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/OneDayX/gophermart/internal/app"
	"github.com/OneDayX/gophermart/internal/config"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	// ctx is cancelled on SIGINT/SIGTERM so we can finish the requests in
	// progress before exiting.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	a, err := app.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer a.Close()

	return a.Run(ctx)
}
