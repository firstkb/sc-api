package tokencoder

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// Schema знает, как превратить доменную структуру в полезную нагрузку токена и обратно.
type Schema[T any] interface {
	Marshal(T) ([]byte, error)
	Unmarshal([]byte) (T, error)
}

type Option func(*codecConfig)

type codecConfig struct {
	delimiter string
}

// Codec инкапсулирует общий формат токенов: base64(payload).base64(HMAC(payload,key)).
type Codec struct {
	key []byte

	delimiter string
}

// New создает новый Codec с обязательным ключом.
func New(key []byte, opts ...Option) (*Codec, error) {
	if len(key) == 0 {
		return nil, errors.New("tokencoder: empty key")
	}

	cfg := codecConfig{
		delimiter: ".",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.delimiter == "" {
		cfg.delimiter = "."
	}

	return &Codec{
		key:       append([]byte(nil), key...),
		delimiter: cfg.delimiter,
	}, nil
}

// WithDelimiter меняет разделитель между payload и подписью.
func WithDelimiter(delim string) Option {
	return func(cfg *codecConfig) {
		cfg.delimiter = delim
	}
}

// Encode сериализует доменную структуру и подписывает payload HMAC-SHA256.
func Encode[T any](codec *Codec, schema Schema[T], data T) (string, error) {
	if codec == nil {
		return "", errors.New("tokencoder: codec is nil")
	}
	if schema == nil {
		return "", errors.New("tokencoder: schema is nil")
	}
	payload, err := schema.Marshal(data)
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", errors.New("tokencoder: empty payload")
	}

	signature := codec.sign(payload)

	return base64.RawURLEncoding.EncodeToString(payload) +
		codec.delimiter +
		base64.RawURLEncoding.EncodeToString(signature), nil
}

// Decode валидирует подпись и десериализует payload через схему.
func Decode[T any](codec *Codec, schema Schema[T], token string) (T, error) {
	var zero T
	if codec == nil {
		return zero, errors.New("tokencoder: codec is nil")
	}
	if schema == nil {
		return zero, errors.New("tokencoder: schema is nil")
	}
	payload, err := codec.extractPayload(token)
	if err != nil {
		return zero, err
	}

	return schema.Unmarshal(payload)
}

func (c *Codec) extractPayload(token string) ([]byte, error) {
	if token == "" {
		return nil, errors.New("tokencoder: empty token")
	}
	parts := strings.SplitN(token, c.delimiter, 2)
	if len(parts) != 2 {
		return nil, errors.New("tokencoder: malformed token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	expected := c.sign(payload)
	if !hmac.Equal(signature, expected) {
		return nil, errors.New("tokencoder: invalid signature")
	}
	return payload, nil
}

func (c *Codec) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, c.key)
	mac.Write(payload)
	return mac.Sum(nil)
}
