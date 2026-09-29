package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OneDayX/gophermart/internal/auth"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type stubTokens struct{}

func (stubTokens) Parse(token string) (int64, error) {
	if token != "good" {
		return 0, errors.New("bad token")
	}
	return 42, nil
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		cookie   string
		wantCode int
	}{
		{name: "header", header: "Bearer good", wantCode: http.StatusOK},
		{name: "cookie", cookie: "good", wantCode: http.StatusOK},
		{name: "no token", wantCode: http.StatusUnauthorized},
		{name: "bad token", header: "Bearer bad", wantCode: http.StatusUnauthorized},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserID(r.Context())
		assert.Equal(t, int64(42), userID)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tt.cookie})
			}

			rr := httptest.NewRecorder()
			Auth(stubTokens{}, zap.NewNop())(next).ServeHTTP(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)
		})
	}
}
