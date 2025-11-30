package identity

import (
	"fmt"
	"regexp"
	"strings"
)

var phoneSanitizer = regexp.MustCompile(`[^0-9+]`)

func normalizeSubjectValue(subjectType SubjectType, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrEmptyValue
	}

	switch subjectType {
	case SubjectTypeEmail:
		return strings.ToLower(trimmed), nil
	case SubjectTypePhone:
		return normalizePhone(trimmed)
	default:
		return "", ErrInvalidSubjectType
	}
}

func normalizePhone(value string) (string, error) {
	sanitized := phoneSanitizer.ReplaceAllString(value, "")
	if strings.HasPrefix(sanitized, "00") {
		sanitized = "+" + sanitized[2:]
	}
	if !strings.HasPrefix(sanitized, "+") {
		sanitized = "+" + sanitized
	}
	if len(sanitized) < 8 {
		return "", fmt.Errorf("phone number too short: %s", value)
	}
	return sanitized, nil
}
