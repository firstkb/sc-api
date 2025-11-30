package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"

	"github.com/firstkb/sc-api/internal/httpx/claims"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
)

type Claim struct {
	TenantID string
	UserID   string
	Email    string
	Level    int
	Role     string
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

	if rc, ok := requestctx.Claims(ctx); ok {
		return &Claim{
			TenantID: rc.TenantID,
			UserID:   rc.UserID,
			Email:    rc.Email,
			Level:    int(rc.Level),
			Role:     string(rc.Role),
		}, nil
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

	ctx := requestctx.WithClaims(r.Context(), requestctx.ClaimsInfo{
		TenantID: claim.TenantID,
		UserID:   claim.UserID,
		Email:    claim.Email,
		Level:    claim.Level,
		Role:     claim.Role,
	})
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
		level    int
		role     string
		err      error
	)

	switch normalizeProvider(provider) {
	case tokenProviderCognito:
		tenantID, email, userID, err = parseCognitoClaims(jwtClaims)
		level = 40
		role = "user"
	default:
		tenantID, email, userID, level, role, err = parseInternalClaims(jwtClaims)
	}
	if err != nil {
		return nil, err
	}

	return &Claim{
		TenantID: tenantID,
		UserID:   userID,
		Email:    email,
		Level:    level,
		Role:     role,
		Claims:   jwtClaims,
	}, nil
}

func parseInternalClaims(jwtClaims jwt.MapClaims) (string, string, string, int, string, error) {
	tenantID, err := mustStringClaim(jwtClaims, "tenant_id")
	if err != nil {
		return "", "", "", 0, "", err
	}
	email, err := mustStringClaim(jwtClaims, "email")
	if err != nil {
		return "", "", "", 0, "", err
	}
	userID, err := mustStringClaim(jwtClaims, "sub")
	if err != nil {
		return "", "", "", 0, "", err
	}

	// todo: add parse "level":50,"roles":["admin"] or roles: "admin,user"
	level, err := mustIntClaim(jwtClaims, "level")
	if err != nil {
		return "", "", "", 0, "", err
	}
	role, err := mustStringClaim(jwtClaims, "role")
	if err != nil {
		return "", "", "", 0, "", err
	}

	return tenantID, email, userID, level, role, nil
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

func mustIntClaim(m jwt.MapClaims, key string) (int, error) {
	value := intClaim(m, key)
	if value == 0 {
		return 0, fmt.Errorf("claim %s not found in token", key)
	}
	return value, nil
}

func intClaim(m jwt.MapClaims, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
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
