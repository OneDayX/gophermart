package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BalanceRepository keeps the loyalty accounts: the balance of a user and the
// withdrawals from it.
type BalanceRepository struct {
	pool *pgxpool.Pool
}

// NewBalanceRepository returns a BalanceRepository that runs its queries on
// pool.
func NewBalanceRepository(pool *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{pool: pool}
}

// Balance returns the account of the user, or models.ErrUserNotFound.
func (r *BalanceRepository) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	var balance models.Balance

	err := r.pool.QueryRow(ctx,
		`SELECT balance, withdrawn FROM users WHERE id = $1`,
		userID,
	).Scan(&balance.Current, &balance.Withdrawn)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.Balance{}, fmt.Errorf("%w: id %d", models.ErrUserNotFound, userID)
	}
	if err != nil {
		return models.Balance{}, fmt.Errorf("get balance: %w", err)
	}

	return balance, nil
}

// Withdraw takes sum off the balance of the user to pay for the order and
// records the withdrawal, both in one transaction. It returns
// models.ErrInsufficientFunds if the balance is less than sum, and
// models.ErrWithdrawalExists if the order has been paid with points before.
func (r *BalanceRepository) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// The check and the subtraction are one statement: of two parallel
	// withdrawals, the second waits for the row lock and then sees the
	// balance the first one left.
	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET balance = balance - $2, withdrawn = withdrawn + $2
		WHERE id = $1 AND balance >= $2`,
		userID, sum,
	)
	if err != nil {
		return fmt.Errorf("withdraw: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: cannot withdraw %v", models.ErrInsufficientFunds, sum)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`,
		userID, order, sum,
	)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %s", models.ErrWithdrawalExists, order)
	}
	if err != nil {
		return fmt.Errorf("record withdrawal: %w", err)
	}

	return tx.Commit(ctx)
}

// Withdrawals returns the withdrawals of the user, newest first.
func (r *BalanceRepository) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT order_number, sum, processed_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY processed_at DESC, id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}

	withdrawals, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.Withdrawal, error) {
		var w models.Withdrawal
		err := row.Scan(&w.Order, &w.Sum, &w.ProcessedAt)
		return w, err
	})
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}

	return withdrawals, nil
}
