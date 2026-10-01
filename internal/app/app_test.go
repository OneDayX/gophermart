package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/OneDayX/gophermart/internal/config"
	"github.com/OneDayX/gophermart/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set, skipping database tests")
	}

	db, err := database.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	_, err = db.Pool().Exec(context.Background(), `TRUNCATE users, orders, withdrawals RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	return db
}

// send makes a request with the token and returns the status and the body.
func send(t *testing.T, method, url, token, contentType, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(data)
}

// TestApp_EndToEnd goes through the whole scenario: registration, an order,
// the accrual for it and a withdrawal.
func TestApp_EndToEnd(t *testing.T) {
	db := newTestDB(t)

	accrualSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"12345678903","status":"PROCESSED","accrual":729.98}`))
	}))
	defer accrualSrv.Close()

	a := newApp(config.Config{AccrualSystemAddress: accrualSrv.URL, AuthKey: "test"}, db, zap.NewNop())
	api := httptest.NewServer(a.Handler())
	defer api.Close()

	req, err := http.NewRequest(http.MethodPost, api.URL+"/api/user/register", strings.NewReader(`{"login":"alice","password":"secret"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	token := resp.Header.Get("Authorization")
	require.NotEmpty(t, token)

	code, _ := send(t, http.MethodPost, api.URL+"/api/user/orders", token, "text/plain", "12345678903")
	assert.Equal(t, http.StatusAccepted, code)

	code, _ = send(t, http.MethodPost, api.URL+"/api/user/orders", "", "text/plain", "12345678903")
	assert.Equal(t, http.StatusUnauthorized, code)

	// One poll instead of waiting for the ticker of a running worker.
	a.worker.Poll(context.Background())

	_, body := send(t, http.MethodGet, api.URL+"/api/user/orders", token, "", "")
	assert.Contains(t, body, `"status":"PROCESSED"`)

	code, _ = send(t, http.MethodPost, api.URL+"/api/user/balance/withdraw", token, "application/json", `{"order":"2377225624","sum":1000}`)
	assert.Equal(t, http.StatusPaymentRequired, code)

	code, _ = send(t, http.MethodPost, api.URL+"/api/user/balance/withdraw", token, "application/json", `{"order":"2377225624","sum":700}`)
	assert.Equal(t, http.StatusOK, code)

	_, body = send(t, http.MethodGet, api.URL+"/api/user/balance", token, "", "")
	var balance struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &balance))
	assert.Equal(t, 29.98, balance.Current)
	assert.Equal(t, 700.0, balance.Withdrawn)

	code, _ = send(t, http.MethodGet, api.URL+"/api/user/withdrawals", token, "", "")
	assert.Equal(t, http.StatusOK, code)
}

func TestApp_Run(t *testing.T) {
	db := newTestDB(t)
	a := newApp(config.Config{RunAddress: "127.0.0.1:0", AccrualSystemAddress: "http://127.0.0.1:1"}, db, zap.NewNop())

	// Already cancelled: Run must start everything and shut it down cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.NoError(t, a.Run(ctx))
}

func TestNew_BadDatabase(t *testing.T) {
	_, err := New(context.Background(), config.Config{
		DatabaseURI: "postgres://postgres:postgres@127.0.0.1:1/praktikum?sslmode=disable&connect_timeout=1",
	}, zap.NewNop())
	assert.Error(t, err)
}
