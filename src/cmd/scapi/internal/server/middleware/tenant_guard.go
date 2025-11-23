package middleware

import (
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/tenantsvc"
	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

// TenantGuard проверяет, что tenant_id соответствует Origin.
func TenantGuard(logger *slog.Logger, tenants *tenantsvc.ServiceTenantProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			routeID, tier, err := utils.GetRouteInfo(r.Context())
			if err != nil {
				logger.Error("TENANT_GUARD: route info missing", "error", err)
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if tier == router.TierSecure || tier == router.TierPublicTenant {
				ctx, err := tenants.Validate(r.Context(), tier, routeID, r)
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
