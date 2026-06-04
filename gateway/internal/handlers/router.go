package handlers

import (
	"net/http"
	"payment-gateway/internal/middleware"
	"payment-gateway/internal/repository"
)

func NewRouter(h *Handler, repo repository.PaymentRepository) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", h.Health)

	mux.Handle("POST /payments/authorize",
		middleware.Idempotency(repo, "authorize")(http.HandlerFunc(h.Authorize)))

	mux.Handle("POST /payments/{id}/capture",
		middleware.Idempotency(repo, "capture")(http.HandlerFunc(h.Capture)))

	mux.Handle("POST /payments/{id}/void",
		middleware.Idempotency(repo, "void")(http.HandlerFunc(h.Void)))

	mux.Handle("POST /payments/{id}/refund",
		middleware.Idempotency(repo, "refund")(http.HandlerFunc(h.Refund)))

	mux.HandleFunc("GET /payments", h.GetByOrderID)

	// Wrap everything with logging and recovery
	return middleware.Recovery(middleware.Logging(mux))
}
