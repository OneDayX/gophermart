package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestBalance(t *testing.T) {
	svc := stubBalances{balance: models.Balance{Current: 500.5, Withdrawn: 42}}

	w := httptest.NewRecorder()
	NewHandler(nil).Balance(svc)(w, newRequest(http.MethodGet, "/api/user/balance", "", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"current":500.5,"withdrawn":42}`, w.Body.String())
}

func TestWithdraw(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		svcErr   error
		wantCode int
	}{
		{name: "success", body: `{"order":"2377225624","sum":751}`, wantCode: http.StatusOK},
		{name: "insufficient funds", body: `{"order":"2377225624","sum":751}`, svcErr: models.ErrInsufficientFunds, wantCode: http.StatusPaymentRequired},
		{name: "invalid order", body: `{"order":"2377225625","sum":751}`, svcErr: models.ErrInvalidOrderNumber, wantCode: http.StatusUnprocessableEntity},
		{name: "broken json", body: `{"order":`, wantCode: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(nil).Withdraw(stubBalances{err: tt.svcErr})(w, newRequest(http.MethodPost, "/api/user/balance/withdraw", "application/json", tt.body))

			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestWithdrawals(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	svc := stubBalances{withdrawals: []models.Withdrawal{
		{Order: "2377225624", Sum: 500, ProcessedAt: time.Date(2020, 12, 9, 16, 9, 57, 0, msk)},
	}}

	w := httptest.NewRecorder()
	NewHandler(nil).Withdrawals(svc)(w, newRequest(http.MethodGet, "/api/user/withdrawals", "", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[{"order":"2377225624","sum":500,"processed_at":"2020-12-09T16:09:57+03:00"}]`, w.Body.String())

	w = httptest.NewRecorder()
	NewHandler(nil).Withdrawals(stubBalances{})(w, newRequest(http.MethodGet, "/api/user/withdrawals", "", ""))

	assert.Equal(t, http.StatusNoContent, w.Code)
}
