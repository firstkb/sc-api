package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32
)

// HashOTP hashes an OTP code using Argon2id
func HashOTP(code string) (string, error) {
	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	// Hash the code
	hash := argon2.IDKey([]byte(code), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode: format is "salt:hash" (both base64)
	// For simplicity, we'll use hex encoding
	return fmt.Sprintf("%x:%x", salt, hash), nil
}

// VerifyOTP verifies an OTP code against its hash
func VerifyOTP(code, hash string) (bool, error) {
	// Parse salt and hash
	var salt, expectedHash []byte
	_, err := fmt.Sscanf(hash, "%x:%x", &salt, &expectedHash)
	if err != nil {
		return false, fmt.Errorf("parse hash: %w", err)
	}

	// Compute hash for the provided code
	computedHash := argon2.IDKey([]byte(code), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1, nil
}
