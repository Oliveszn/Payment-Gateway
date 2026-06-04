package domain_test

import (
	"payment-gateway/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayment_Authorize(t *testing.T) {
	t.Run("can authorize a pending payment", func(t *testing.T) {
		p := newPendingPayment()
		err := p.Authorize("bank-auth-001")
		require.NoError(t, err)
		assert.Equal(t, domain.StateAuthorized, p.State)
		assert.Equal(t, "bank-auth-001", p.BankAuthID)
		assert.NotNil(t, p.AuthorizedAt)
	})

	t.Run("cannot authorize an already authorized payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		err := p.Authorize("bank-auth-002")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})
}

func TestPayment_Capture(t *testing.T) {
	t.Run("can capture an authorized payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		err := p.Capture("bank-capture-001")
		require.NoError(t, err)
		assert.Equal(t, domain.StateCaptured, p.State)
		assert.NotNil(t, p.CapturedAt)
	})

	t.Run("cannot capture a pending payment", func(t *testing.T) {
		p := newPendingPayment()
		err := p.Capture("bank-capture-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})

	t.Run("cannot capture a voided payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		_ = p.Void("bank-void-001")
		err := p.Capture("bank-capture-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})
}

func TestPayment_Void(t *testing.T) {
	t.Run("can void an authorized payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		err := p.Void("bank-void-001")
		require.NoError(t, err)
		assert.Equal(t, domain.StateVoided, p.State)
		assert.NotNil(t, p.VoidedAt)
	})

	t.Run("cannot void a pending payment", func(t *testing.T) {
		p := newPendingPayment()
		err := p.Void("bank-void-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})

	t.Run("cannot void a captured payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		_ = p.Capture("bank-capture-001")
		err := p.Void("bank-void-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})

	t.Run("cannot void a refunded payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		_ = p.Capture("bank-capture-001")
		_ = p.Refund("bank-refund-001")
		err := p.Void("bank-void-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})
}

func TestPayment_Refund(t *testing.T) {
	t.Run("can refund a captured payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		_ = p.Capture("bank-capture-001")
		err := p.Refund("bank-refund-001")
		require.NoError(t, err)
		assert.Equal(t, domain.StateRefunded, p.State)
		assert.NotNil(t, p.RefundedAt)
	})

	t.Run("cannot refund an authorized payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		err := p.Refund("bank-refund-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})

	t.Run("cannot refund a voided payment", func(t *testing.T) {
		p := newPendingPayment()
		_ = p.Authorize("bank-auth-001")
		_ = p.Void("bank-void-001")
		err := p.Refund("bank-refund-001")
		assert.ErrorIs(t, err, domain.ErrInvalidStateTransition)
	})
}

func TestPayment_FullLifecycle(t *testing.T) {
	t.Run("authorize -> capture -> refund", func(t *testing.T) {
		p := newPendingPayment()
		assert.Equal(t, domain.StatePending, p.State)

		require.NoError(t, p.Authorize("auth-001"))
		assert.Equal(t, domain.StateAuthorized, p.State)

		require.NoError(t, p.Capture("capture-001"))
		assert.Equal(t, domain.StateCaptured, p.State)

		require.NoError(t, p.Refund("refund-001"))
		assert.Equal(t, domain.StateRefunded, p.State)
	})

	t.Run("authorize -> void", func(t *testing.T) {
		p := newPendingPayment()
		assert.Equal(t, domain.StatePending, p.State)

		require.NoError(t, p.Authorize("auth-001"))
		assert.Equal(t, domain.StateAuthorized, p.State)

		require.NoError(t, p.Void("void-001"))
		assert.Equal(t, domain.StateVoided, p.State)
	})
}

// newPendingPayment creates a fresh payment for testing
func newPendingPayment() *domain.Payment {
	return &domain.Payment{
		ID:         "test-payment-001",
		OrderID:    "test-order-001",
		CustomerID: "test-customer-001",
		Amount:     5000,
		Currency:   "USD",
		State:      domain.StatePending,
		CardNumber: "4111111111111111",
		CardExpiry: "12/2030",
		CardCVV:    "123",
	}
}
