package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Flags(t *testing.T) {
	cfg, err := Load([]string{"-a", ":9090", "-d", "postgres://db", "-r", "localhost:8081/"})
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.RunAddress)
	assert.Equal(t, "postgres://db", cfg.DatabaseURI)
	assert.Equal(t, "http://localhost:8081", cfg.AccrualSystemAddress)
}

func TestLoad_EnvOverridesFlags(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "localhost:7000")
	t.Setenv("DATABASE_URI", "postgres://env")

	cfg, err := Load([]string{"-a", ":9090", "-d", "postgres://flag", "-r", "http://accrual"})
	require.NoError(t, err)

	assert.Equal(t, "localhost:7000", cfg.RunAddress)
	assert.Equal(t, "postgres://env", cfg.DatabaseURI)
}

func TestLoad_Errors(t *testing.T) {
	t.Setenv("DATABASE_URI", "")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "")

	_, err := Load([]string{"-r", "http://accrual"})
	assert.ErrorIs(t, err, ErrNoDatabaseURI)

	_, err = Load([]string{"-d", "postgres://db"})
	assert.ErrorIs(t, err, ErrNoAccrualAddress)

	_, err = Load([]string{"-unknown"})
	assert.Error(t, err)
}
