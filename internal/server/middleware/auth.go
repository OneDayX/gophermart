package middleware

import (
	"net/http"

	"github.com/OneDayX/gophermart/internal/auth"
	"go.uber.org/zap"
)

type tokenParser interface {
	Parse(token string) (int64, error)
}

// Auth returns a middleware that lets through only requests with a valid
// token and puts the ID of the user into the request context. The token is
// taken from the Authorization header, or from the cookie if there is no
// header. Everything else gets 401.
func Auth(tokens tokenParser, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := tokenFromRequest(r)
			if token == "" {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			userID, err := tokens.Parse(token)
			if err != nil {
				log.Info("rejected token",
					zap.String("uri", r.RequestURI),
					zap.Error(err),
				)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), userID)))
		})
	}
}

// tokenFromRequest returns the token the request carries, or an empty string.
func tokenFromRequest(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		return auth.TokenFromHeader(header)
	}

	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		return cookie.Value
	}

	return ""
}
