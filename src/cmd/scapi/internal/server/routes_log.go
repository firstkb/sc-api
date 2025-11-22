package server

import (
	"context"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
)

func (srv *Server) FieldsForLog(ctx context.Context, r *http.Request, in any) []any {
	fields := []any{}

	if c, err := utils.GetClaim(ctx); err == nil && c != nil {
		fields = append(fields,
			"tenantID", c.TenantID,
			"userID", c.UserID,
			"email", c.Email,
		)
	}

	if _, tier, err := utils.GetRouteInfo(ctx); err == nil {
		fields = append(fields,
			//"routeID", string(routeID),
			"tier", string(tier),
		)
	}

	domain := utils.GetRouteDomain(ctx)
	if domain != "" && domain != "undefined" {
		fields = append(fields, "domain", domain)
	}

	query := r.URL.RawQuery
	if query != "" {
		fields = append(fields, "query", query)
	}

	if in != nil {
		fields = append(fields, "input", in)
	}

	return fields
}
