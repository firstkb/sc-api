package pingsvc

import (
	"context"
	"log/slog"

	"github.com/firstkb/sc-api/cmd/scapi/internal/repository"
	"github.com/firstkb/sc-api/internal/sqlserver"
)

type Repo struct {
	repository.Repository
}

func NewRepo(client *sqlserver.Client, logger *slog.Logger) *Repo {
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
