package eventsvc

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/postgres"
)

// Service writes audit events to master DB.
type EventService struct {
	client *postgres.Client
	repo   *Repo
	logger *slog.Logger
}

type tenantTarget struct {
	ID           string
	DBName       string
	InstanceCode string
}

// NewService wires repository with postgres client.
func NewService(sqlClient *postgres.Client, _ *config.Config, logger *slog.Logger) (*EventService, error) {

	repo := NewRepo(sqlClient, logger)

	return &EventService{
		client: sqlClient,
		repo:   repo,
		logger: logger,
	}, nil
}

// Log builds event payload from context/request and writes it to tenant DB.
func (s *EventService) Log(ctx context.Context, eventType EventType, r *http.Request, data EventData) error {
	event := Event{
		EventType: eventType,
		EventData: data,
		CreatedAt: time.Now(),
	}

	if identity, ok := requestctx.Identity(ctx); ok && identity.ID != "" {
		if parsed, err := uuid.Parse(identity.ID); err == nil {
			event.UserID = &parsed
		}
	}

	if r != nil {
		event.IPAddress = extractIP(r)
		event.UserAgent = summarizeUserAgent(r.Header.Get("User-Agent"))
	}

	return s.repo.insert(ctx, event)
}
