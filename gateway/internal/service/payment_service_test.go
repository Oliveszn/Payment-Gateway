package service_test

import (
	"context"
	"payment-gateway/internal/bank"
	"payment-gateway/internal/domain"
	"payment-gateway/internal/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//Mock repository

type mockRepo struct {
	payments        map[string]*domain.Payment
	idempotencyKeys map[string]*domain.IdempotencyRecord
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		payments:        make(map[string]*domain.Payment),
		idempotencyKeys: make(map[string]*domain.IdempotencyRecord),
	}
}

func (m *mockRepo) CreatePayment(_ context.Context, p *domain.Payment) error {
	m.payments[p.ID] = p
	return nil
}

func (m *mockRepo) GetPaymentByID(_ context.Context, id string) (*domain.Payment, error) {
	p, ok := m.payments[id]
	if !ok {
		return nil, domain.ErrPaymentNotFound
	}
	return p, nil
}

func (m *mockRepo) GetPaymentByOrderID(_ context.Context, orderID string) (*domain.Payment, error) {
	for _, p := range m.payments {
		if p.OrderID == orderID {
			return p, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}

func (m *mockRepo) GetPaymentsByCustomerID(_ context.Context, customerID string) ([]*domain.Payment, error) {
	var results []*domain.Payment
	for _, p := range m.payments {
		if p.CustomerID == customerID {
			results = append(results, p)
		}
	}
	return results, nil
}

func (m *mockRepo) UpdatePayment(_ context.Context, p *domain.Payment) error {
	m.payments[p.ID] = p
	return nil
}

func (m *mockRepo) GetIdempotencyRecord(_ context.Context, key, operation string) (*domain.IdempotencyRecord, error) {
	rec, ok := m.idempotencyKeys[key+":"+operation]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

func (m *mockRepo) SaveIdempotencyRecord(_ context.Context, rec *domain.IdempotencyRecord) error {
	m.idempotencyKeys[rec.Key+":"+rec.Operation] = rec
	return nil
}

// Mock bank client

type mockBank struct {
	authorizeResp *bank.AuthorizeResponse
	authorizeErr  error
	captureResp   *bank.CaptureResponse
	captureErr    error
	voidResp      *bank.VoidResponse
	voidErr       error
	refundResp    *bank.RefundResponse
	refundErr     error
	callCount     int
}

func (m *mockBank) Authorize(_ context.Context, _ string, _ bank.AuthorizeRequest) (*bank.AuthorizeResponse, error) {
	m.callCount++
	return m.authorizeResp, m.authorizeErr
}

func (m *mockBank) Capture(_ context.Context, _ string, _ bank.CaptureRequest) (*bank.CaptureResponse, error) {
	m.callCount++
	return m.captureResp, m.captureErr
}

func (m *mockBank) Void(_ context.Context, _ string, _ bank.VoidRequest) (*bank.VoidResponse, error) {
	m.callCount++
	return m.voidResp, m.voidErr
}

func (m *mockBank) Refund(_ context.Context, _ string, _ bank.RefundRequest) (*bank.RefundResponse, error) {
	m.callCount++
	return m.refundResp, m.refundErr
}

// Helpers

func newAuthorizeRequest() service.AuthorizeRequest {
	return service.AuthorizeRequest{
		OrderID:    "order-001",
		CustomerID: "customer-001",
		Amount:     5000,
		Currency:   "USD",
		CardNumber: "4111111111111111",
		CardExpiry: "12/2030",
		CardCVV:    "123",
	}
}

//Tests

func TestService_Authorize_HappyPath(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
			Amount:          5000,
		},
	}
	svc := service.New(repo, b)

	resp, err := svc.Authorize(context.Background(), "idem-key-001", newAuthorizeRequest())
	require.NoError(t, err)
	assert.Equal(t, domain.StateAuthorized, resp.State)
	assert.Equal(t, "bank-auth-001", resp.BankAuthID)
	assert.Equal(t, int64(5000), resp.Amount)
	assert.Equal(t, 1, b.callCount)
}

func TestService_Authorize_Idempotency(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
			Amount:          5000,
		},
	}
	svc := service.New(repo, b)

	// First request
	resp1, err := svc.Authorize(context.Background(), "idem-key-001", newAuthorizeRequest())
	require.NoError(t, err)

	// Duplicate request with same key
	resp2, err := svc.Authorize(context.Background(), "idem-key-001", newAuthorizeRequest())
	require.NoError(t, err)

	// Same payment ID returned
	assert.Equal(t, resp1.PaymentID, resp2.PaymentID)

	// Bank was only called once
	assert.Equal(t, 1, b.callCount)
}

func TestService_Authorize_InsufficientFunds(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeErr: domain.ErrInsufficientFunds,
	}
	svc := service.New(repo, b)

	_, err := svc.Authorize(context.Background(), "idem-key-002", newAuthorizeRequest())
	assert.ErrorIs(t, err, domain.ErrInsufficientFunds)
}

func TestService_Authorize_CardExpired(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeErr: domain.ErrCardExpired,
	}
	svc := service.New(repo, b)

	_, err := svc.Authorize(context.Background(), "idem-key-003", newAuthorizeRequest())
	assert.ErrorIs(t, err, domain.ErrCardExpired)
}

func TestService_FullLifecycle(t *testing.T) {
	repo := newMockRepo()
	now := time.Now()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
			Amount:          5000,
		},
		captureResp: &bank.CaptureResponse{
			CaptureID:       "bank-capture-001",
			AuthorizationID: "bank-auth-001",
			Status:          "captured",
			Amount:          5000,
		},
		refundResp: &bank.RefundResponse{
			RefundID:  "bank-refund-001",
			CaptureID: "bank-capture-001",
			Status:    "refunded",
			Amount:    5000,
		},
	}
	svc := service.New(repo, b)
	ctx := context.Background()

	// Authorize
	authResp, err := svc.Authorize(ctx, "key-auth", newAuthorizeRequest())
	require.NoError(t, err)
	assert.Equal(t, domain.StateAuthorized, authResp.State)
	assert.True(t, authResp.AuthorizedAt.After(now))

	// Capture
	capResp, err := svc.Capture(ctx, "key-cap", authResp.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, domain.StateCaptured, capResp.State)
	assert.True(t, capResp.CapturedAt.After(now))

	// Refund
	refResp, err := svc.Refund(ctx, "key-ref", authResp.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, domain.StateRefunded, refResp.State)
	assert.True(t, refResp.RefundedAt.After(now))
}

func TestService_Void_AfterAuthorize(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
			Amount:          5000,
		},
		voidResp: &bank.VoidResponse{
			VoidID:          "bank-void-001",
			AuthorizationID: "bank-auth-001",
			Status:          "voided",
		},
	}
	svc := service.New(repo, b)
	ctx := context.Background()

	authResp, err := svc.Authorize(ctx, "key-auth", newAuthorizeRequest())
	require.NoError(t, err)

	voidResp, err := svc.Void(ctx, "key-void", authResp.PaymentID)
	require.NoError(t, err)
	assert.Equal(t, domain.StateVoided, voidResp.State)
}

func TestService_Capture_InvalidState(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
		},
		voidResp: &bank.VoidResponse{
			VoidID: "bank-void-001",
			Status: "voided",
		},
	}
	svc := service.New(repo, b)
	ctx := context.Background()

	// Authorize then void
	authResp, err := svc.Authorize(ctx, "key-auth", newAuthorizeRequest())
	require.NoError(t, err)

	_, err = svc.Void(ctx, "key-void", authResp.PaymentID)
	require.NoError(t, err)

	// Try to capture a voided payment
	_, err = svc.Capture(ctx, "key-cap", authResp.PaymentID)
	assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
}

func TestService_GetByOrderID(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{
		authorizeResp: &bank.AuthorizeResponse{
			AuthorizationID: "bank-auth-001",
			Status:          "authorized",
			Amount:          5000,
		},
	}
	svc := service.New(repo, b)
	ctx := context.Background()

	_, err := svc.Authorize(ctx, "key-auth", newAuthorizeRequest())
	require.NoError(t, err)

	resp, err := svc.GetByOrderID(ctx, "order-001")
	require.NoError(t, err)
	assert.Equal(t, "order-001", resp.OrderID)
	assert.Equal(t, domain.StateAuthorized, resp.State)
}

func TestService_GetByOrderID_NotFound(t *testing.T) {
	repo := newMockRepo()
	b := &mockBank{}
	svc := service.New(repo, b)

	_, err := svc.GetByOrderID(context.Background(), "nonexistent-order")
	assert.ErrorIs(t, err, domain.ErrPaymentNotFound)
}
