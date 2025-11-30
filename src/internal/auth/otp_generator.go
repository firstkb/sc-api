package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateOTP generates a random numeric OTP code
func GenerateOTP(length int) (string, error) {
	if length < 4 || length > 8 {
		return "", fmt.Errorf("OTP length must be between 4 and 8, got %d", length)
	}

	max := big.NewInt(10)
	max.Exp(max, big.NewInt(int64(length)), nil)
	max.Sub(max, big.NewInt(1))

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate random OTP: %w", err)
	}

	// Format with leading zeros
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n.Int64()), nil
}
