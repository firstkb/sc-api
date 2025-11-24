package tenantsvc

import (
	"context"
	"errors"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/httpx/router"
	"github.com/firstkb/sc-api/internal/tokencoder"
)

var (
	ErrTenantClaimMissing            = errors.New("tenant claim is missing")
	ErrOriginMissing                 = errors.New("origin domain is missing")
	ErrTenantHostMismatch            = errors.New("tenant host mismatch")
	ErrTenantNotActive               = errors.New("tenant is not active")
	ErrInvalidTier                   = errors.New("invalid tier")
	ErrInvalidRouteIDForPublicTenant = errors.New("invalid route ID for public tenant validation")
	ErrTokenCodecNotConfigured       = errors.New("token codec not configured")
)

func (s *ServiceTenantProvider) Validate(ctx context.Context, tier router.Tier, routeID router.RouteID, domain string, r *http.Request) (context.Context, error) {

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

	ctx = requestctx.WithTenant(ctx, requestctx.TenantInfo{
		ID:             tenant.ID,
		Host:           tenant.Host,
		Status:         tenant.Status,
		Plan:           tenant.Plan,
		DBName:         tenant.DBName,
		DBInstanceID:   tenant.DBInstanceID,
		DBInstanceCode: tenant.DBInstanceCode,
	})

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
	routeInfo, ok := requestctx.Route(ctx)
	if !ok {
		return nil, ErrOriginMissing
	}

	var tenant *Tenant
	var err error
	switch routeID {
	case router.RouteID("LOGIN_TEST_GET"):
		if routeInfo.Domain == "" {
			return nil, ErrOriginMissing
		}
		tenant, err = s.GetByHost(ctx, routeInfo.Domain)
		if err != nil {
			return nil, err
		}
	default:
		params, paramsOK := router.ExtractPathParams(routeInfo.URI, r.URL.Path)
		code := ""
		if paramsOK {
			code = params["code"]
		}
		if code == "" {
			return nil, ErrInvalidRouteIDForPublicTenant
		}

		payload, err := s.decodeSurveyToken(code)
		if err != nil {
			return nil, err
		}

		tenant, err = s.GetByID(ctx, payload.TenantID)
		if err != nil {
			return nil, err
		}
		if s.logger != nil {
			s.logger.Info("Validate: survey code accepted", "tenantID", payload.TenantID, "surveyID", payload.SurveyID, "issuedAt", payload.IssuedAt)
		}
	}

	return tenant, nil
}

func (s *ServiceTenantProvider) decodeSurveyToken(code string) (SurveyTokenPayload, error) {
	var zero SurveyTokenPayload
	if s.tokenCodec == nil {
		return zero, ErrTokenCodecNotConfigured
	}

	payload, err := tokencoder.Decode(s.tokenCodec, SurveyTokenSchema{}, code)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Validate: invalid survey code", "error", err)
		}
		return zero, ErrInvalidRouteIDForPublicTenant
	}

	return payload, nil
}
