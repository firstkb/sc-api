package tenantsvc

import (
	"context"
	"errors"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

var (
	ErrTenantClaimMissing            = errors.New("tenant claim is missing")
	ErrOriginMissing                 = errors.New("origin domain is missing")
	ErrTenantHostMismatch            = errors.New("tenant host mismatch")
	ErrTenantNotActive               = errors.New("tenant is not active")
	ErrInvalidTier                   = errors.New("invalid tier")
	ErrInvalidRouteIDForPublicTenant = errors.New("invalid route ID for public tenant validation")
)

func (s *ServiceTenantProvider) Validate(ctx context.Context, tier router.Tier, routeID router.RouteID, r *http.Request) (context.Context, error) {
	domain := utils.GetRouteDomain(ctx)
	if domain == "" {
		return ctx, ErrOriginMissing
	}

	var tenant *Tenant
	var err error
	switch tier {
	case router.TierSecure:
		tenant, err = s.validateSecure(ctx, routeID, r)
		if err != nil {
			return ctx, err
		}
	case router.TierPublicTenant:
		tenant, err = s.validatePublicTenant(ctx, routeID, r)
		if err != nil {
			return ctx, err
		}
	default:
		return ctx, ErrInvalidTier
	}

	if tenant.Host != domain {
		s.logger.Error("Validate: Host mismatch", "tenantHost", tenant.Host, "domain", domain)
		return ctx, ErrTenantHostMismatch
	}

	// TODO: check if tenant is active
	if tenant.Status != "active" {
		s.logger.Error("Validate: Tenant is not active", "tenant", tenant)
		return ctx, ErrTenantNotActive
	}

	// TODO: set tenant in context
	ctx = utils.WithTenantID(ctx, tenant.ID)

	return ctx, nil
}

func (s *ServiceTenantProvider) validateSecure(ctx context.Context, routeID router.RouteID, r *http.Request) (*Tenant, error) {
	claim, err := utils.GetClaim(ctx)
	if err != nil {
		return nil, err
	}
	if claim == nil || claim.TenantID == "" {
		return nil, ErrTenantClaimMissing
	}

	tenant, err := s.GetByID(ctx, claim.TenantID)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}

func (s *ServiceTenantProvider) validatePublicTenant(ctx context.Context, routeID router.RouteID, r *http.Request) (*Tenant, error) {
	// TODO: set switch on routerID and routeID to get tenant by host or other way
	var tenant *Tenant
	var err error
	switch routeID {
	case "survey_get":
		tenant, err = s.GetByHost(ctx, utils.GetRouteDomain(ctx))
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidRouteIDForPublicTenant
	}
	return tenant, nil
}
