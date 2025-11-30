package auth

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ClaimResolver extracts claims from JWT tokens
type ClaimResolver interface {
	ExtractTenantID(claims jwt.MapClaims) (int64, error)
	ExtractUserID(claims jwt.MapClaims) (uuid.UUID, error)
	ExtractEmail(claims jwt.MapClaims) (string, error)
	ExtractLevel(claims jwt.MapClaims) (int, error)
	ExtractRoles(claims jwt.MapClaims) ([]string, error)
	Validate(claims jwt.MapClaims) error
}

// InternalClaimResolver extracts claims from internal JWT tokens
type InternalClaimResolver struct{}

func NewInternalClaimResolver() ClaimResolver {
	return &InternalClaimResolver{}
}

func (r *InternalClaimResolver) ExtractTenantID(claims jwt.MapClaims) (int64, error) {
	// Extract "ten" (UUID string) and convert to BIGINT
	ten, ok := claims["ten"].(string)
	if !ok {
		return 0, errors.New("ten claim missing or invalid")
	}

	// For now, we'll parse as int64 directly (if it's a string representation)
	// TODO: Implement proper UUID → BIGINT mapping via tenant table
	tenantID, err := strconv.ParseInt(ten, 10, 64)
	if err != nil {
		// If it's a UUID, we need to map it via database
		// For now, return error
		return 0, fmt.Errorf("tenant ID format not supported: %s", ten)
	}

	return tenantID, nil
}

func (r *InternalClaimResolver) ExtractUserID(claims jwt.MapClaims) (uuid.UUID, error) {
	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, errors.New("sub claim missing or invalid")
	}
	return uuid.Parse(sub)
}

func (r *InternalClaimResolver) ExtractEmail(claims jwt.MapClaims) (string, error) {
	eml, ok := claims["eml"].(string)
	if !ok || eml == "" {
		return "", errors.New("eml claim missing or invalid")
	}
	return eml, nil
}

func (r *InternalClaimResolver) ExtractLevel(claims jwt.MapClaims) (int, error) {
	lvl, ok := claims["lvl"].(float64) // JSON numbers are float64
	if !ok {
		return 0, errors.New("lvl claim missing or invalid")
	}
	return int(lvl), nil
}

func (r *InternalClaimResolver) ExtractRoles(claims jwt.MapClaims) ([]string, error) {
	roles, ok := claims["roles"].([]interface{})
	if !ok {
		return nil, nil // roles are optional
	}

	result := make([]string, 0, len(roles))
	for _, r := range roles {
		if role, ok := r.(string); ok {
			result = append(result, role)
		}
	}
	return result, nil
}

func (r *InternalClaimResolver) Validate(claims jwt.MapClaims) error {
	if _, err := r.ExtractTenantID(claims); err != nil {
		return fmt.Errorf("tenant_id: %w", err)
	}
	if _, err := r.ExtractUserID(claims); err != nil {
		return fmt.Errorf("user_id: %w", err)
	}
	if _, err := r.ExtractEmail(claims); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

// CognitoClaimResolver extracts claims from Cognito JWT tokens
type CognitoClaimResolver struct{}

func NewCognitoClaimResolver() ClaimResolver {
	return &CognitoClaimResolver{}
}

func (r *CognitoClaimResolver) ExtractTenantID(claims jwt.MapClaims) (int64, error) {
	// Try custom:tenant_id first
	if customTenantID, ok := claims["custom:tenant_id"].(float64); ok {
		return int64(customTenantID), nil
	}

	// Try username format "tenant_id|email"
	if username, ok := claims["username"].(string); ok {
		parts := strings.Split(username, "|")
		if len(parts) >= 1 {
			tenantID, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil {
				return tenantID, nil
			}
		}
	}

	return 0, errors.New("tenant_id not found in cognito claims")
}

func (r *CognitoClaimResolver) ExtractUserID(claims jwt.MapClaims) (uuid.UUID, error) {
	// Try custom:user_id
	if customUserID, ok := claims["custom:user_id"].(string); ok {
		return uuid.Parse(customUserID)
	}

	// Fallback: try sub (if it's a UUID)
	if sub, ok := claims["sub"].(string); ok {
		if u, err := uuid.Parse(sub); err == nil {
			return u, nil
		}
	}

	return uuid.Nil, errors.New("user_id not found in cognito claims")
}

func (r *CognitoClaimResolver) ExtractEmail(claims jwt.MapClaims) (string, error) {
	// Try email claim
	if email, ok := claims["email"].(string); ok && email != "" {
		return email, nil
	}

	// Try username format "tenant_id|email"
	if username, ok := claims["username"].(string); ok {
		parts := strings.Split(username, "|")
		if len(parts) >= 2 {
			return parts[1], nil
		}
	}

	return "", errors.New("email not found in cognito claims")
}

func (r *CognitoClaimResolver) ExtractLevel(claims jwt.MapClaims) (int, error) {
	// Try custom:level
	if customLevel, ok := claims["custom:level"].(float64); ok {
		return int(customLevel), nil
	}

	// Fallback: default user level
	return int(LevelUser), nil
}

func (r *CognitoClaimResolver) ExtractRoles(claims jwt.MapClaims) ([]string, error) {
	// Try custom:roles
	if customRoles, ok := claims["custom:roles"].([]interface{}); ok {
		result := make([]string, 0, len(customRoles))
		for _, r := range customRoles {
			if role, ok := r.(string); ok {
				result = append(result, role)
			}
		}
		return result, nil
	}

	// Try cognito:groups
	if groups, ok := claims["cognito:groups"].([]interface{}); ok {
		result := make([]string, 0, len(groups))
		for _, g := range groups {
			if group, ok := g.(string); ok {
				result = append(result, group)
			}
		}
		return result, nil
	}

	return nil, nil
}

func (r *CognitoClaimResolver) Validate(claims jwt.MapClaims) error {
	if _, err := r.ExtractTenantID(claims); err != nil {
		return fmt.Errorf("tenant_id: %w", err)
	}
	if _, err := r.ExtractEmail(claims); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

// LegacyClaimResolver extracts claims from legacy JWT tokens (backward compatibility)
type LegacyClaimResolver struct{}

func NewLegacyClaimResolver() ClaimResolver {
	return &LegacyClaimResolver{}
}

func (r *LegacyClaimResolver) ExtractTenantID(claims jwt.MapClaims) (int64, error) {
	// Try tenant_id (int64)
	if tenantID, ok := claims["tenant_id"].(float64); ok {
		return int64(tenantID), nil
	}

	// Try username format "tenant_id|email"
	if username, ok := claims["username"].(string); ok {
		parts := strings.Split(username, "|")
		if len(parts) >= 1 {
			tenantID, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil {
				return tenantID, nil
			}
		}
	}

	return 0, errors.New("tenant_id not found in legacy claims")
}

func (r *LegacyClaimResolver) ExtractUserID(claims jwt.MapClaims) (uuid.UUID, error) {
	// Legacy format may not have user_id
	// Return uuid.Nil (will be handled by caller)
	return uuid.Nil, nil
}

func (r *LegacyClaimResolver) ExtractEmail(claims jwt.MapClaims) (string, error) {
	// Try email
	if email, ok := claims["email"].(string); ok && email != "" {
		return email, nil
	}

	// Try username
	if username, ok := claims["username"].(string); ok {
		parts := strings.Split(username, "|")
		if len(parts) >= 2 {
			return parts[1], nil
		}
	}

	return "", errors.New("email not found in legacy claims")
}

func (r *LegacyClaimResolver) ExtractLevel(claims jwt.MapClaims) (int, error) {
	// Legacy format may not have level
	return int(LevelUser), nil // default user level
}

func (r *LegacyClaimResolver) ExtractRoles(claims jwt.MapClaims) ([]string, error) {
	// Legacy format may not have roles
	return nil, nil
}

func (r *LegacyClaimResolver) Validate(claims jwt.MapClaims) error {
	if _, err := r.ExtractTenantID(claims); err != nil {
		return fmt.Errorf("tenant_id: %w", err)
	}
	if _, err := r.ExtractEmail(claims); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	return nil
}

// NewClaimResolver creates a claim resolver based on provider
func NewClaimResolver(provider string) (ClaimResolver, error) {
	switch provider {
	case "internal":
		return NewInternalClaimResolver(), nil
	case "cognito":
		return NewCognitoClaimResolver(), nil
	case "legacy", "auto":
		return NewLegacyClaimResolver(), nil
	default:
		return nil, fmt.Errorf("unknown auth provider: %s", provider)
	}
}
