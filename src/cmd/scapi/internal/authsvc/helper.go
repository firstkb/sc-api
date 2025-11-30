package authsvc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/firstkb/sc-api/internal/auth"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
)

func isMembershipActive(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "active")
}

func tenantFromContext(ctx context.Context) (tenant requestctx.TenantInfo, tenantID int64, err error) {
	info, ok := requestctx.Tenant(ctx)
	if !ok || info.ID == "" {
		return requestctx.TenantInfo{}, 0, ErrTenantMissing
	}
	id, convErr := strconv.ParseInt(strings.TrimSpace(info.ID), 10, 64)
	if convErr != nil {
		return requestctx.TenantInfo{}, 0, ErrTenantMissing
	}
	return info, id, nil
}

func resolveChannel(email, phone *string) (string, string) {
	if email != nil && strings.TrimSpace(*email) != "" {
		value := strings.TrimSpace(*email)
		return "email", value
	}
	if phone != nil && strings.TrimSpace(*phone) != "" {
		value := strings.TrimSpace(*phone)
		return "sms", value
	}
	return "", ""
}

func maskAddress(channel, address string) string {
	if channel == "email" {
		return maskEmail(address)
	}
	return maskPhone(address)
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) == 0 {
		return "***@" + domain
	}
	if len(local) == 1 {
		return local[:1] + "***@" + domain
	}
	return local[:1] + "***@" + domain
}

func maskPhone(phone string) string {
	if len(phone) <= 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}

func generateRefreshToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func parseDuration(value string, def time.Duration) time.Duration {
	if strings.TrimSpace(value) == "" {
		return def
	}
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	return def
}

func parseRateLimit(value string, defaultMax int, defaultWindow time.Duration) auth.RateLimiterConfig {
	cfg := auth.RateLimiterConfig{
		MaxRequests: defaultMax,
		Window:      defaultWindow,
		MaxSize:     10000,
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return cfg
	}
	if max, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && max > 0 {
		cfg.MaxRequests = max
	}
	if window, err := time.ParseDuration(strings.TrimSpace(parts[1])); err == nil && window > 0 {
		cfg.Window = window
	}
	return cfg
}
