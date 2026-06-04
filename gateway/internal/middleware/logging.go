package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logging middleware logs every request with method, path, status code and how long it took
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"idempotency_key", r.Header.Get("Idempotency-Key"),
		)
	})
}
