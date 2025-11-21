package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout sets the timeout for the request.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if timeout <= 0 || timeout > 300*time.Second {
				timeout = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)
			done := make(chan struct{})

			go func() {
				next.ServeHTTP(w, r)
				close(done)
			}()

			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					http.Error(w, http.StatusText(http.StatusGatewayTimeout), http.StatusGatewayTimeout)
					return
				}
			case <-done:
			}
		})
	}
}
