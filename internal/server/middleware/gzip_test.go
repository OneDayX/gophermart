package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipDecompressRequest(t *testing.T) {
	const body = `{"login":"alice","password":"secret"}`

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(body))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, body, string(got))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	Gzip(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGzipCompressResponse(t *testing.T) {
	const body = `{"current":500.5,"withdrawn":42}`

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	Gzip(next).ServeHTTP(rr, req)

	assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

	zr, err := gzip.NewReader(rr.Body)
	require.NoError(t, err)
	got, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
}

func TestGzipInvalidRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	Gzip(next).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
