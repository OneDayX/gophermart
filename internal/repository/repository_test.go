package repository

import (
	"context"
	"os"
	"testing"

	"github.com/OneDayX/gophermart/internal/database"
	"github.com/OneDayX/gophermart/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
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

	return db.Pool()
}

func TestUserRepository(t *testing.T) {
	users := NewUserRepository(newTestPool(t))
	ctx := context.Background()

	id, err := users.Create(ctx, "alice", "hash")
	require.NoError(t, err)

	_, err = users.Create(ctx, "alice", "hash")
	assert.ErrorIs(t, err, models.ErrLoginTaken)

	user, err := users.GetByLogin(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, models.User{ID: id, Login: "alice", PasswordHash: "hash"}, user)

	_, err = users.GetByLogin(ctx, "bob")
	assert.ErrorIs(t, err, models.ErrUserNotFound)
}

func TestOrderRepository(t *testing.T) {
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	orders := NewOrderRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, "alice", "hash")
	require.NoError(t, err)
	bob, err := users.Create(ctx, "bob", "hash")
	require.NoError(t, err)

	require.NoError(t, orders.Create(ctx, alice, "1"))
	require.NoError(t, orders.Create(ctx, alice, "2"))
	assert.ErrorIs(t, orders.Create(ctx, alice, "1"), models.ErrOrderAlreadyUploaded)
	assert.ErrorIs(t, orders.Create(ctx, bob, "1"), models.ErrOrderUploadedByAnotherUser)

	list, err := orders.ListByUser(ctx, alice)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "2", list[0].Number, "newest first")
	assert.Equal(t, models.OrderStatusNew, list[0].Status)

	pending, err := orders.Pending(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 2)
}

func TestOrderRepository_ApplyAccrual(t *testing.T) {
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	orders := NewOrderRepository(pool)
	balances := NewBalanceRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, "alice", "hash")
	require.NoError(t, err)
	require.NoError(t, orders.Create(ctx, alice, "1"))

	accrual := 729.98
	require.NoError(t, orders.ApplyAccrual(ctx, "1", models.OrderStatusProcessed, &accrual))
	// A processed order is final, the second answer must not credit again.
	require.NoError(t, orders.ApplyAccrual(ctx, "1", models.OrderStatusProcessed, &accrual))

	balance, err := balances.Balance(ctx, alice)
	require.NoError(t, err)
	assert.Equal(t, 729.98, balance.Current)

	pending, err := orders.Pending(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestBalanceRepository_Withdraw(t *testing.T) {
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	orders := NewOrderRepository(pool)
	balances := NewBalanceRepository(pool)
	ctx := context.Background()

	alice, err := users.Create(ctx, "alice", "hash")
	require.NoError(t, err)
	require.NoError(t, orders.Create(ctx, alice, "1"))
	accrual := 500.5
	require.NoError(t, orders.ApplyAccrual(ctx, "1", models.OrderStatusProcessed, &accrual))

	err = balances.Withdraw(ctx, alice, "2377225624", 751)
	assert.ErrorIs(t, err, models.ErrInsufficientFunds)

	require.NoError(t, balances.Withdraw(ctx, alice, "2377225624", 42))

	err = balances.Withdraw(ctx, alice, "2377225624", 1)
	assert.ErrorIs(t, err, models.ErrWithdrawalExists)

	balance, err := balances.Balance(ctx, alice)
	require.NoError(t, err)
	assert.Equal(t, models.Balance{Current: 458.5, Withdrawn: 42}, balance)

	withdrawals, err := balances.Withdrawals(ctx, alice)
	require.NoError(t, err)
	require.Len(t, withdrawals, 1)
	assert.Equal(t, "2377225624", withdrawals[0].Order)
	assert.Equal(t, 42.0, withdrawals[0].Sum)
}
