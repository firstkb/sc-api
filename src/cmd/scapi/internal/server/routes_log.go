package server

import (
	"context"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
)

func (srv *Server) ClaimForLog(ctx context.Context) []any {
	fields := []any{}
	if c, err := utils.GetClaim(ctx); err == nil && c != nil {
		fields = append(fields,
			"tenantID", c.TenantID,
			"email", c.Email,
			"server", c.ServerName,
		)
	}
	return fields
}
