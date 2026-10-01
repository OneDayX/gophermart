package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestUploadOrder(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		svcErr      error
		wantCode    int
	}{
		{name: "new order", contentType: "text/plain", body: "12345678903", wantCode: http.StatusAccepted},
		{name: "already uploaded", contentType: "text/plain", body: "12345678903", svcErr: models.ErrOrderAlreadyUploaded, wantCode: http.StatusOK},
		{name: "another user's order", contentType: "text/plain", body: "12345678903", svcErr: models.ErrOrderUploadedByAnotherUser, wantCode: http.StatusConflict},
		{name: "invalid number", contentType: "text/plain", body: "12345678901", svcErr: models.ErrInvalidOrderNumber, wantCode: http.StatusUnprocessableEntity},
		{name: "empty body", contentType: "text/plain", body: "", wantCode: http.StatusBadRequest},
		{name: "json body", contentType: "application/json", body: `{"number":"12345678903"}`, wantCode: http.StatusBadRequest},
		{name: "database error", contentType: "text/plain", body: "12345678903", svcErr: errors.New("db is down"), wantCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(nil).UploadOrder(stubOrders{err: tt.svcErr})(w, newRequest(http.MethodPost, "/api/user/orders", tt.contentType, tt.body))

			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}

func TestListOrders(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)
	svc := stubOrders{orders: []models.Order{
		{Number: "9278923470", Status: models.OrderStatusProcessed, Accrual: new(500.0), UploadedAt: time.Date(2020, 12, 10, 15, 15, 45, 0, msk)},
		{Number: "12345678903", Status: models.OrderStatusProcessing, UploadedAt: time.Date(2020, 12, 10, 15, 12, 1, 0, msk)},
	}}

	w := httptest.NewRecorder()
	NewHandler(nil).ListOrders(svc)(w, newRequest(http.MethodGet, "/api/user/orders", "", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[
		{"number":"9278923470","status":"PROCESSED","accrual":500,"uploaded_at":"2020-12-10T15:15:45+03:00"},
		{"number":"12345678903","status":"PROCESSING","uploaded_at":"2020-12-10T15:12:01+03:00"}
	]`, w.Body.String())
}

func TestListOrders_Empty(t *testing.T) {
	w := httptest.NewRecorder()
	NewHandler(nil).ListOrders(stubOrders{})(w, newRequest(http.MethodGet, "/api/user/orders", "", ""))

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOrders_Unauthenticated(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	NewHandler(nil).ListOrders(stubOrders{})(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
