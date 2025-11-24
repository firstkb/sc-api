package tokencoder

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// ParseKey принимает строковое представление ключа и пытается декодировать его из Base64/RawBase64/HEX,
// либо возвращает байты как есть.
func ParseKey(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("tokencoder: key is empty")
	}

	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) > 0 {
		return decoded, nil
	}

	return []byte(value), nil
}

// GenerateNonce генерирует URL-safe строку указанного размера.
func GenerateNonce(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
