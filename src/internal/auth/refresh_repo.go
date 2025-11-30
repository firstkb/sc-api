package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/firstkb/sc-api/internal/postgres"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
)

// RefreshToken represents a refresh token
type RefreshToken struct {
	ID        uuid.UUID
	TenantID  int64
	UserID    uuid.UUID
	TokenHash string // SHA256 hash of the token
	ClientID  *string
	DeviceID  *string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// RefreshTokenRepository handles refresh token database operations
type RefreshTokenRepository interface {
	CreateToken(ctx context.Context, token *RefreshToken) error
	GetToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeToken(ctx context.Context, tokenHash string) error
	RevokeUserTokens(ctx context.Context, tenantID int64, userID uuid.UUID, exceptTokenHash *string) error
	DeleteExpiredTokens(ctx context.Context, tenantID int64) error
}

type refreshTokenRepository struct {
	client *postgres.Client
}

// NewRefreshTokenRepository creates a new refresh token repository.
func NewRefreshTokenRepository(client *postgres.Client) RefreshTokenRepository {
	return &refreshTokenRepository{client: client}
}

// HashToken hashes a refresh token using SHA256
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (r *refreshTokenRepository) CreateToken(ctx context.Context, token *RefreshToken) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("refresh: open master db: %w", err)
	}

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	query := `
		INSERT INTO auth_refresh_token (id, tenant_id, user_id, token_hash, client_id, device_id, created_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = db.Exec(query,
		token.ID,
		token.TenantID,
		token.UserID,
		token.TokenHash,
		token.ClientID,
		token.DeviceID,
		token.CreatedAt,
		token.ExpiresAt,
		token.RevokedAt,
	)

	return err
}

func (r *refreshTokenRepository) GetToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("refresh: open master db: %w", err)
	}

	query := `
		SELECT id, tenant_id, user_id, token_hash, client_id, device_id, created_at, expires_at, revoked_at
		FROM auth_refresh_token
		WHERE token_hash = $1
	`

	var token RefreshToken
	err = db.QueryRow(query, tokenHash).Scan(
		&token.ID,
		&token.TenantID,
		&token.UserID,
		&token.TokenHash,
		&token.ClientID,
		&token.DeviceID,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.RevokedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}

	// Check expiration
	if time.Now().After(token.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	// Check if revoked
	if token.RevokedAt != nil {
		return nil, ErrRefreshTokenRevoked
	}

	return &token, nil
}

func (r *refreshTokenRepository) RevokeToken(ctx context.Context, tokenHash string) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("refresh: open master db: %w", err)
	}

	query := `
		UPDATE auth_refresh_token
		SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`

	result, err := db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRefreshTokenNotFound
	}

	return nil
}

func (r *refreshTokenRepository) RevokeUserTokens(ctx context.Context, tenantID int64, userID uuid.UUID, exceptTokenHash *string) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("refresh: open master db: %w", err)
	}

	var query string
	var args []interface{}

	if exceptTokenHash != nil {
		query = `
			UPDATE auth_refresh_token
			SET revoked_at = NOW()
			WHERE tenant_id = $1 AND user_id = $2 AND revoked_at IS NULL AND token_hash != $3
		`
		args = []interface{}{tenantID, userID, *exceptTokenHash}
	} else {
		query = `
			UPDATE auth_refresh_token
			SET revoked_at = NOW()
			WHERE tenant_id = $1 AND user_id = $2 AND revoked_at IS NULL
		`
		args = []interface{}{tenantID, userID}
	}

	_, err = db.Exec(query, args...)
	return err
}

func (r *refreshTokenRepository) DeleteExpiredTokens(ctx context.Context, tenantID int64) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("refresh: open master db: %w", err)
	}

	query := `DELETE FROM auth_refresh_token WHERE tenant_id = $1 AND expires_at < NOW()`
	_, err = db.Exec(query, tenantID)
	return err
}
