package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderRepository stores the orders users upload and the accruals for them.
type OrderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository returns an OrderRepository that runs its queries on pool.
func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

// Create stores a new order of the user with status NEW. When the number is
// already taken, it returns models.ErrOrderAlreadyUploaded if it was the same
// user, and models.ErrOrderUploadedByAnotherUser otherwise.
func (r *OrderRepository) Create(ctx context.Context, userID int64, number string) error {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO orders (number, user_id) VALUES ($1, $2) ON CONFLICT (number) DO NOTHING`,
		number, userID,
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	// A separate statement takes a fresh snapshot, so it sees the row even if
	// a parallel upload of the same number committed it a moment ago.
	var owner int64
	err = r.pool.QueryRow(ctx, `SELECT user_id FROM orders WHERE number = $1`, number).Scan(&owner)
	if err != nil {
		return fmt.Errorf("find order owner: %w", err)
	}

	if owner == userID {
		return fmt.Errorf("%w: %s", models.ErrOrderAlreadyUploaded, number)
	}
	return fmt.Errorf("%w: %s", models.ErrOrderUploadedByAnotherUser, number)
}

// ListByUser returns the orders of the user, newest first.
func (r *OrderRepository) ListByUser(ctx context.Context, userID int64) ([]models.Order, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT number, user_id, status, accrual, uploaded_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC, number DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.Order, error) {
		var o models.Order
		err := row.Scan(&o.Number, &o.UserID, &o.Status, &o.Accrual, &o.UploadedAt)
		return o, err
	})
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	return orders, nil
}

// Pending returns the numbers of at most limit orders that have no final
// status yet, the ones checked longest ago first.
func (r *OrderRepository) Pending(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT number
		FROM orders
		WHERE status IN ('NEW', 'PROCESSING')
		ORDER BY checked_at
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending orders: %w", err)
	}

	numbers, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("list pending orders: %w", err)
	}

	return numbers, nil
}

// ApplyAccrual records the status the accrual system reported for the order
// and marks it as checked. When the order becomes PROCESSED, the accrual is
// credited to its owner in the same transaction. An order that already has a
// final status is left alone, so a repeated call never credits twice.
func (r *OrderRepository) ApplyAccrual(ctx context.Context, number string, status models.OrderStatus, accrual *float64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// The update locks the row, so a parallel call for the same order waits
	// here and then finds the status final.
	var userID int64
	err = tx.QueryRow(ctx, `
		UPDATE orders
		SET status = $2, accrual = $3, checked_at = now()
		WHERE number = $1 AND status NOT IN ('INVALID', 'PROCESSED')
		RETURNING user_id`,
		number, status, accrual,
	).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("update order %s: %w", number, err)
	}

	if status == models.OrderStatusProcessed && accrual != nil && *accrual > 0 {
		_, err := tx.Exec(ctx, `UPDATE users SET balance = balance + $2 WHERE id = $1`, userID, *accrual)
		if err != nil {
			return fmt.Errorf("credit accrual for order %s: %w", number, err)
		}
	}

	return tx.Commit(ctx)
}
