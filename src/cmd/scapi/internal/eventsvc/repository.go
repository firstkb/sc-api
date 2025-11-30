package eventsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/firstkb/sc-api/cmd/scapi/internal/repository"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
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

func (r *Repo) insert(ctx context.Context, event Event) error {
	db, err := r.OpenDB(ctx)
	if err != nil {
		return fmt.Errorf("eventsvc: open tenant db: %w", err)
	}

	tenant, ok := requestctx.Tenant(ctx)
	if !ok || tenant.ID == "" {
		return fmt.Errorf("eventsvc: tenant not found")
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(tenant.ID), 10, 64)
	if err != nil {
		return fmt.Errorf("eventsvc: invalid tenant id")
	}

	var userID interface{}
	if event.UserID != nil && *event.UserID != uuid.Nil {
		userID = *event.UserID
	}

	var eventDataJSON []byte
	if len(event.EventData) > 0 {
		eventDataJSON, err = json.Marshal(event.EventData)
		if err != nil {
			return fmt.Errorf("eventsvc: marshal event data: %w", err)
		}
	}

	var ipAddr interface{}
	if parsed := net.ParseIP(event.IPAddress); parsed != nil {
		ipAddr = parsed.String()
	}

	var userAgent interface{}
	if ua := strings.TrimSpace(event.UserAgent); ua != "" {
		userAgent = ua
	}

	const query = `
INSERT INTO event_log (tenant_id, user_id, event_type, event_data, ip_address, user_agent, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = db.Exec(query,
		tenantID,
		userID,
		string(event.EventType),
		eventDataJSON,
		ipAddr,
		userAgent,
		event.CreatedAt,
	)
	return err
}
