// Package models describes the entities of the loyalty system: users, their
// orders, loyalty accounts and withdrawals, and the errors all layers share.
// It holds no business logic.
//
// Amounts of points are float64 on the Go side only to carry them between
// JSON and the database. They are stored as numeric, and every addition or
// subtraction happens in SQL, so no rounding errors pile up.
package models

// User is a registered customer of the loyalty system.
type User struct {
	// ID is the surrogate key the rest of the system refers to the user by.
	ID int64
	// Login is unique across all users.
	Login string
	// PasswordHash is the bcrypt hash of the password; the password itself is
	// never stored.
	PasswordHash string
}
