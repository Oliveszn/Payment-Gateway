package domain

import (
	"errors"
	"time"
)

// PaymentState is the state of a payment
type PaymentState string

const (
	StatePending    PaymentState = "PENDING"
	StateAuthorized PaymentState = "AUTHORIZED"
	StateCaptured   PaymentState = "CAPTURED"
	StateVoided     PaymentState = "VOIDED"
	StateRefunded   PaymentState = "REFUNDED"
)

// This is basically the receiept/source of truth
type Payment struct {
	ID         string       `json:"id"`          // gateway's ID
	OrderID    string       `json:"order_id"`    // from FicMart(bank)
	CustomerID string       `json:"customer_id"` // from FicMart(bank)
	Amount     int64        `json:"amount"`
	Currency   string       `json:"currency"`
	State      PaymentState `json:"state"`
	CardNumber string       `json:"card_number"`
	CardExpiry string       `json:"card_expiry"`
	CardCVV    string       `json:"card_cvv"`

	// Bank reference IDs, filled in as payment progresses
	BankAuthID    string `json:"bank_auth_id,omitempty"`
	BankCaptureID string `json:"bank_capture_id,omitempty"`
	BankVoidID    string `json:"bank_void_id,omitempty"`
	BankRefundID  string `json:"bank_refund_id,omitempty"`

	CreatedAt    time.Time  `json:"created_at"`
	AuthorizedAt *time.Time `json:"authorized_at,omitempty"`
	CapturedAt   *time.Time `json:"captured_at,omitempty"`
	VoidedAt     *time.Time `json:"voided_at,omitempty"`
	RefundedAt   *time.Time `json:"refunded_at,omitempty"`
}

// Domain errors
var (
	ErrPaymentNotFound        = errors.New("payment not found")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrDuplicatePayment       = errors.New("duplicate payment")
	ErrInsufficientFunds      = errors.New("insufficient funds")
	ErrInvalidCard            = errors.New("invalid card details")
	ErrCardExpired            = errors.New("card expired")
	ErrBankUnavailable        = errors.New("bank temporarily unavailable")
)

// CanAuthorize checks if payment can move to AUTHORIZED
func (p *Payment) CanAuthorize() bool {
	return p.State == StatePending
}

// CanCapture checks if payment can move to CAPTURED
func (p *Payment) CanCapture() bool {
	return p.State == StateAuthorized
}

// CanVoid checks if payment can move to VOIDED
func (p *Payment) CanVoid() bool {
	return p.State == StateAuthorized
}

// CanRefund checks if payment can move to REFUNDED
func (p *Payment) CanRefund() bool {
	return p.State == StateCaptured
}

// Authorize transitions payment to AUTHORIZED state
func (p *Payment) Authorize(bankAuthID string) error {
	if !p.CanAuthorize() {
		return ErrInvalidStateTransition
	}
	now := time.Now()
	p.State = StateAuthorized
	p.BankAuthID = bankAuthID
	p.AuthorizedAt = &now
	return nil
}

// Capture transitions payment to CAPTURED state
func (p *Payment) Capture(bankCaptureID string) error {
	if !p.CanCapture() {
		return ErrInvalidStateTransition
	}
	now := time.Now()
	p.State = StateCaptured
	p.BankCaptureID = bankCaptureID
	p.CapturedAt = &now
	return nil
}

// Void transitions payment to VOIDED state
func (p *Payment) Void(bankVoidID string) error {
	if !p.CanVoid() {
		return ErrInvalidStateTransition
	}
	now := time.Now()
	p.State = StateVoided
	p.BankVoidID = bankVoidID
	p.VoidedAt = &now
	return nil
}

// Refund transitions payment to REFUNDED state
func (p *Payment) Refund(bankRefundID string) error {
	if !p.CanRefund() {
		return ErrInvalidStateTransition
	}
	now := time.Now()
	p.State = StateRefunded
	p.BankRefundID = bankRefundID
	p.RefundedAt = &now
	return nil
}
