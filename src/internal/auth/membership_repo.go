package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/firstkb/sc-api/internal/postgres"
)

var (
	ErrMembershipNotFound = errors.New("membership not found")
	ErrMembershipInactive = errors.New("membership is inactive")
)

// MembershipRepository handles membership database operations
type MembershipRepository interface {
	GetMembership(ctx context.Context, tenantUserID string, tenantID int64) (*Membership, error)
}

type membershipRepository struct {
	client *postgres.Client
}

// NewMembershipRepository creates a new membership repository against master DB.
func NewMembershipRepository(client *postgres.Client) MembershipRepository {
	return &membershipRepository{client: client}
}

func (r *membershipRepository) GetMembership(ctx context.Context, tenantUserID string, tenantID int64) (*Membership, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("membership: open master db: %w", err)
	}

	query := `
		SELECT tenant_user_id, tenant_id, identity_id, role, level, status
		FROM identity_tenant_membership
		WHERE tenant_user_id = $1 AND tenant_id = $2
	`

	var m Membership
	var roleStr string
	var levelInt int

	err = db.QueryRow(query, tenantUserID, tenantID).Scan(
		&m.TenantUserID,
		&m.TenantID,
		&m.IdentityID,
		&roleStr,
		&levelInt,
		&m.Status,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMembershipNotFound
		}
		return nil, fmt.Errorf("get membership: %w", err)
	}

	m.Role = Role(roleStr)
	m.Level = AccessLevel(levelInt)

	return &m, nil
}
