package server

import (
	"net/http"
	"strings"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	Debug            bool
}

func (srv *Server) addMiddleware(handler http.Handler, origin string) http.Handler {

	var DefaultCORSConfig = CORSConfig{
		AllowedOrigins:   []string{origin},
		AllowedMethods:   []string{http.MethodGet, http.MethodPut},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		Debug:            false,
	}

	handler = srv.corsMiddleware(handler, DefaultCORSConfig)
	//handler = srv.loggingMiddleware(handler)
	handler = srv.claimsMiddleware(handler)
	return handler
}

func (srv *Server) corsMiddleware(next http.Handler, config CORSConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/status" {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin, config.AllowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			if config.Debug {
				srv.logger.Debug("CORS: Invalid Origin", "origin", origin)
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

func (srv *Server) claimsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path == "/status" {
			next.ServeHTTP(w, r)
			return
		}

		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		ctx, err := utils.CreateContextWithClaim(r)
		if err != nil {
			srv.logger.Error("Parse claim error", "error", err)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*func (srv *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			srv.logger.Debug("Header", "data", r.Header)
			srv.logger.Debug("Request", "method", r.Method, "url", r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}*/

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

/*func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}*/
