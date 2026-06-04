package service

import (
	"context"
	"encoding/json"
	"fmt"
	"payment-gateway/internal/domain"
	"time"
)

func (s *PaymentService) saveIdempotencyRecord(ctx context.Context, key, operation, paymentID string, resp *PaymentResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return s.repo.SaveIdempotencyRecord(ctx, &domain.IdempotencyRecord{
		Key:       key,
		Operation: operation,
		PaymentID: paymentID,
		Response:  data,
		CreatedAt: time.Now(),
	})
}

func toPaymentResponse(p *domain.Payment) *PaymentResponse {
	return &PaymentResponse{
		PaymentID:    p.ID,
		OrderID:      p.OrderID,
		CustomerID:   p.CustomerID,
		Amount:       p.Amount,
		Currency:     p.Currency,
		State:        p.State,
		BankAuthID:   p.BankAuthID,
		CreatedAt:    p.CreatedAt,
		AuthorizedAt: p.AuthorizedAt,
		CapturedAt:   p.CapturedAt,
		VoidedAt:     p.VoidedAt,
		RefundedAt:   p.RefundedAt,
	}
}

func deserializeResponse(data []byte) (*PaymentResponse, error) {
	var resp PaymentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("deserialize response: %w", err)
	}
	return &resp, nil
}
