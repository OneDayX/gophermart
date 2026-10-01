// Package config reads the gophermart settings from command-line flags and
// environment variables. A variable that is set wins over its flag.
package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// defaultRunAddress is where the server listens unless told otherwise.
const defaultRunAddress = "localhost:8080"

var (
	// ErrNoDatabaseURI means neither DATABASE_URI nor -d is set.
	ErrNoDatabaseURI = errors.New("database URI is required: set DATABASE_URI or -d")

	// ErrNoAccrualAddress means neither ACCRUAL_SYSTEM_ADDRESS nor -r is set.
	ErrNoAccrualAddress = errors.New("accrual system address is required: set ACCRUAL_SYSTEM_ADDRESS or -r")
)

// Config holds the gophermart settings.
type Config struct {
	// RunAddress is the host:port the HTTP server listens on.
	RunAddress string `env:"RUN_ADDRESS"`
	// DatabaseURI is the PostgreSQL connection string.
	DatabaseURI string `env:"DATABASE_URI"`
	// AccrualSystemAddress is the base URL of the accrual system, without a
	// trailing slash. A bare host:port gets the http scheme.
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	// AuthKey signs the auth tokens. When it is empty, a random key is made
	// on every start, so tokens do not survive a restart.
	AuthKey string `env:"AUTH_KEY"`
}

// Load parses the command-line arguments, without the program name, and then
// the environment. It fails on an unknown flag and on a missing required
// setting; -h makes it return flag.ErrHelp after printing the usage.
func Load(args []string) (Config, error) {
	cfg := Config{RunAddress: defaultRunAddress}

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "address to run the server on (host:port)")
	fs.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "PostgreSQL connection string")
	fs.StringVar(&cfg.AccrualSystemAddress, "r", cfg.AccrualSystemAddress, "accrual system address")
	fs.StringVar(&cfg.AuthKey, "k", cfg.AuthKey, "key to sign auth tokens with (random if empty)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment variables: %w", err)
	}

	if cfg.DatabaseURI == "" {
		return Config{}, ErrNoDatabaseURI
	}
	if cfg.AccrualSystemAddress == "" {
		return Config{}, ErrNoAccrualAddress
	}

	cfg.AccrualSystemAddress = normalizeURL(cfg.AccrualSystemAddress)

	return cfg, nil
}

// normalizeURL makes a base URL out of an address: the autotests pass the
// accrual system with a scheme, a person is likely to type just host:port.
func normalizeURL(addr string) string {
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}

	return strings.TrimRight(addr, "/")
}
