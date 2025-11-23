package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/tenantsvc"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

// TenantGuard проверяет, что tenant_id соответствует Origin.
func TenantGuard(logger *slog.Logger, tenants *tenantsvc.ServiceTenantProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			routeInfo, ok := requestctx.Route(r.Context())
			if !ok || routeInfo.Domain == "" || routeInfo.Tier == "" {
				logger.Error("TENANT_GUARD: route info missing", "error", errors.New("route info not found in context"))
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if routeInfo.Tier == router.TierSecure || routeInfo.Tier == router.TierPublicTenant {
				ctx, err := tenants.Validate(r.Context(), routeInfo.Tier, routeInfo.ID, routeInfo.Domain, r)
				if err != nil {
					logger.Error("TENANT_GUARD: tenant validation failed", "error", err)
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
