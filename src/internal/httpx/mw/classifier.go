package mw

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/internal/httpx/router"
)

func Classifier(c *router.Classifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m, ok := c.Match(r); ok {
				ctx := context.WithValue(r.Context(), router.CtxKeyTier, m.Tier)
				ctx = context.WithValue(ctx, router.CtxKeyRouteID, m.RouteID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			slog.Warn("route not matched", "method", r.Method, "path", r.URL.Path)
			http.NotFound(w, r)
		})
	}
}
