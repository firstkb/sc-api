package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog writes a simple access-log using the slog logger.
func AccessLog(logger *slog.Logger, enabled bool) func(http.Handler) http.Handler {
	if !enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// Wrap ResponseWriter to get the status code.
			ww := &responseWriterWrapper{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
