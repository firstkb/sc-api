package tenantsvc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/firstkb/sc-api/internal/postgres"
)

type Repository struct {
	client *postgres.Client
	logger *slog.Logger
}

func NewRepository(client *postgres.Client, logger *slog.Logger) *Repository {
	return &Repository{client: client, logger: logger}
}

func (r *Repository) GetByID(ctx context.Context, tenantID string) (*Tenant, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	query := `
	SELECT host,
		tenant_id,
		tenant_name,
		plan,
		isolation,
		status,
		db_name,
		tenant_updated_at,
		db_instance_id,
		db_instance_code,
		updated_at
	FROM v_tenant_by_host
	WHERE tenant_id = $1`

	row := db.QueryRow(query, tenantID)
	var tenant Tenant
	if err := row.Scan(&tenant.Host, &tenant.ID, &tenant.Name, &tenant.Plan, &tenant.Isolation, &tenant.Status, &tenant.DBName, &tenant.TenantUpdatedAt, &tenant.DBInstanceID, &tenant.DBInstanceCode, &tenant.UpdatedAt); err != nil {
		return nil, fmt.Errorf("GetByID tenant: %w", err)
	}
	return &tenant, nil
}

func (r *Repository) GetByHost(ctx context.Context, host string) (*Tenant, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	query := `
	SELECT host,
		tenant_id,
		tenant_name,
		plan,
		isolation,
		status,
		db_name,
		tenant_updated_at,
		db_instance_id,
		db_instance_code,
		updated_at
	FROM v_tenant_by_host
	WHERE host = $1`
	row := db.QueryRow(query, host)
	var tenant Tenant
	if err := row.Scan(&tenant.Host, &tenant.ID, &tenant.Name, &tenant.Plan, &tenant.Isolation, &tenant.Status, &tenant.DBName, &tenant.TenantUpdatedAt, &tenant.DBInstanceID, &tenant.DBInstanceCode, &tenant.UpdatedAt); err != nil {
		return nil, fmt.Errorf("GetByHost tenant: %w", err)
	}
	return &tenant, nil
}

func (r *Repository) CheckTenantByHostAndID(ctx context.Context, host string, tenantID string) (*Tenant, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	query := `
	SELECT host,
		tenant_id,
		tenant_name,
		plan,
		isolation,
		status,
		db_name,
		tenant_updated_at,
		db_instance_id,
		db_instance_code,
		updated_at
	FROM v_tenant_by_host
	WHERE host = $1 AND tenant_id = $2`
	row := db.QueryRow(query, host, tenantID)
	var tenant Tenant
	if err := row.Scan(&tenant.Host, &tenant.ID, &tenant.Name, &tenant.Plan, &tenant.Isolation, &tenant.Status, &tenant.DBName, &tenant.TenantUpdatedAt, &tenant.DBInstanceID, &tenant.DBInstanceCode, &tenant.UpdatedAt); err != nil {
		return nil, fmt.Errorf("CheckTenantByHostAndID tenant: %w", err)
	}
	return &tenant, nil
}
