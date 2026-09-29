package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})

	rr := httptest.NewRecorder()
	Logger(zap.NewNop())(next).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/user/orders", nil))

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}
