package handler

import (
	"errors"
	"net/http"

	"github.com/OneDayX/gophermart/internal/models"
)

// statusForError turns a service error into an HTTP status by introspecting
// it with errors.Is. Anything unknown is a server fault.
func statusForError(err error) int {
	switch {
	case errors.Is(err, models.ErrMalformedCredentials),
		errors.Is(err, models.ErrInvalidSum):
		return http.StatusBadRequest

	case errors.Is(err, models.ErrInvalidCredentials):
		return http.StatusUnauthorized

	case errors.Is(err, models.ErrInsufficientFunds):
		return http.StatusPaymentRequired

	case errors.Is(err, models.ErrLoginTaken),
		errors.Is(err, models.ErrOrderUploadedByAnotherUser):
		return http.StatusConflict

	case errors.Is(err, models.ErrInvalidOrderNumber),
		errors.Is(err, models.ErrWithdrawalExists):
		return http.StatusUnprocessableEntity

	default:
		return http.StatusInternalServerError
	}
}
