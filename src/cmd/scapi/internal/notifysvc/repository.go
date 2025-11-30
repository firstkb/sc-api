package notifysvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (r *Repo) getTemplate(ctx context.Context, tenantID, kind, key, locale string) (*Template, error) {
	db, err := r.OpenDB(ctx)
	if err != nil {
		return nil, err
	}
	var tpl Template
	queryTenant := `
SELECT id, tenant_id, kind, key, locale, subject, html, text
  FROM notification_template
 WHERE tenant_id = $1 AND kind = $2 AND key = $3 AND locale = $4
 LIMIT 1`

	err = db.QueryRow(queryTenant, tenantID, kind, key, locale).Scan(
		&tpl.ID, &tpl.TenantID, &tpl.Kind, &tpl.Key, &tpl.Locale, &tpl.Subject, &tpl.HTML, &tpl.Text,
	)
	switch {
	case err == nil:
		return &tpl, nil
	case errors.Is(err, sql.ErrNoRows):
		// fallback
	default:
		return nil, fmt.Errorf("notifysvc: tenant template query: %w", err)
	}

	queryGlobal := `
SELECT id, tenant_id, kind, key, locale, subject, html, text
  FROM notification_template
 WHERE tenant_id IS NULL AND kind = $1 AND key = $2 AND locale = $3
 LIMIT 1`

	err = db.QueryRow(queryGlobal, kind, key, locale).Scan(
		&tpl.ID, &tpl.TenantID, &tpl.Kind, &tpl.Key, &tpl.Locale, &tpl.Subject, &tpl.HTML, &tpl.Text,
	)
	switch {
	case err == nil:
		return &tpl, nil
	case errors.Is(err, sql.ErrNoRows):
		if locale != "default" {
			return r.getTemplate(ctx, tenantID, kind, key, "default")
		}
		return nil, fmt.Errorf("notifysvc: template %s/%s locale=%s not found", kind, key, locale)
	default:
		return nil, fmt.Errorf("notifysvc: global template query: %w", err)
	}
}
