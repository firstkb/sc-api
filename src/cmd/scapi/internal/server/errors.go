package server

import (
	"context"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/utils/httpext/apperr"
)

func (srv *Server) wrapErr(
	ctx context.Context,
	code string,
	status int,
	msg string,
	err error,
	kv ...any,
) *apperr.AppError {

	claim, _ := utils.GetClaim(ctx)

	fields := []any{
		"code", code,
		"error", err,
	}

	if claim != nil {
		fields = append(fields,
			"tenantID", func() any {
				if claim != nil {
					return claim.TenantID
				}
				return nil
			}(),
			"email", func() any {
				if claim != nil {
					return claim.Email
				}
				return nil
			}(),
			"server", func() any {
				if claim != nil {
					return claim.ServerName
				}
				return nil
			}(),
		)
	}

	fields = append(fields, kv...)

	srv.logger.Error(msg, fields...)

	return apperr.Wrap(err, code, status, msg)
}
