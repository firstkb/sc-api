package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"

	"github.com/firstkb/sc-api/internal/httpx/claims"
)

type Claim struct {
	TenantID string
	UserID   string
	Email    string
	Claims   jwt.MapClaims
}

const (
	tokenProviderInternal = "internal"
	tokenProviderCognito  = "cognito"
)

// GetClaim returns Claim from Context by extracting value from httpx/claims slot.
func GetClaim(ctx context.Context) (*Claim, error) {
	if ctx == nil {
		return nil, errors.New("no claim in context")
	}

	if raw := claims.FromContext(ctx); raw != nil {
		if c, ok := raw.(*Claim); ok && c != nil {
			return c, nil
		}
	}

	return nil, errors.New("no claim in context")
}

// CreateContextWithClaim parses the token and returns a new Context with Claim,
// saving it in the universal httpx/claims slot.
func CreateContextWithClaim(r *http.Request, provider string) (context.Context, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("authorization header missing")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		return nil, errors.New("authorization header must be formatted as 'Bearer token'")
	}

	if parts[0] != "Bearer" {
		return nil, errors.New("authorization header must start with Bearer")
	}

	tokenString := parts[1]
	if tokenString == "" {
		return nil, errors.New("token cannot be empty")
	}

	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, errors.New("invalid token")
	}

	jwtClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	claim, err := buildClaim(jwtClaims, r, provider)
	if err != nil {
		return nil, err
	}

	ctx := claims.With(r.Context(), claim)
	return ctx, nil
}

// parseUsername splits tenant_id and email (format tenant|email).
func parseUsername(username string) (string, string, error) {
	parts := strings.Split(username, "|")
	if len(parts) != 2 {
		return "", "", errors.New("invalid username format")
	}
	return parts[0], parts[1], nil
}

func buildClaim(jwtClaims jwt.MapClaims, r *http.Request, provider string) (*Claim, error) {
	var (
		tenantID string
		email    string
		userID   string
		err      error
	)

	switch normalizeProvider(provider) {
	case tokenProviderCognito:
		tenantID, email, userID, err = parseCognitoClaims(jwtClaims)
	default:
		tenantID, email, userID, err = parseInternalClaims(jwtClaims)
	}
	if err != nil {
		return nil, err
	}

	return &Claim{
		TenantID: tenantID,
		UserID:   userID,
		Email:    email,
		Claims:   jwtClaims,
	}, nil
}

func parseInternalClaims(jwtClaims jwt.MapClaims) (string, string, string, error) {
	tenantID, err := mustStringClaim(jwtClaims, "tenant_id")
	if err != nil {
		return "", "", "", err
	}
	email, err := mustStringClaim(jwtClaims, "email")
	if err != nil {
		return "", "", "", err
	}
	userID, err := mustStringClaim(jwtClaims, "user_id")
	if err != nil {
		return "", "", "", err
	}

	return tenantID, email, userID, nil
}

func parseCognitoClaims(jwtClaims jwt.MapClaims) (string, string, string, error) {
	username := stringClaim(jwtClaims, "username")
	tenantID := stringClaim(jwtClaims, "custom:tenant_id")
	email := stringClaim(jwtClaims, "email")
	userID := stringClaim(jwtClaims, "custom:user_id")
	if userID == "" {
		userID = stringClaim(jwtClaims, "sub")
	}

	if tenantID == "" || email == "" {
		var err error
		tenantID, email, err = parseUsername(username)
		if err != nil {
			return "", "", "", err
		}
	}

	if tenantID == "" || email == "" {
		return "", "", "", errors.New("insufficient claims for cognito token")
	}

	if userID == "" {
		userID = "undefined"
	}

	return tenantID, email, userID, nil
}

func mustStringClaim(m jwt.MapClaims, key string) (string, error) {
	value := stringClaim(m, key)
	if value == "" {
		return "", fmt.Errorf("claim %s not found in token", key)
	}
	return value, nil
}

func stringClaim(m jwt.MapClaims, key string) string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case fmt.Stringer:
			return val.String()
		}
	}
	return ""
}

func normalizeProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case tokenProviderCognito:
		return tokenProviderCognito
	default:
		return tokenProviderInternal
	}
}
