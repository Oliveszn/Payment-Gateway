package bank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"payment-gateway/internal/domain"
	"time"
)

// Client talks to the mock bank API
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	baseDelay  time.Duration
}

func New(baseURL string, maxRetries int, baseDelayMS int) *Client {
	return &Client{
		baseURL:    baseURL,
		maxRetries: maxRetries,
		baseDelay:  time.Duration(baseDelayMS) * time.Millisecond,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type AuthorizeRequest struct {
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"expiry_date"`
	CardCVV    string `json:"cvv"`
}

type AuthorizeResponse struct {
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
	Amount          int64  `json:"amount"`
}

type CaptureRequest struct {
	AuthorizationID string `json:"authorization_id"`
	Amount          int64  `json:"amount"`
}

type CaptureResponse struct {
	CaptureID       string `json:"capture_id"`
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
	Amount          int64  `json:"amount"`
}

type VoidRequest struct {
	AuthorizationID string `json:"authorization_id"`
}

type VoidResponse struct {
	VoidID          string `json:"void_id"`
	AuthorizationID string `json:"authorization_id"`
	Status          string `json:"status"`
}

type RefundRequest struct {
	CaptureID string `json:"capture_id"`
	Amount    int64  `json:"amount"`
}

type RefundResponse struct {
	RefundID  string `json:"refund_id"`
	CaptureID string `json:"capture_id"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
}

type bankErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (c *Client) Authorize(ctx context.Context, idempotencyKey string, req AuthorizeRequest) (*AuthorizeResponse, error) {
	var resp AuthorizeResponse
	err := c.doWithRetry(ctx, "POST", "/api/v1/authorizations", idempotencyKey, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Capture(ctx context.Context, idempotencyKey string, req CaptureRequest) (*CaptureResponse, error) {
	var resp CaptureResponse
	err := c.doWithRetry(ctx, "POST", "/api/v1/captures", idempotencyKey, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Void(ctx context.Context, idempotencyKey string, req VoidRequest) (*VoidResponse, error) {
	var resp VoidResponse
	err := c.doWithRetry(ctx, "POST", "/api/v1/voids", idempotencyKey, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Refund(ctx context.Context, idempotencyKey string, req RefundRequest) (*RefundResponse, error) {
	var resp RefundResponse
	err := c.doWithRetry(ctx, "POST", "/api/v1/refunds", idempotencyKey, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// our retry logic

// doWithRetry calls the bank and retries on transient failures with exponential backoff with jitter so retries don't all hit the bank at the same tim
func (c *Client) doWithRetry(ctx context.Context, method, path, idempotencyKey string, body, result interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.backoffDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			}
		}

		err := c.do(ctx, method, path, idempotencyKey, body, result)
		if err == nil {
			return nil
		}

		// Only retry transient errors
		if !isTransient(err) {
			return err
		}

		lastErr = err
	}

	return fmt.Errorf("bank unavailable after %d attempts: %w", c.maxRetries, lastErr)
}

// do performs a single HTTP request to the bank
func (c *Client) do(ctx context.Context, method, path, idempotencyKey string, body, result interface{}) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network error always transient
		return &TransientError{Cause: err}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TransientError{Cause: fmt.Errorf("read response: %w", err)}
	}

	// 2xx success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
		return nil
	}

	// Parse error response from bank
	var bankErr bankErrorResponse
	_ = json.Unmarshal(respBytes, &bankErr)
	msg := bankErr.Message
	if msg == "" {
		msg = bankErr.Error
	}

	// 500 transient, retry
	if resp.StatusCode >= 500 {
		return &TransientError{Cause: fmt.Errorf("bank error %d: %s", resp.StatusCode, msg)}
	}

	// 4xx permanent, map to domain errors
	return mapBankError(resp.StatusCode, msg)
}

// backoffDelay calculates exponential backoff with jitter
// attempt 1 100ms, attempt 2 200ms, attempt 3 400ms
// jitter adds randomness so retries don't all hit at once
func (c *Client) backoffDelay(attempt int) time.Duration {
	base := float64(c.baseDelay) * math.Pow(2, float64(attempt-1))
	jitter := rand.Float64() * float64(c.baseDelay)
	return time.Duration(base + jitter)
}

// Error types

// TransientError means the request failed but is safe to retry
type TransientError struct {
	Cause error
}

func (e *TransientError) Error() string {
	return fmt.Sprintf("transient error: %v", e.Cause)
}

// isTransient returns true if the error is worth retrying
func isTransient(err error) bool {
	var t *TransientError
	return err != nil && (fmt.Sprintf("%T", err) == "*bank.TransientError" ||
		func() bool {
			_, ok := err.(*TransientError)
			_ = t
			return ok
		}())
}

// mapBankError translates bank HTTP errors to domain errors
func mapBankError(statusCode int, message string) error {
	switch statusCode {
	case 402:
		return domain.ErrInsufficientFunds
	case 422:
		// Unprocessable could be expired card, invalid card, bad state
		if contains(message, "expired") {
			return domain.ErrCardExpired
		}
		if contains(message, "insufficient") {
			return domain.ErrInsufficientFunds
		}
		return domain.ErrInvalidCard
	case 404:
		return domain.ErrPaymentNotFound
	case 409:
		return domain.ErrInvalidStateTransition
	default:
		return fmt.Errorf("bank rejected request (%d): %s", statusCode, message)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				func() bool {
					for i := 0; i <= len(s)-len(substr); i++ {
						if s[i:i+len(substr)] == substr {
							return true
						}
					}
					return false
				}())
}
