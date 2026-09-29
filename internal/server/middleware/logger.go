package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// responseData is what the logger reports about a response.
type responseData struct {
	status int
	size   int
}

// responseWriter records the status and the size of the response it passes on.
type responseWriter struct {
	http.ResponseWriter
	responseData responseData
}

// WriteHeader records the status and sends it.
func (rw *responseWriter) WriteHeader(status int) {
	rw.responseData.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Write counts the bytes of the body and writes them.
func (rw *responseWriter) Write(b []byte) (int, error) {
	// Set 200 OK status if the handler did not call WriteHeader explicitly.
	if rw.responseData.status == 0 {
		rw.responseData.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.responseData.size += n
	return n, err
}

// Logger returns a middleware that logs every request with the status and the
// size of its response. Bodies are never logged: they carry passwords.
func Logger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(rw, r)

			// A handler that writes nothing answers 200.
			status := rw.responseData.status
			if status == 0 {
				status = http.StatusOK
			}

			log.Info("request",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", time.Since(start)),
				zap.Int("status", status),
				zap.Int("size", rw.responseData.size),
			)
		})
	}
}
