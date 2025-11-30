package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTIssuer handles JWT token generation and validation
type JWTIssuer interface {
	IssueToken(claims *JWTClaims) (string, error)
	ValidateToken(tokenString string) (*JWTClaims, error)
	GetPublicKey() (*rsa.PublicKey, error)
}

type jwtIssuer struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	audience   string
	algorithm  string
}

// Config holds JWT issuer configuration
type JWTConfig struct {
	Algorithm      string // "rs256" or "eddsa"
	PrivateKeyPath string
	PublicKeyPath  string
	Issuer         string
	Audience       string
	AccessTTL      time.Duration
}

// NewJWTIssuer creates a new JWT issuer
func NewJWTIssuer(config JWTConfig) (JWTIssuer, error) {
	// For now, we'll support RS256 only
	if config.Algorithm != "rs256" {
		return nil, fmt.Errorf("unsupported algorithm: %s (only rs256 supported for now)", config.Algorithm)
	}

	// Load private key
	privateKey, err := loadPrivateKey(config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	// Load or derive public key
	var publicKey *rsa.PublicKey
	if config.PublicKeyPath != "" {
		publicKey, err = loadPublicKey(config.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load public key: %w", err)
		}
	} else {
		publicKey = &privateKey.PublicKey
	}

	return &jwtIssuer{
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     config.Issuer,
		audience:   config.Audience,
		algorithm:  config.Algorithm,
	}, nil
}

func (j *jwtIssuer) IssueToken(claims *JWTClaims) (string, error) {
	// Set issuer and audience if not set
	if claims.Issuer == "" {
		claims.Issuer = j.issuer
	}
	if claims.Audience == "" {
		claims.Audience = j.audience
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims.ToMap()))

	// Sign token
	tokenString, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

func (j *jwtIssuer) ValidateToken(tokenString string) (*JWTClaims, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify algorithm
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Convert to JWTClaims
	jwtClaims := &JWTClaims{
		Issuer:    getStringClaim(claims, "iss"),
		Audience:  getStringClaim(claims, "aud"),
		IssuedAt:  getInt64Claim(claims, "iat"),
		ExpiresAt: getInt64Claim(claims, "exp"),
		JWTID:     getStringClaim(claims, "jti"),
		Subject:   getStringClaim(claims, "sub"),
		Tenant:    getStringClaim(claims, "ten"),
		Email:     getStringClaim(claims, "eml"),
		Phone:     getStringClaim(claims, "phn"),
		Level:     getIntClaim(claims, "lvl"),
		Role:      getStringClaim(claims, "role"),
	}

	return jwtClaims, nil
}

func (j *jwtIssuer) GetPublicKey() (*rsa.PublicKey, error) {
	return j.publicKey, nil
}

// Helper functions for claim extraction
func getStringClaim(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

func getInt64Claim(claims jwt.MapClaims, key string) int64 {
	if v, ok := claims[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func getIntClaim(claims jwt.MapClaims, key string) int {
	if v, ok := claims[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getStringSliceClaim(claims jwt.MapClaims, key string) []string {
	if v, ok := claims[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// loadPrivateKey loads an RSA private key from a PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return rsaKey, nil
	}

	return privateKey, nil
}

// loadPublicKey loads an RSA public key from a PEM file
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// GenerateKeyPair generates a new RSA key pair (for development/testing)
func GenerateKeyPair(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}
