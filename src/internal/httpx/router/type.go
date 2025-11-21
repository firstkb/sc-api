package router

type Tier string

const (
	TierHealth       Tier = "health"        // /healthz, /readyz
	TierPublic       Tier = "public"        // no tenant, no auth
	TierPublicTenant Tier = "public_tenant" // tenant resolved, no auth (e.g. /auth/otp)
	TierSecure       Tier = "secure"        // tenant + auth
)

type RouteID string

type Route struct {
	ID     RouteID
	Method string // "GET", "POST", ...
	Path   string // "GET /ping" style
	Tier   Tier
}
