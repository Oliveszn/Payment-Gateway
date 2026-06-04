package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"payment-gateway/internal/domain"
)

func validateAuthorizeRequest(req authorizeRequest) error {
	if req.OrderID == "" {
		return errors.New("order_id is required")
	}
	if req.CustomerID == "" {
		return errors.New("customer_id is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if req.CardNumber == "" {
		return errors.New("card_number is required")
	}
	if req.CardExpiry == "" {
		return errors.New("card_expiry is required")
	}
	if req.CardCVV == "" {
		return errors.New("card_cvv is required")
	}
	return nil
}

// writeServiceError maps domain errors to HTTP status codes
func writeServiceError(w http.ResponseWriter, err error) {
	slog.Error("service error", "error", err, "type", fmt.Sprintf("%T", err))
	switch {
	case errors.Is(err, domain.ErrPaymentNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidStateTransition):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case errors.Is(err, domain.ErrInvalidCard):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrCardExpired):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrBankUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
