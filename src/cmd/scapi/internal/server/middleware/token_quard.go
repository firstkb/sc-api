package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/firstkb/sc-api/cmd/scapi/internal/authsvc"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

// ValidatedClaims validates JWT tokens via authsvc and stores claims in context.
func ValidatedClaims(logger *slog.Logger, service *authsvc.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeInfo, ok := requestctx.Route(r.Context())
			if !ok {
				logger.Error("CONTEXT: route info missing", "error", errors.New("route info not found in context"))
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			if routeInfo.Tier != router.TierSecure {
				next.ServeHTTP(w, r)
				return
			}

			token, err := bearerToken(r)
			if err != nil {
				logger.Error("AUTHORIZATION: bearer token invalid", "error", err)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			_, err = service.ValidateToken(token)
			if err != nil {
				logger.Error("AUTHORIZATION: token validation failed", "error", err)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("authorization header must be 'Bearer <token>'")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("token cannot be empty")
	}
	return token, nil
}
