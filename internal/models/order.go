package models

import "time"

// OrderStatus is the stage of the reward calculation an order is at.
type OrderStatus string

const (
	// OrderStatusNew means the order is uploaded, but the accrual system has
	// not taken it yet.
	OrderStatusNew OrderStatus = "NEW"
	// OrderStatusProcessing means the accrual system is calculating the reward.
	OrderStatusProcessing OrderStatus = "PROCESSING"
	// OrderStatusInvalid means the accrual system refused to calculate the
	// reward. The status is final.
	OrderStatusInvalid OrderStatus = "INVALID"
	// OrderStatusProcessed means the reward is calculated and credited to the
	// user. The status is final.
	OrderStatusProcessed OrderStatus = "PROCESSED"
)

// Order is an order number a user uploaded to get loyalty points for.
type Order struct {
	// Number is the order number; it is unique across all users.
	Number string
	// UserID is the ID of the user who uploaded the order.
	UserID int64
	// Status is the stage of the reward calculation.
	Status OrderStatus
	// Accrual is the reward credited for the order. It is nil until the order
	// is processed, and stays nil for an order that earns nothing.
	Accrual *float64
	// UploadedAt is when the user uploaded the order.
	UploadedAt time.Time
}
