package router

type ctxKey string

const (
	CtxKeyTier    ctxKey = "httpx.tier"
	CtxKeyRouteID ctxKey = "httpx.route_id"
)
