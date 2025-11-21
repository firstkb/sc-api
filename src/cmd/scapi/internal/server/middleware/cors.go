package middleware

import (
	"net/http"
	"strings"

	"log/slog"

	"github.com/firstkb/sc-api/internal/httpx/router"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	Debug            bool
}

func CORS(logger *slog.Logger, config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			tier, ok := r.Context().Value(router.CtxKeyTier).(router.Tier)
			if !ok {
				logger.Error("CORS: Tier not found", "path", r.URL.Path)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if tier == router.TierHealth {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodOptions {
				origin := r.Header.Get("Origin")
				if isAllowedOrigin(origin, config.AllowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					if config.Debug {
						logger.Debug("CORS: Invalid Origin", "origin", origin)
					}
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			if isAllowedOrigin(origin, config.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				if config.Debug {
					logger.Debug("CORS: Invalid Origin", "origin", origin)
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")

			if strings.HasSuffix(origin, domain) && isSubdomain(origin, domain) {
				return true
			}
		} else if origin == allowedOrigin {
			return true
		}
	}
	return false
}

func isSubdomain(origin, domain string) bool {
	if strings.HasSuffix(origin, "."+domain) || origin == domain {
		return true
	}
	return false
}
