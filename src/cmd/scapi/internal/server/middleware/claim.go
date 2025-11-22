package middleware

import (
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

// Claims parses the JWT from the Authorization header and puts the claims in the context.
func Claims(logger *slog.Logger, provider string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			_, tier, err := utils.GetRouteInfo(r.Context())
			if err != nil {
				logger.Error("CLAIMS: Route info not found", "path", r.URL.Path, "error", err)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			} else if tier != router.TierSecure {
				next.ServeHTTP(w, r)
				return
			}

			ctx, err := utils.CreateContextWithClaim(r, provider)
			if err != nil {
				logger.Error("Parse claim error", "error", err)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
