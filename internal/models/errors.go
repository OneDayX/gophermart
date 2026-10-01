package models

import "errors"

// Sentinel errors shared by the repositories, the services and the handlers.
// The handlers turn them into HTTP statuses with errors.Is, so the layers below
// wrap them instead of replacing.
var (
	// ErrMalformedCredentials means the login or the password is empty or too
	// long to be stored.
	ErrMalformedCredentials = errors.New("malformed credentials")

	// ErrLoginTaken means another user has already registered with this login.
	ErrLoginTaken = errors.New("login is already taken")

	// ErrInvalidCredentials means the login is unknown or the password does
	// not match it. The two cases are not told apart on purpose.
	ErrInvalidCredentials = errors.New("invalid login or password")

	// ErrUserNotFound means there is no user with this login or ID.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidOrderNumber means the order number is not a sequence of digits
	// or fails the Luhn check.
	ErrInvalidOrderNumber = errors.New("invalid order number")

	// ErrOrderAlreadyUploaded means the same user has already uploaded this
	// order. It is not a failure: the order is being processed.
	ErrOrderAlreadyUploaded = errors.New("order is already uploaded by this user")

	// ErrOrderUploadedByAnotherUser means another user has already uploaded
	// this order.
	ErrOrderUploadedByAnotherUser = errors.New("order is already uploaded by another user")

	// ErrInvalidSum means the sum to withdraw is not a positive number.
	ErrInvalidSum = errors.New("invalid sum")

	// ErrInsufficientFunds means the balance is less than the sum to withdraw.
	ErrInsufficientFunds = errors.New("insufficient funds")

	// ErrWithdrawalExists means points have already been withdrawn to pay for
	// this order.
	ErrWithdrawalExists = errors.New("points are already withdrawn for this order")
)
