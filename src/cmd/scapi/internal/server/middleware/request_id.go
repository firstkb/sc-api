package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKeyRequestID string

// RequestIDKey is the key for the request-id in the context.
const RequestIDKey ctxKeyRequestID = "request_id"

// RequestID sets the request-id in the context and the response header.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.NewString()
			}

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			w.Header().Set("X-Request-ID", reqID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
