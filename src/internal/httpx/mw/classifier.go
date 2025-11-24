package mw

import (
	"net/http"
	"strings"

	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

func Classifier(c *router.Classifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if match, ok := c.Match(r); ok {
				ctx := requestctx.WithRoute(r.Context(), requestctx.RouteInfo{
					Tier:    match.Tier,
					ID:      match.RouteID,
					Pattern: match.Pattern,
					URI:     match.URI,
					Domain:  router.ExtractDomain(r),
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if methods, ok := c.AllowedMethods(r.URL.Path); ok {
				w.Header().Set("Allow", strings.Join(methods, ", "))
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}

			http.NotFound(w, r)
		})
	}
}
