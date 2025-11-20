package utils

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt"
)

type Claim struct {
	TenantID   string
	Email      string
	Claims     jwt.MapClaims
	ServerName string
}

type contextKey string

const ClaimKey contextKey = "claim"

// GetClaim return Claim from Context
func GetClaim(ctx context.Context) (*Claim, error) {
	claim, ok := ctx.Value(ClaimKey).(*Claim)
	if !ok {
		return nil, errors.New("no claim in context")
	}
	return claim, nil
}

// SetClaim parce token and set Claim to Context
func CreateContextWithClaim(r *http.Request) (context.Context, error) {
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

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return nil, errors.New("username not found in token")
	}

	tenantID, email, err := parseUsername(username)
	if err != nil {
		return nil, err
	}

	originVal := r.Header.Get("Origin")
	var serverName string
	if originVal == "" {
		serverName = "PWA"
	} else {
		parsed, err := url.Parse(originVal)
		if err == nil && parsed.Host != "" {
			serverName = parsed.Host
		} else {
			serverName = strings.TrimPrefix(originVal, "http://")
			serverName = strings.TrimPrefix(serverName, "https://")
			serverName = strings.TrimSuffix(serverName, "/")
		}
	}

	claim := &Claim{
		TenantID:   tenantID,
		Email:      email,
		Claims:     claims,
		ServerName: serverName,
	}

	// set Claim to Context
	ctx := context.WithValue(r.Context(), ClaimKey, claim)
	return ctx, nil
}

// parseUsername Split tenant_id and email
func parseUsername(username string) (string, string, error) {
	parts := strings.Split(username, "|")
	if len(parts) != 2 {
		return "", "", errors.New("invalid username format")
	}
	return parts[0], parts[1], nil
}
