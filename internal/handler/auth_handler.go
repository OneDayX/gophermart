package handler

import (
	"context"
	"net/http"

	"github.com/OneDayX/gophermart/internal/auth"
)

// credentials is the body of the register and login requests.
type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type registrar interface {
	Register(ctx context.Context, login, password string) (string, error)
}

type authenticator interface {
	Login(ctx context.Context, login, password string) (string, error)
}

// Register returns an HTTP handler that signs a user up and at once
// authenticates them.
// URL pattern: POST /api/user/register
func (h *Handler) Register(svc registrar) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds credentials
		if err := decodeJSON(w, r, &creds); err != nil {
			h.badRequest(w, r, err)
			return
		}

		token, err := svc.Register(r.Context(), creds.Login, creds.Password)
		if err != nil {
			h.fail(w, r, "failed to register user", err)
			return
		}

		setToken(w, token)
		w.WriteHeader(http.StatusOK)
	}
}

// Login returns an HTTP handler that authenticates a user by login and
// password.
// URL pattern: POST /api/user/login
func (h *Handler) Login(svc authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds credentials
		if err := decodeJSON(w, r, &creds); err != nil {
			h.badRequest(w, r, err)
			return
		}

		token, err := svc.Login(r.Context(), creds.Login, creds.Password)
		if err != nil {
			h.fail(w, r, "failed to log user in", err)
			return
		}

		setToken(w, token)
		w.WriteHeader(http.StatusOK)
	}
}

// setToken hands the token to the client both ways the API allows, in the
// Authorization header and in a cookie; the client keeps the one it supports.
func setToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", auth.BearerHeader(token))
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
