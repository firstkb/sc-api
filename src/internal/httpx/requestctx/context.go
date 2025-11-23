package requestctx

import "context"

// from returns a *RequestCtx if present; nil otherwise.
func from(ctx context.Context) *RequestCtx {
	if ctx == nil {
		return nil
	}
	if rc, ok := ctx.Value(requestCtxKey).(*RequestCtx); ok && rc != nil {
		return rc
	}
	return nil
}

// base returns a value copy of current ctx (or zero) to be mutated and re-inserted.
func base(ctx context.Context) RequestCtx {
	if rc := from(ctx); rc != nil {
		return *rc // copy value
	}
	return RequestCtx{}
}

// with mutates a value-copy and stores a new pointer in a derived context (copy-on-write).
func with(ctx context.Context, mutate func(*RequestCtx)) context.Context {
	b := base(ctx)
	mutate(&b)
	return context.WithValue(ctx, requestCtxKey, &b)
}

// -------- Public API --------

func From(ctx context.Context) *RequestCtx {
	return from(ctx) // read-only pointer; do not mutate outside
}

func WithRoute(ctx context.Context, route RouteInfo) context.Context {
	return with(ctx, func(r *RequestCtx) { r.Route = route })
}

func Route(ctx context.Context) (RouteInfo, bool) {
	if rc := from(ctx); rc != nil && rc.Route.ID != "" {
		return rc.Route, true
	}
	return RouteInfo{}, false
}

func WithTenant(ctx context.Context, tenant TenantInfo) context.Context {
	return with(ctx, func(r *RequestCtx) { r.Tenant = tenant })
}

func Tenant(ctx context.Context) (TenantInfo, bool) {
	if rc := from(ctx); rc != nil && rc.Tenant.ID != "" {
		return rc.Tenant, true
	}
	return TenantInfo{}, false
}

func WithIdentity(ctx context.Context, identity IdentityInfo) context.Context {
	return with(ctx, func(r *RequestCtx) { r.Identity = identity })
}

func Identity(ctx context.Context) (IdentityInfo, bool) {
	if rc := from(ctx); rc != nil && rc.Identity.ID != "" {
		return rc.Identity, true
	}
	return IdentityInfo{}, false
}

func WithClaims(ctx context.Context, claims ClaimsInfo) context.Context {
	return with(ctx, func(r *RequestCtx) { r.Claims = claims })
}

func Claims(ctx context.Context) (ClaimsInfo, bool) {
	if rc := from(ctx); rc != nil {
		c := rc.Claims
		if c.TenantID != "" || c.UserID != "" || c.Email != "" {
			return c, true
		}
	}
	return ClaimsInfo{}, false
}

func WithUser(ctx context.Context, user UserInfo) context.Context {
	return with(ctx, func(r *RequestCtx) { r.User = user })
}

func User(ctx context.Context) (UserInfo, bool) {
	if rc := from(ctx); rc != nil && rc.User.ID != "" {
		return rc.User, true
	}
	return UserInfo{}, false
}

// Convenience getters (zero-values, без bool)
func TenantID(ctx context.Context) string {
	if rc := from(ctx); rc != nil && rc.Tenant.ID != "" {
		return rc.Tenant.ID
	}
	return ""
}

func RequestID(ctx context.Context) string {
	if rc := from(ctx); rc != nil {
		return rc.RequestID
	}
	return ""
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return with(ctx, func(r *RequestCtx) { r.RequestID = id })
}

func WithTraceID(ctx context.Context, id string) context.Context {
	return with(ctx, func(r *RequestCtx) { r.TraceID = id })
}
