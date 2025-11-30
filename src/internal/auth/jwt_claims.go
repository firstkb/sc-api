package auth

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	// Standard claims
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	JWTID     string `json:"jti"`

	// Custom claims
	Subject string `json:"sub"`             // user_id (UUID)
	Tenant  string `json:"tenant_id"`       // tenant_id (numeric string)
	Email   string `json:"email"`           // email
	Phone   string `json:"phone,omitempty"` // phone (E.164, optional)
	Level   int    `json:"level"`           // access level
	Role    string `json:"role,omitempty"`  // role (optional)
}

// ToMap converts JWTClaims to a map for JWT library
func (c *JWTClaims) ToMap() map[string]interface{} {
	claims := map[string]interface{}{
		"iss":       c.Issuer,
		"aud":       c.Audience,
		"iat":       c.IssuedAt,
		"exp":       c.ExpiresAt,
		"jti":       c.JWTID,
		"sub":       c.Subject,
		"tenant_id": c.Tenant,
		"email":     c.Email,
		"level":     c.Level,
	}

	if c.Phone != "" {
		claims["phone"] = c.Phone
	}

	if c.Role != "" {
		claims["role"] = c.Role
	}

	return claims
}

// NewJWTClaims creates new JWT claims for a user
func NewJWTClaims(issuer, audience string, userID uuid.UUID, tenantID int64, email, phone string, level AccessLevel, role string) *JWTClaims {
	now := time.Now()

	return &JWTClaims{
		Issuer:    issuer,
		Audience:  audience,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(15 * time.Minute).Unix(), // Default 15 minutes
		JWTID:     uuid.New().String(),
		Subject:   userID.String(),
		Tenant:    fmt.Sprintf("%d", tenantID),
		Email:     email,
		Phone:     phone,
		Level:     int(level),
		Role:      role,
	}
}
