package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/OneDayX/gophermart/internal/models"
)

// orderResponse is one entry of the order list.
type orderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

type orderUploader interface {
	Upload(ctx context.Context, userID int64, number string) error
}

type orderLister interface {
	List(ctx context.Context, userID int64) ([]models.Order, error)
}

// UploadOrder returns an HTTP handler that accepts an order number, sent as
// plain text, for the accrual calculation.
// URL pattern: POST /api/user/orders
func (h *Handler) UploadOrder(svc orderUploader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.userID(w, r)
		if !ok {
			return
		}

		if err := requireContentType(r, "text/plain"); err != nil {
			h.badRequest(w, r, err)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
		if err != nil {
			h.badRequest(w, r, err)
			return
		}

		number := strings.TrimSpace(string(body))
		if number == "" {
			h.badRequest(w, r, errors.New("empty order number"))
			return
		}

		err = svc.Upload(r.Context(), userID, number)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusAccepted)
		case errors.Is(err, models.ErrOrderAlreadyUploaded):
			// Not a failure: the order is already being processed.
			w.WriteHeader(http.StatusOK)
		default:
			h.fail(w, r, "failed to upload order", err)
		}
	}
}

// ListOrders returns an HTTP handler that lists the orders of the user,
// newest first, with their statuses and accruals.
// URL pattern: GET /api/user/orders
func (h *Handler) ListOrders(svc orderLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.userID(w, r)
		if !ok {
			return
		}

		orders, err := svc.List(r.Context(), userID)
		if err != nil {
			h.fail(w, r, "failed to list orders", err)
			return
		}

		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]orderResponse, 0, len(orders))
		for _, order := range orders {
			response = append(response, orderResponse{
				Number:     order.Number,
				Status:     string(order.Status),
				Accrual:    order.Accrual,
				UploadedAt: order.UploadedAt.Format(time.RFC3339),
			})
		}

		h.writeJSON(w, r, http.StatusOK, response)
	}
}
