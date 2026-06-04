package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"payment-gateway/internal/domain"
	"payment-gateway/internal/repository"
	"time"
)

// responseRecorder captures the response so we can store it in the idempotency table after the handler runs
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Idempotency checks if a request has been processed before.
func Idempotency(repo repository.PaymentRepository, operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")

			// No key skip idempotency check, just pass through
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if weve seen this key before
			existing, err := repo.GetIdempotencyRecord(r.Context(), key, operation)
			if err != nil {
				writeMiddlewareError(w, http.StatusInternalServerError, "failed to check idempotency key")
				return
			}

			// We have a stored response return it directly
			if existing != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Replayed", "true")
				w.WriteHeader(http.StatusOK)
				w.Write(existing.Response)
				return
			}

			// New request record the response after handler runs
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Only store successful responses, here we wont store 4xx/5xx just let the client retry with same key
			if rec.status >= 200 && rec.status < 300 {
				record := &domain.IdempotencyRecord{
					Key:       key,
					Operation: operation,
					Response:  rec.body.Bytes(),
					CreatedAt: time.Now(),
				}

				// Extract payment_id from response if present
				var respBody map[string]interface{}
				if err := json.Unmarshal(rec.body.Bytes(), &respBody); err == nil {
					if id, ok := respBody["payment_id"].(string); ok {
						record.PaymentID = id
					}
				}

				_ = repo.SaveIdempotencyRecord(r.Context(), record)
			}
		})
	}
}

func writeMiddlewareError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
