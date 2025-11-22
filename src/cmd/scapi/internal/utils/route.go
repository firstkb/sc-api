package utils

import (
	"context"
	"errors"

	"github.com/firstkb/sc-api/internal/httpx/router"
)

var errNoRouteInContext = errors.New("route info not found in context")

// GetRouteInfo returns the RouteID and Tier from context.
func GetRouteInfo(ctx context.Context) (router.RouteID, router.Tier, error) {
	if ctx == nil {
		return "", "", errNoRouteInContext
	}

	tier, ok := ctx.Value(router.CtxKeyTier).(router.Tier)
	if !ok {
		return "", "", errNoRouteInContext
	}

	routeID, ok := ctx.Value(router.CtxKeyRouteID).(router.RouteID)
	if !ok {
		return "", "", errNoRouteInContext
	}

	return routeID, tier, nil
}

// GetRouteDomain extracts normalized domain stored in context (via router middleware).
func GetRouteDomain(ctx context.Context) string {
	if ctx == nil {
		return "undefined"
	}

	if domain, ok := ctx.Value(router.CtxKeyDomain).(string); ok && domain != "" {
		return domain
	}

	return "undefined"
}
