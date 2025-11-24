package pingsvc

import (
	"context"
	"log/slog"

	"github.com/firstkb/sc-api/cmd/scapi/internal/repository"
	"github.com/firstkb/sc-api/internal/postgres"
)

type Repo struct {
	repository.Repository
}

func NewRepo(client *postgres.Client, logger *slog.Logger) *Repo {
	repository := repository.NewRepository(client, logger)
	return &Repo{
		Repository: repository,
	}
}

func (r *Repo) getPing(ctx context.Context) (string, error) {
	_, err := r.OpenDB(ctx)
	if err != nil {
		return "", err
	}
	return "pong", nil
}

func (r *Repo) getUsers(ctx context.Context, tenantID string) ([]map[string]any, error) {
	db, err := r.OpenDB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.Select("SELECT * FROM users WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 10", tenantID)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
