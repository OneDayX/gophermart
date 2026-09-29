package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUsers struct {
	byLogin map[string]models.User
}

func (u *fakeUsers) Create(ctx context.Context, login, passwordHash string) (int64, error) {
	if _, ok := u.byLogin[login]; ok {
		return 0, models.ErrLoginTaken
	}
	id := int64(len(u.byLogin) + 1)
	u.byLogin[login] = models.User{ID: id, Login: login, PasswordHash: passwordHash}
	return id, nil
}

func (u *fakeUsers) GetByLogin(ctx context.Context, login string) (models.User, error) {
	user, ok := u.byLogin[login]
	if !ok {
		return models.User{}, models.ErrUserNotFound
	}
	return user, nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(userID int64) (string, error) {
	return fmt.Sprintf("token-%d", userID), nil
}

func TestUserService(t *testing.T) {
	svc := NewUserService(&fakeUsers{byLogin: map[string]models.User{}}, fakeTokens{})
	ctx := context.Background()

	token, err := svc.Register(ctx, "alice", "secret")
	require.NoError(t, err)
	assert.Equal(t, "token-1", token)

	_, err = svc.Register(ctx, "alice", "other")
	assert.ErrorIs(t, err, models.ErrLoginTaken)

	_, err = svc.Register(ctx, "", "secret")
	assert.ErrorIs(t, err, models.ErrMalformedCredentials)

	token, err = svc.Login(ctx, "alice", "secret")
	require.NoError(t, err)
	assert.Equal(t, "token-1", token)

	_, err = svc.Login(ctx, "alice", "wrong")
	assert.ErrorIs(t, err, models.ErrInvalidCredentials)

	_, err = svc.Login(ctx, "bob", "secret")
	assert.ErrorIs(t, err, models.ErrInvalidCredentials)
}

type fakeOrders struct {
	created []string
}

func (o *fakeOrders) Create(ctx context.Context, userID int64, number string) error {
	o.created = append(o.created, number)
	return nil
}

func (o *fakeOrders) ListByUser(ctx context.Context, userID int64) ([]models.Order, error) {
	return nil, nil
}

func TestOrderService_Upload(t *testing.T) {
	orders := &fakeOrders{}
	svc := NewOrderService(orders)

	require.NoError(t, svc.Upload(context.Background(), 1, "12345678903"))

	err := svc.Upload(context.Background(), 1, "12345678901")
	assert.ErrorIs(t, err, models.ErrInvalidOrderNumber)

	assert.Equal(t, []string{"12345678903"}, orders.created)

	_, err = svc.List(context.Background(), 1)
	assert.NoError(t, err)
}

type fakeBalances struct {
	withdrawn []float64
}

func (b *fakeBalances) Balance(ctx context.Context, userID int64) (models.Balance, error) {
	return models.Balance{}, nil
}

func (b *fakeBalances) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	b.withdrawn = append(b.withdrawn, sum)
	return nil
}

func (b *fakeBalances) Withdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return nil, nil
}

func TestBalanceService(t *testing.T) {
	svc := NewBalanceService(&fakeBalances{})

	_, err := svc.Balance(context.Background(), 1)
	assert.NoError(t, err)

	_, err = svc.Withdrawals(context.Background(), 1)
	assert.NoError(t, err)
}

func TestBalanceService_Withdraw(t *testing.T) {
	tests := []struct {
		name    string
		order   string
		sum     float64
		wantErr error
	}{
		{name: "valid", order: "2377225624", sum: 751},
		{name: "invalid order", order: "2377225625", sum: 751, wantErr: models.ErrInvalidOrderNumber},
		{name: "zero sum", order: "2377225624", sum: 0, wantErr: models.ErrInvalidSum},
		{name: "negative sum", order: "2377225624", sum: -1, wantErr: models.ErrInvalidSum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeBalances{}
			err := NewBalanceService(store).Withdraw(context.Background(), 1, tt.order, tt.sum)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, store.withdrawn)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []float64{tt.sum}, store.withdrawn)
		})
	}
}
