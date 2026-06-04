package domain

import "time"

// IdempotencyRecord stores the original response for a request so duplicate requests return the same result
type IdempotencyRecord struct {
	Key       string    `json:"key"`
	PaymentID string    `json:"payment_id"`
	Operation string    `json:"operation"` // authorize, capture, void, refund
	Response  []byte    `json:"response"`
	CreatedAt time.Time `json:"created_at"`
}
