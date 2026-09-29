package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OneDayX/gophermart/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		svcErr      error
		wantCode    int
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `{"login":"alice","password":"secret"}`,
			wantCode:    http.StatusOK,
		},
		{
			name:        "wrong content type",
			contentType: "text/plain",
			body:        `{"login":"alice","password":"secret"}`,
			wantCode:    http.StatusBadRequest,
		},
		{
			name:        "broken json",
			contentType: "application/json",
			body:        `{"login":`,
			wantCode:    http.StatusBadRequest,
		},
		{
			name:        "login taken",
			contentType: "application/json",
			body:        `{"login":"alice","password":"secret"}`,
			svcErr:      models.ErrLoginTaken,
			wantCode:    http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(nil).Register(stubUsers{err: tt.svcErr})(w, newRequest(http.MethodPost, "/api/user/register", tt.contentType, tt.body))

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantCode == http.StatusOK {
				assert.Equal(t, "Bearer the-token", w.Header().Get("Authorization"))
				assert.Len(t, w.Result().Cookies(), 1)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	body := `{"login":"alice","password":"secret"}`

	w := httptest.NewRecorder()
	NewHandler(nil).Login(stubUsers{})(w, newRequest(http.MethodPost, "/api/user/login", "application/json", body))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer the-token", w.Header().Get("Authorization"))

	w = httptest.NewRecorder()
	NewHandler(nil).Login(stubUsers{err: models.ErrInvalidCredentials})(w, newRequest(http.MethodPost, "/api/user/login", "application/json", body))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
