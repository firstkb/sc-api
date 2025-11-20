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

	users100, err := s.repository.getUsers(ctx, "100")
	if err != nil {
		return nil, err
	}

	users101, err := s.repository.getUsers(ctx, "101")
	if err != nil {
		return nil, err
	}

	users102, err := s.repository.getUsers(ctx, "102")
	if err != nil {
		return nil, err
	}

	return &Ping{
		Message: ping,
		Users: map[string][]map[string]any{
			"100": users100,
			"101": users101,
			"102": users102,
		},
	}, nil
}
