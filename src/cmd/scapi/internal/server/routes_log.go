package server

import (
	"context"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
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

	if routeInfo, ok := requestctx.Route(ctx); ok {
		fields = append(fields,
			//"routeID", string(routeInfo.ID),
			"tier", string(routeInfo.Tier),
			"domain", routeInfo.Domain,
		)
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
