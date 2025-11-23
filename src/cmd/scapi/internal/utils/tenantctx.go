package utils

import (
	"context"
	"strings"

	"github.com/firstkb/sc-api/internal/httpx/claims"
)

type tenantCtxKey string

const (
	ctxKeyTenantID tenantCtxKey = "scapi.tenant.id"
)

// WithTenantID сохраняет tenant_id в контексте, чтобы его могли использовать middleware и сервисы.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKeyTenantID, tenantID)
}

// GetTenantID возвращает tenant_id из кастомного контекста или из claim.
func GetTenantID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if tenantID, ok := ctx.Value(ctxKeyTenantID).(string); ok && tenantID != "" {
		return tenantID
	}
	if claim, err := GetClaim(ctx); err == nil && claim != nil && claim.TenantID != "" {
		return claim.TenantID
	}
	return ""
}

// BindTenantClaim гарантирует, что в контексте будет Claim с указанным tenant_id.
func BindTenantClaim(ctx context.Context, tenantID string) context.Context {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}

	if claim, err := GetClaim(ctx); err == nil && claim != nil {
		if claim.TenantID == tenantID {
			return ctx
		}
		clone := *claim
		clone.TenantID = tenantID
		return claims.With(ctx, &clone)
	}

	return claims.With(ctx, &Claim{TenantID: tenantID, UserID: "anonymous", Email: ""})
}

// BindTenant полностью настраивает контекст для публичных роутов: сохраняет tenant_id и Claim.
func BindTenant(ctx context.Context, tenantID string) context.Context {
	ctx = WithTenantID(ctx, tenantID)
	ctx = BindTenantClaim(ctx, tenantID)
	return ctx
}
