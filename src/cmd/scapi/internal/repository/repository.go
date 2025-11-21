package repository

import (
	"context"
	"log/slog"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/postgres"
)

type Repository struct {
	Client *postgres.Client
	Logger *slog.Logger
	Claim  *utils.Claim
}

func NewRepository(client *postgres.Client, logger *slog.Logger) Repository {
	return Repository{
		Client: client,
		Logger: logger,
	}
}

func (r *Repository) OpenDB(ctx context.Context) (*postgres.Database, error) {
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
