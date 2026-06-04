package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"payment-gateway/internal/domain"

	_ "github.com/lib/pq"
)

type PaymentRepository interface {
	// Payment
	CreatePayment(ctx context.Context, payment *domain.Payment) error
	GetPaymentByID(ctx context.Context, id string) (*domain.Payment, error)
	GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error)
	GetPaymentsByCustomerID(ctx context.Context, customerID string) ([]*domain.Payment, error)
	UpdatePayment(ctx context.Context, payment *domain.Payment) error

	// Idempotency
	GetIdempotencyRecord(ctx context.Context, key, operation string) (*domain.IdempotencyRecord, error)
	SaveIdempotencyRecord(ctx context.Context, record *domain.IdempotencyRecord) error
}

type postgresRepo struct {
	db *sql.DB
}

func New(db *sql.DB) PaymentRepository {
	return &postgresRepo{db: db}
}

// CreatePayment creates a new payment record
func (r *postgresRepo) CreatePayment(ctx context.Context, p *domain.Payment) error {
	query := `
INSERT INTO payments (
id, order_id, customer_id, amount, currency, state,
card_number, card_expiry, card_cvv, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.OrderID, p.CustomerID, p.Amount, p.Currency, p.State,
		p.CardNumber, p.CardExpiry, p.CardCVV, p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// GetPaymentByID fetches a single payment by ID(gateway's)
func (r *postgresRepo) GetPaymentByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `
SELECT
id, order_id, customer_id, amount, currency, state,
card_number, card_expiry, card_cvv,
COALESCE(bank_auth_id, ''), COALESCE(bank_capture_id, ''),
COALESCE(bank_void_id, ''), COALESCE(bank_refund_id, ''),
created_at, authorized_at, captured_at, voided_at, refunded_at
FROM payments WHERE id = $1`

	p := &domain.Payment{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.OrderID, &p.CustomerID, &p.Amount, &p.Currency, &p.State,
		&p.CardNumber, &p.CardExpiry, &p.CardCVV,
		&p.BankAuthID, &p.BankCaptureID, &p.BankVoidID, &p.BankRefundID,
		&p.CreatedAt, &p.AuthorizedAt, &p.CapturedAt, &p.VoidedAt, &p.RefundedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	return p, nil
}

// GetPaymentByOrderID fetches a payment by FicMart's(bank) order ID
func (r *postgresRepo) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	query := `
SELECT
id, order_id, customer_id, amount, currency, state,
card_number, card_expiry, card_cvv,
COALESCE(bank_auth_id, ''), COALESCE(bank_capture_id, ''),
COALESCE(bank_void_id, ''), COALESCE(bank_refund_id, ''),
created_at, authorized_at, captured_at, voided_at, refunded_at
FROM payments WHERE order_id = $1`

	p := &domain.Payment{}
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&p.ID, &p.OrderID, &p.CustomerID, &p.Amount, &p.Currency, &p.State,
		&p.CardNumber, &p.CardExpiry, &p.CardCVV,
		&p.BankAuthID, &p.BankCaptureID, &p.BankVoidID, &p.BankRefundID,
		&p.CreatedAt, &p.AuthorizedAt, &p.CapturedAt, &p.VoidedAt, &p.RefundedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payment by order id: %w", err)
	}
	return p, nil
}

// GetPaymentsByCustomerID fetches all payments for a customer
func (r *postgresRepo) GetPaymentsByCustomerID(ctx context.Context, customerID string) ([]*domain.Payment, error) {
	query := `
SELECT
id, order_id, customer_id, amount, currency, state,
card_number, card_expiry, card_cvv,
COALESCE(bank_auth_id, ''), COALESCE(bank_capture_id, ''),
COALESCE(bank_void_id, ''), COALESCE(bank_refund_id, ''),
created_at, authorized_at, captured_at, voided_at, refunded_at
FROM payments WHERE customer_id = $1
ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("get payments by customer: %w", err)
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		err := rows.Scan(
			&p.ID, &p.OrderID, &p.CustomerID, &p.Amount, &p.Currency, &p.State,
			&p.CardNumber, &p.CardExpiry, &p.CardCVV,
			&p.BankAuthID, &p.BankCaptureID, &p.BankVoidID, &p.BankRefundID,
			&p.CreatedAt, &p.AuthorizedAt, &p.CapturedAt, &p.VoidedAt, &p.RefundedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		payments = append(payments, p)
	}
	return payments, nil
}

// UpdatePayment saves state changes after a transition
func (r *postgresRepo) UpdatePayment(ctx context.Context, p *domain.Payment) error {
	query := `
UPDATE payments SET
state          = $1,
bank_auth_id   = $2,
bank_capture_id = $3,
bank_void_id   = $4,
bank_refund_id = $5,
authorized_at  = $6,
captured_at    = $7,
voided_at      = $8,
refunded_at    = $9
WHERE id = $10`

	_, err := r.db.ExecContext(ctx, query,
		p.State,
		nullableString(p.BankAuthID),
		nullableString(p.BankCaptureID),
		nullableString(p.BankVoidID),
		nullableString(p.BankRefundID),
		p.AuthorizedAt,
		p.CapturedAt,
		p.VoidedAt,
		p.RefundedAt,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	return nil
}

// GetIdempotencyRecord looks up a previous request by key
func (r *postgresRepo) GetIdempotencyRecord(ctx context.Context, key, operation string) (*domain.IdempotencyRecord, error) {
	query := `
SELECT key, operation, payment_id, response, created_at
FROM idempotency_keys
WHERE key = $1 AND operation = $2`

	rec := &domain.IdempotencyRecord{}
	err := r.db.QueryRowContext(ctx, query, key, operation).Scan(
		&rec.Key, &rec.Operation, &rec.PaymentID, &rec.Response, &rec.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get idempotency record: %w", err)
	}
	return rec, nil
}

// SaveIdempotencyRecord stores the response for a completed request
func (r *postgresRepo) SaveIdempotencyRecord(ctx context.Context, rec *domain.IdempotencyRecord) error {
	query := `
INSERT INTO idempotency_keys (key, operation, payment_id, response, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (key, operation) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		rec.Key, rec.Operation, rec.PaymentID, rec.Response, rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save idempotency record: %w", err)
	}
	return nil
}

// nullableString converts empty string to NULL for postgres
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
