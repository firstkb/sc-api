package identitysvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/firstkb/sc-api/internal/postgres"
)

type Repository struct {
	client *postgres.Client
}

func NewRepository(client *postgres.Client) *Repository {
	return &Repository{client: client}
}

func (r *Repository) FindSubject(ctx context.Context, subjectType SubjectType, hmac []byte) (*Subject, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	const query = `
SELECT id, type, hmac, hmac_kid, is_primary, status, created_at, updated_at
  FROM identity_subject
 WHERE type = $1 AND hmac = $2`

	var subj Subject
	if err := db.QueryRow(query, string(subjectType), hmac).Scan(
		&subj.ID,
		&subj.Type,
		&subj.HMAC,
		&subj.HMACKID,
		&subj.IsPrimary,
		&subj.Status,
		&subj.CreatedAt,
		&subj.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find subject: %w", err)
	}
	return &subj, nil
}

func (r *Repository) CreateSubject(ctx context.Context, subj *Subject) (*Subject, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	const query = `
INSERT INTO identity_subject (type, hmac, hmac_kid, is_primary, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (type, hmac)
DO UPDATE SET status = EXCLUDED.status
RETURNING id, type, hmac, hmac_kid, is_primary, status, created_at, updated_at`

	var created Subject
	if err := db.QueryRow(query, string(subj.Type), subj.HMAC, subj.HMACKID, subj.IsPrimary, subj.Status).
		Scan(&created.ID, &created.Type, &created.HMAC, &created.HMACKID, &created.IsPrimary, &created.Status, &created.CreatedAt, &created.UpdatedAt); err != nil {
		return nil, fmt.Errorf("create subject: %w", err)
	}
	return &created, nil
}

func (r *Repository) UpsertMembership(ctx context.Context, m *Membership) (*Membership, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("open master db: %w", err)
	}

	const query = `
INSERT INTO identity_tenant_membership (tenant_id, identity_id, tenant_user_id, role, level, status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, identity_id, tenant_user_id)
DO UPDATE SET role = EXCLUDED.role,
              level = EXCLUDED.level,
              status = EXCLUDED.status,
              updated_at = now()
RETURNING tenant_id, identity_id, tenant_user_id, role, level, status, created_at, updated_at`

	var res Membership
	if err := db.QueryRow(query, m.TenantID, m.IdentityID, m.TenantUserID, m.Role, m.Level, m.Status).
		Scan(&res.TenantID, &res.IdentityID, &res.TenantUserID, &res.Role, &res.Level, &res.Status, &res.CreatedAt, &res.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert membership: %w", err)
	}
	return &res, nil
}

func (r *Repository) DeleteMembership(ctx context.Context, tenantID int64, identityID int64, tenantUserID string) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("open master db: %w", err)
	}

	const query = `
DELETE FROM identity_tenant_membership
 WHERE tenant_id = $1 AND identity_id = $2 AND tenant_user_id = $3`

	if _, err := db.Exec(query, tenantID, identityID, tenantUserID); err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	return nil
}
