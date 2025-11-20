package repository

import (
	"context"
	"log/slog"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/sqlserver"
)

type Repository struct {
	Client *sqlserver.Client
	Logger *slog.Logger
	Claim  *utils.Claim
}

func NewRepository(client *sqlserver.Client, logger *slog.Logger) Repository {
	return Repository{
		Client: client,
		Logger: logger,
	}
}

func (r *Repository) OpenDB(ctx context.Context) (*sqlserver.Database, error) {
	claim, err := utils.GetClaim(ctx)
	if err != nil {
		return nil, err
	}

	db, err := r.Client.OpenDB(ctx, claim.TenantID)
	if err != nil {
		return nil, err
	}

	r.Claim = claim

	return db, nil
}
