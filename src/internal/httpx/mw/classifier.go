package mw

import (
	"context"
	"net/http"
	"strings"

	"github.com/firstkb/sc-api/internal/httpx/router"
)

func Classifier(c *router.Classifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m, ok := c.Match(r); ok {
				ctx := context.WithValue(r.Context(), router.CtxKeyTier, m.Tier)
				ctx = context.WithValue(ctx, router.CtxKeyRouteID, m.RouteID)
				ctx = context.WithValue(ctx, router.CtxKeyDomain, router.ExtractDomain(r))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			//slog.Warn("route not matched", "method", r.Method, "path", r.URL.Path)

			if methods, ok := c.AllowedMethods(r.URL.Path); ok {
				w.Header().Set("Allow", strings.Join(methods, ", "))
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}

			http.NotFound(w, r)
		})
	}
}
