package service

import (
	"context"
	"fmt"

	"github.com/OneDayX/gophermart/internal/luhn"
	"github.com/OneDayX/gophermart/internal/models"
)

type orderStore interface {
	Create(ctx context.Context, userID int64, number string) error
	ListByUser(ctx context.Context, userID int64) ([]models.Order, error)
}

// OrderService accepts the order numbers users upload for the accrual and
// lists them. The accrual itself is the job of the worker package.
type OrderService struct {
	orders orderStore
}

// NewOrderService returns an OrderService that keeps orders in the store.
func NewOrderService(orders orderStore) *OrderService {
	return &OrderService{orders: orders}
}

// Upload accepts the order for the accrual calculation. It returns
// models.ErrInvalidOrderNumber if the number fails the Luhn check,
// models.ErrOrderAlreadyUploaded if the user has uploaded it before, and
// models.ErrOrderUploadedByAnotherUser if someone else has.
func (s *OrderService) Upload(ctx context.Context, userID int64, number string) error {
	if !luhn.Valid(number) {
		return fmt.Errorf("%w: %q", models.ErrInvalidOrderNumber, number)
	}

	return s.orders.Create(ctx, userID, number)
}

// List returns the orders of the user, newest first.
func (s *OrderService) List(ctx context.Context, userID int64) ([]models.Order, error) {
	return s.orders.ListByUser(ctx, userID)
}
