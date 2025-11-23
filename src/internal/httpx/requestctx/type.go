// internal/httpx/requestctx/type.go
package requestctx

import "github.com/firstkb/sc-api/internal/httpx/router"

type ctxKey struct{}

var requestCtxKey ctxKey

// RequestCtx aggregates per-request metadata populated by HTTP middleware.
type RequestCtx struct {
	// Generic observability
	RequestID string
	TraceID   string

	Route    RouteInfo
	Tenant   TenantInfo
	Identity IdentityInfo
	Claims   ClaimsInfo
	User     UserInfo
}

type RouteInfo struct {
	Tier    router.Tier
	ID      router.RouteID
	Pattern string
	Domain  string // normalized host for routing/metrics
}

type TenantInfo struct {
	ID             string // prefer numeric if DB is BIGINT
	Host           string
	Status         string
	Plan           string
	DBName         string
	DBInstanceID   string
	DBInstanceCode string
}

type IdentityInfo struct {
	ID string // master identity/subject id
}

type ClaimsInfo struct {
	TenantID string
	UserID   string
	Email    string
	Level    int
	Roles    []string
}

type UserInfo struct {
	ID    string
	Email string
	Level int
	Roles []string
}
