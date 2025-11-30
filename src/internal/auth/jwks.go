package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	KTY string `json:"kty"` // Key type: "RSA"
	USE string `json:"use"` // Public key use: "sig"
	KID string `json:"kid"` // Key ID
	ALG string `json:"alg"` // Algorithm: "RS256"
	N   string `json:"n"`   // RSA modulus (base64url)
	E   string `json:"e"`   // RSA exponent (base64url)
}

// JWKSCache provides caching for JWKS
type JWKSCache interface {
	Get() (*JWKS, bool)
	Set(jwks *JWKS, ttl time.Duration)
}

type jwksCache struct {
	jwks    *JWKS
	expires time.Time
}

// NewJWKSCache creates a new JWKS cache
func NewJWKSCache() JWKSCache {
	return &jwksCache{}
}

func (c *jwksCache) Get() (*JWKS, bool) {
	if c.jwks == nil || time.Now().After(c.expires) {
		return nil, false
	}
	return c.jwks, true
}

func (c *jwksCache) Set(jwks *JWKS, ttl time.Duration) {
	c.jwks = jwks
	c.expires = time.Now().Add(ttl)
}

// GenerateJWKS generates a JWKS from a public key
func GenerateJWKS(publicKey *rsa.PublicKey, kid string) (*JWKS, error) {
	// Encode modulus (N) and exponent (E) to base64url
	nBytes := publicKey.N.Bytes()
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()

	n := base64.RawURLEncoding.EncodeToString(nBytes)
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	jwk := JWK{
		KTY: "RSA",
		USE: "sig",
		KID: kid,
		ALG: "RS256",
		N:   n,
		E:   e,
	}

	return &JWKS{
		Keys: []JWK{jwk},
	}, nil
}

// ToJSON converts JWKS to JSON
func (j *JWKS) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}

// JWKSEndpoint provides JWKS endpoint functionality
type JWKSEndpoint struct {
	issuer JWTIssuer
	cache  JWKSCache
	ttl    time.Duration
}

// NewJWKSEndpoint creates a new JWKS endpoint
func NewJWKSEndpoint(issuer JWTIssuer, cache JWKSCache, ttl time.Duration) *JWKSEndpoint {
	return &JWKSEndpoint{
		issuer: issuer,
		cache:  cache,
		ttl:    ttl,
	}
}

// GetJWKS returns the JWKS (from cache or generates new)
func (e *JWKSEndpoint) GetJWKS() ([]byte, error) {
	// Try cache first
	if e.cache != nil {
		if jwks, ok := e.cache.Get(); ok {
			return jwks.ToJSON()
		}
	}

	// Generate new JWKS
	publicKey, err := e.issuer.GetPublicKey()
	if err != nil {
		return nil, fmt.Errorf("get public key: %w", err)
	}

	jwks, err := GenerateJWKS(publicKey, "default")
	if err != nil {
		return nil, fmt.Errorf("generate JWKS: %w", err)
	}

	// Cache it
	if e.cache != nil {
		e.cache.Set(jwks, e.ttl)
	}

	return jwks.ToJSON()
}
