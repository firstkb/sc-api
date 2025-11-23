package repository

import (
	"context"
	"errors"
	"log/slog"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/postgres"
)

// Repository is the repository for the application.
type Repository struct {
	Client *postgres.Client
	Logger *slog.Logger
	Claim  *utils.Claim
}

// NewRepository creates a new repository.
func NewRepository(client *postgres.Client, logger *slog.Logger) Repository {
	return Repository{
		Client: client,
		Logger: logger,
	}
}

// OpenDBFromClaim open database from claim in context.
/*func (r *Repository) OpenDBFromClaim(ctx context.Context) (*postgres.Database, error) {
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
}*/

func (r *Repository) OpenDBFromTenant(ctx context.Context) (*postgres.Database, error) {
	tenantID := utils.GetTenantID(ctx)
	if tenantID == "" {
		return nil, errors.New("tenant ID is required")
	}

	db, err := r.Client.OpenDB(ctx, tenantID)
	if err != nil {
		return nil, errors.New("failed to open database")
	}

	return db, nil
}

// OpenDBByTenantID open database by explicitly passed tenantID, without using claim.
func (r *Repository) OpenDBByTenantID(ctx context.Context, tenantID string) (*postgres.Database, error) {
	db, err := r.Client.OpenDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return db, nil
}
