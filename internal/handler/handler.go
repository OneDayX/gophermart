// Package handler holds the HTTP handlers of the loyalty system API. They
// decode requests, call the services and turn the results and errors into
// HTTP responses; the business rules live in the service package.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/OneDayX/gophermart/internal/auth"
	"go.uber.org/zap"
)

// maxBodySize caps a request body. The largest valid one, a pair of
// credentials, takes a few hundred bytes.
const maxBodySize = 64 << 10 // 64 KiB

// errWrongContentType means the request body is not of the expected type.
var errWrongContentType = errors.New("wrong content type")

// Handler builds the HTTP handlers. Each method takes the service it needs as
// a small interface, so the handlers are tested with stubs.
type Handler struct {
	log *zap.Logger
}

// NewHandler returns a Handler with the given logger. It uses a nop logger
// if none is provided.
func NewHandler(log *zap.Logger) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{log: log}
}

// userID returns the ID of the user the auth middleware has let through. If
// the handler is mounted without the middleware, it answers 401 itself.
func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		h.log.Error("handler is reached without authentication", zap.String("uri", r.RequestURI))
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
	return userID, ok
}

// fail answers with the status the error maps to. Server faults are logged as
// errors, client mistakes as warnings.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, msg string, err error) {
	status := statusForError(err)

	fields := []zap.Field{
		zap.String("uri", r.RequestURI),
		zap.Int("status", status),
		zap.Error(err),
	}
	if status >= http.StatusInternalServerError {
		h.log.Error(msg, fields...)
	} else {
		h.log.Warn(msg, fields...)
	}

	http.Error(w, http.StatusText(status), status)
}

// badRequest answers 400 to a request that could not be decoded.
func (h *Handler) badRequest(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Warn("malformed request",
		zap.String("uri", r.RequestURI),
		zap.Error(err),
	)
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

// writeJSON answers with v encoded as JSON. The body is encoded before the
// status is sent, so an encoding failure still turns into a clean 500.
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		h.log.Error("failed to encode response",
			zap.String("uri", r.RequestURI),
			zap.Error(err),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.log.Warn("failed to write response",
			zap.String("uri", r.RequestURI),
			zap.Error(err),
		)
	}
}

// decodeJSON reads a JSON request body of limited size into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if err := requireContentType(r, "application/json"); err != nil {
		return err
	}

	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(v)
}

// requireContentType checks the media type of the body and ignores its
// parameters, such as the charset.
func requireContentType(r *http.Request, want string) error {
	ct := r.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != want {
		return fmt.Errorf("%w: got %q, want %q", errWrongContentType, ct, want)
	}

	return nil
}
