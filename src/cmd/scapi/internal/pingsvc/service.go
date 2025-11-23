package pingsvc

import (
	"context"
	"errors"
	"log/slog"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/postgres"
)

type Request struct {
	Parts []string `json:"parts"`
}

type PingService struct {
	logger     *slog.Logger
	repository *Repo
}

func NewService(sqlClient *postgres.Client, config *config.Config, logger *slog.Logger) (*PingService, error) {

	return &PingService{
		logger:     logger,
		repository: NewRepo(sqlClient, logger),
	}, nil
}

func (s *PingService) GetPing(ctx context.Context) (*Ping, error) {

	tenantID := requestctx.TenantID(ctx)
	if tenantID == "" {
		return nil, errors.New("tenant ID not found")
	}

	ping, err := s.repository.getPing(ctx)
	if err != nil {
		return nil, err
	}

	users, err := s.repository.getUsers(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	tenantInfo, ok := requestctx.Tenant(ctx)
	if !ok {
		return nil, errors.New("tenant not found")
	}
	identityInfo, ok := requestctx.Identity(ctx)
	if !ok {
		//return nil, errors.New("identity not found")
		identityInfo = requestctx.IdentityInfo{
			ID: "anonymous",
		}
	}
	claimsInfo, ok := requestctx.Claims(ctx)
	if !ok {
		return nil, errors.New("claims ot found")
	}
	userInfo, ok := requestctx.User(ctx)
	if !ok {
		//return nil, errors.New("user not found")
		userInfo = requestctx.UserInfo{
			ID: "anonymous",
		}
	}

	return &Ping{
		Message:  ping,
		Users:    users,
		TenantId: claimsInfo.TenantID,
		Tenant:   tenantInfo,
		Identity: identityInfo,
		Claims:   claimsInfo,
		Plan:     tenantInfo.Plan,
		User:     userInfo,
	}, nil
}
