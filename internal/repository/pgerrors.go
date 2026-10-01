// Package repository keeps the loyalty system data in PostgreSQL. It turns
// database facts, such as a taken unique key, into the errors of the models
// package and holds no business rules.
package repository

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether the statement failed on a unique key.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
