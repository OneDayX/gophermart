package models

import "time"

// Balance is the state of a user's loyalty account.
type Balance struct {
	// Current is how many points the user can spend now.
	Current float64
	// Withdrawn is how many points the user has spent since registration.
	Withdrawn float64
}

// Withdrawal is a payment for an order with loyalty points.
type Withdrawal struct {
	// Order is the number of the order paid with the points.
	Order string
	// Sum is how many points were spent.
	Sum float64
	// ProcessedAt is when the points were withdrawn.
	ProcessedAt time.Time
}
