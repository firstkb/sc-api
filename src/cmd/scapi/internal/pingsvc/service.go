package pingsvc

import (
	"context"
	"log/slog"

	"github.com/firstkb/sc-api/internal/sqlserver"

	"github.com/firstkb/sc-api/internal/config"
)

type Request struct {
	Parts []string `json:"parts"`
}

type PingService struct {
	logger     *slog.Logger
	repository *Repo
}

func NewService(sqlClient *sqlserver.Client, config *config.Config, logger *slog.Logger) (*PingService, error) {

	return &PingService{
		logger:     logger,
		repository: NewRepo(sqlClient, logger),
	}, nil
}

func (s *PingService) GetPing(ctx context.Context) (*Ping, error) {

	ping, err := s.repository.getPing(ctx)
	if err != nil {
		return nil, err
	}

	return &Ping{
		Message: ping,
	}, nil
}
