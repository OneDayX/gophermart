package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/OneDayX/gophermart/internal/models"
)

// balanceResponse is the state of the loyalty account.
type balanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// withdrawRequest is the body of the withdrawal request.
type withdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// withdrawalResponse is one entry of the withdrawal list.
type withdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

type balanceGetter interface {
	Balance(ctx context.Context, userID int64) (models.Balance, error)
}

type withdrawer interface {
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
}

type withdrawalLister interface {
	Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

// Balance returns an HTTP handler that reports how many points the user has
// and how many they have spent.
// URL pattern: GET /api/user/balance
func (h *Handler) Balance(svc balanceGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.userID(w, r)
		if !ok {
			return
		}

		balance, err := svc.Balance(r.Context(), userID)
		if err != nil {
			h.fail(w, r, "failed to get balance", err)
			return
		}

		h.writeJSON(w, r, http.StatusOK, balanceResponse{
			Current:   balance.Current,
			Withdrawn: balance.Withdrawn,
		})
	}
}

// Withdraw returns an HTTP handler that pays for a new order with points.
// URL pattern: POST /api/user/balance/withdraw
func (h *Handler) Withdraw(svc withdrawer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.userID(w, r)
		if !ok {
			return
		}

		var req withdrawRequest
		if err := decodeJSON(w, r, &req); err != nil {
			h.badRequest(w, r, err)
			return
		}

		if err := svc.Withdraw(r.Context(), userID, req.Order, req.Sum); err != nil {
			h.fail(w, r, "failed to withdraw", err)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Withdrawals returns an HTTP handler that lists the withdrawals of the user,
// newest first.
// URL pattern: GET /api/user/withdrawals
func (h *Handler) Withdrawals(svc withdrawalLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := h.userID(w, r)
		if !ok {
			return
		}

		withdrawals, err := svc.Withdrawals(r.Context(), userID)
		if err != nil {
			h.fail(w, r, "failed to list withdrawals", err)
			return
		}

		if len(withdrawals) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]withdrawalResponse, 0, len(withdrawals))
		for _, withdrawal := range withdrawals {
			response = append(response, withdrawalResponse{
				Order:       withdrawal.Order,
				Sum:         withdrawal.Sum,
				ProcessedAt: withdrawal.ProcessedAt.Format(time.RFC3339),
			})
		}

		h.writeJSON(w, r, http.StatusOK, response)
	}
}
