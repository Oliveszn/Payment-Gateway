package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"payment-gateway/internal/service"
)

type Handler struct {
	svc *service.PaymentService
}

func New(svc *service.PaymentService) *Handler {
	return &Handler{svc: svc}
}

type authorizeRequest struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	CardNumber string `json:"card_number"`
	CardExpiry string `json:"card_expiry"`
	CardCVV    string `json:"card_cvv"`
}

// Authorize handles POST /payments/authorize
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var req authorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateAuthorizeRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Authorize(r.Context(), idempotencyKey, service.AuthorizeRequest{
		OrderID:    req.OrderID,
		CustomerID: req.CustomerID,
		Amount:     req.Amount,
		Currency:   req.Currency,
		CardNumber: req.CardNumber,
		CardExpiry: req.CardExpiry,
		CardCVV:    req.CardCVV,
	})
	if err != nil {
		slog.Error("authorize failed", "error", err)
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// Capture handles POST /payments/{id}/capture
func (h *Handler) Capture(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	paymentID := r.PathValue("id")
	if paymentID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
		return
	}

	resp, err := h.svc.Capture(r.Context(), idempotencyKey, paymentID)
	if err != nil {
		slog.Error("capture failed", "error", err)
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Void handles POST /payments/{id}/void
func (h *Handler) Void(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	paymentID := r.PathValue("id")
	if paymentID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
		return
	}

	resp, err := h.svc.Void(r.Context(), idempotencyKey, paymentID)
	if err != nil {
		slog.Error("void failed", "error", err)
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Refund handles POST /payments/{id}/refund
func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	paymentID := r.PathValue("id")
	if paymentID == "" {
		writeError(w, http.StatusBadRequest, "payment id is required")
		return
	}

	resp, err := h.svc.Refund(r.Context(), idempotencyKey, paymentID)
	if err != nil {
		slog.Error("refund failed", "error", err)
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetByOrderID handles GET /payments?order_id=xxx
func (h *Handler) GetByOrderID(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	customerID := r.URL.Query().Get("customer_id")

	if orderID == "" && customerID == "" {
		writeError(w, http.StatusBadRequest, "order_id or customer_id query param is required")
		return
	}

	if orderID != "" {
		resp, err := h.svc.GetByOrderID(r.Context(), orderID)
		if err != nil {
			slog.Error("get by order id failed", "error", err)
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp, err := h.svc.GetByCustomerID(r.Context(), customerID)
	if err != nil {
		slog.Error("get by customer id failed", "error", err)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
