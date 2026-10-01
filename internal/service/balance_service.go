package service

import (
	"context"
	"fmt"
	"math"

	"github.com/OneDayX/gophermart/internal/luhn"
	"github.com/OneDayX/gophermart/internal/models"
)

type balanceStore interface {
	Balance(ctx context.Context, userID int64) (models.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

// BalanceService runs the loyalty accounts: it reports balances and pays for
// new orders with points.
type BalanceService struct {
	store balanceStore
}

// NewBalanceService returns a BalanceService that keeps accounts in the store.
func NewBalanceService(store balanceStore) *BalanceService {
	return &BalanceService{store: store}
}

// Balance returns the account of the user.
func (s *BalanceService) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return s.store.Balance(ctx, userID)
}

// Withdraw pays for the order with sum points from the account of the user.
// It returns models.ErrInvalidOrderNumber if the number fails the Luhn check,
// models.ErrInvalidSum if sum is not positive, models.ErrInsufficientFunds if
// the balance is short of it, and models.ErrWithdrawalExists if the order has
// been paid with points before.
func (s *BalanceService) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	if !luhn.Valid(order) {
		return fmt.Errorf("%w: %q", models.ErrInvalidOrderNumber, order)
	}

	// JSON has no NaN, but the check costs nothing and keeps the store safe
	// from callers other than the handler.
	if !(sum > 0) || math.IsInf(sum, 0) {
		return fmt.Errorf("%w: %v", models.ErrInvalidSum, sum)
	}

	return s.store.Withdraw(ctx, userID, order, sum)
}

// Withdrawals returns the withdrawals of the user, newest first.
func (s *BalanceService) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return s.store.Withdrawals(ctx, userID)
}
