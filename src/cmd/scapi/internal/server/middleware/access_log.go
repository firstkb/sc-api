package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog пишет простой access-log с использованием slog-логгера.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			// Оборачиваем ResponseWriter, чтобы получить статус-код.
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
