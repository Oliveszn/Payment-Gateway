package service

import (
	"context"
	"fmt"
	"payment-gateway/internal/bank"
	"payment-gateway/internal/domain"
	"payment-gateway/internal/repository"
	"time"

	"github.com/google/uuid"
)

// PaymentService orchestrates all payment operations
type PaymentService struct {
	repo       repository.PaymentRepository
	bankClient *bank.Client
}

func New(repo repository.PaymentRepository, bankClient *bank.Client) *PaymentService {
	return &PaymentService{
		repo:       repo,
		bankClient: bankClient,
	}
}

type AuthorizeRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"card_expiry"`
	CardCVV    string `json:"card_cvv"`
}

type PaymentResponse struct {
	PaymentID    string              `json:"payment_id"`
	OrderID      string              `json:"order_id"`
	CustomerID   string              `json:"customer_id"`
	Amount       int64               `json:"amount"`
	Currency     string              `json:"currency"`
	State        domain.PaymentState `json:"state"`
	BankAuthID   string              `json:"bank_auth_id,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	AuthorizedAt *time.Time          `json:"authorized_at,omitempty"`
	CapturedAt   *time.Time          `json:"captured_at,omitempty"`
	VoidedAt     *time.Time          `json:"voided_at,omitempty"`
	RefundedAt   *time.Time          `json:"refunded_at,omitempty"`
}

// Authorize reserves funds on the card. save PENDING → call bank → update to AUTHORIZED
func (s *PaymentService) Authorize(ctx context.Context, idempotencyKey string, req AuthorizeRequest) (*PaymentResponse, error) {
	//1 Check idempotency
	existing, err := s.repo.GetIdempotencyRecord(ctx, idempotencyKey, "authorize")
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		// seen this request before return the original response
		return deserializeResponse(existing.Response)
	}

	//2 Create payment in PENDING state first
	//if we crash after calling the bank but before saving, we can detect the orphaned PENDING payment on recovery.
	payment := &domain.Payment{
		ID:         uuid.New().String(),
		OrderID:    req.OrderID,
		CustomerID: req.CustomerID,
		Amount:     req.Amount,
		Currency:   req.Currency,
		State:      domain.StatePending,
		CardNumber: req.CardNumber,
		CardExpiry: req.CardExpiry,
		CardCVV:    req.CardCVV,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	//3 Call the bank to authorize
	bankResp, err := s.bankClient.Authorize(ctx, idempotencyKey, bank.AuthorizeRequest{
		Amount:     req.Amount,
		Currency:   req.Currency,
		CardNumber: req.CardNumber,
		CardExpiry: req.CardExpiry,
		CardCVV:    req.CardCVV,
	})
	if err != nil {
		//Bank rejected permanently delete the pending payment so FicMart can retry with a fresh request
		_ = s.repo.UpdatePayment(ctx, payment)
		return nil, err
	}

	//4 Transition to AUTHORIZED and save
	if err := payment.Authorize(bankResp.AuthorizationID); err != nil {
		return nil, fmt.Errorf("authorize transition: %w", err)
	}

	if err := s.repo.UpdatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("save authorized payment: %w", err)
	}

	//5 Save idempotency record so duplicate requests return this response
	resp := toPaymentResponse(payment)
	if err := s.saveIdempotencyRecord(ctx, idempotencyKey, "authorize", payment.ID, resp); err != nil {
		// Non-fatal — log in production but don't fail the request
		_ = err
	}

	return resp, nil
}

// Capture charges the previously authorized amount. load payment → validate state → call bank → update to CAPTURED
func (s *PaymentService) Capture(ctx context.Context, idempotencyKey, paymentID string) (*PaymentResponse, error) {
	//check idempotency
	existing, err := s.repo.GetIdempotencyRecord(ctx, idempotencyKey, "capture")
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return deserializeResponse(existing.Response)
	}

	//load payment
	payment, err := s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	//validate state transition before calling bank
	if !payment.CanCapture() {
		return nil, fmt.Errorf("%w: cannot capture a %s payment", domain.ErrInvalidStateTransition, payment.State)
	}

	//call bank
	bankResp, err := s.bankClient.Capture(ctx, idempotencyKey, bank.CaptureRequest{
		AuthorizationID: payment.BankAuthID,
		Amount:          payment.Amount,
	})
	if err != nil {
		return nil, err
	}

	//transition to CAPTURED
	if err := payment.Capture(bankResp.CaptureID); err != nil {
		return nil, fmt.Errorf("capture transition: %w", err)
	}

	if err := s.repo.UpdatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("save captured payment: %w", err)
	}

	resp := toPaymentResponse(payment)
	_ = s.saveIdempotencyRecord(ctx, idempotencyKey, "capture", payment.ID, resp)

	return resp, nil
}

// Void cancels an authorization before capture.
func (s *PaymentService) Void(ctx context.Context, idempotencyKey, paymentID string) (*PaymentResponse, error) {
	existing, err := s.repo.GetIdempotencyRecord(ctx, idempotencyKey, "void")
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return deserializeResponse(existing.Response)
	}

	payment, err := s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if !payment.CanVoid() {
		return nil, fmt.Errorf("%w: cannot void a %s payment", domain.ErrInvalidStateTransition, payment.State)
	}

	bankResp, err := s.bankClient.Void(ctx, idempotencyKey, bank.VoidRequest{
		AuthorizationID: payment.BankAuthID,
	})
	if err != nil {
		return nil, err
	}

	if err := payment.Void(bankResp.VoidID); err != nil {
		return nil, fmt.Errorf("void transition: %w", err)
	}

	if err := s.repo.UpdatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("save voided payment: %w", err)
	}

	resp := toPaymentResponse(payment)
	_ = s.saveIdempotencyRecord(ctx, idempotencyKey, "void", payment.ID, resp)

	return resp, nil
}

// Refund returns money after a capture.
func (s *PaymentService) Refund(ctx context.Context, idempotencyKey, paymentID string) (*PaymentResponse, error) {
	existing, err := s.repo.GetIdempotencyRecord(ctx, idempotencyKey, "refund")
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return deserializeResponse(existing.Response)
	}

	payment, err := s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}

	if !payment.CanRefund() {
		return nil, fmt.Errorf("%w: cannot refund a %s payment", domain.ErrInvalidStateTransition, payment.State)
	}

	bankResp, err := s.bankClient.Refund(ctx, idempotencyKey, bank.RefundRequest{
		CaptureID: payment.BankCaptureID,
		Amount:    payment.Amount,
	})
	if err != nil {
		return nil, err
	}

	if err := payment.Refund(bankResp.RefundID); err != nil {
		return nil, fmt.Errorf("refund transition: %w", err)
	}

	if err := s.repo.UpdatePayment(ctx, payment); err != nil {
		return nil, fmt.Errorf("save refunded payment: %w", err)
	}

	resp := toPaymentResponse(payment)
	_ = s.saveIdempotencyRecord(ctx, idempotencyKey, "refund", payment.ID, resp)

	return resp, nil
}

// GetByOrderID returns payment status for a given order
func (s *PaymentService) GetByOrderID(ctx context.Context, orderID string) (*PaymentResponse, error) {
	payment, err := s.repo.GetPaymentByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toPaymentResponse(payment), nil
}

// GetByCustomerID returns all payments for a customer
func (s *PaymentService) GetByCustomerID(ctx context.Context, customerID string) ([]*PaymentResponse, error) {
	payments, err := s.repo.GetPaymentsByCustomerID(ctx, customerID)
	if err != nil {
		return nil, err
	}
	var responses []*PaymentResponse
	for _, p := range payments {
		responses = append(responses, toPaymentResponse(p))
	}
	return responses, nil
}
