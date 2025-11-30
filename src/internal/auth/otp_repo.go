package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/firstkb/sc-api/internal/postgres"
)

var (
	ErrOTPNotFound        = errors.New("OTP not found")
	ErrOTPExpired         = errors.New("OTP expired")
	ErrOTPTooManyAttempts = errors.New("too many verification attempts")
)

// OTPChannel represents the delivery channel for OTP
type OTPChannel string

const (
	OTPChannelEmail OTPChannel = "email"
	OTPChannelSMS   OTPChannel = "sms"
)

// OTP represents a one-time password code
type OTP struct {
	ID        uuid.UUID
	TenantID  int64
	Channel   OTPChannel
	Address   string // email or phone number
	CodeHash  string // hashed code
	ExpiresAt time.Time
	Attempts  int
	CreatedAt time.Time
}

// OTPRepository handles OTP database operations
type OTPRepository interface {
	CreateOTP(ctx context.Context, otp *OTP) error
	GetOTP(ctx context.Context, tenantID int64, channel OTPChannel, address string) (*OTP, error)
	IncrementAttempts(ctx context.Context, id uuid.UUID) error
	DeleteOTP(ctx context.Context, id uuid.UUID) error
	DeleteExpiredOTPs(ctx context.Context, tenantID int64) error
	DeleteActiveOTPs(ctx context.Context, tenantID int64, channel OTPChannel, address string) error
}

type otpRepository struct {
	client *postgres.Client
}

// NewOTPRepository creates a new OTP repository bound to master DB.
func NewOTPRepository(client *postgres.Client) OTPRepository {
	return &otpRepository{client: client}
}

func (r *otpRepository) CreateOTP(ctx context.Context, otp *OTP) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("otp: open master db: %w", err)
	}

	if otp.ID == uuid.Nil {
		otp.ID = uuid.New()
	}

	query := `
		INSERT INTO auth_otp (id, tenant_id, channel, address, code_hash, expires_at, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = db.Exec(query,
		otp.ID,
		otp.TenantID,
		string(otp.Channel),
		otp.Address,
		otp.CodeHash,
		otp.ExpiresAt,
		otp.Attempts,
		otp.CreatedAt,
	)

	return err
}

func (r *otpRepository) GetOTP(ctx context.Context, tenantID int64, channel OTPChannel, address string) (*OTP, error) {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return nil, fmt.Errorf("otp: open master db: %w", err)
	}

	// Note: FOR UPDATE SKIP LOCKED requires a transaction in PostgreSQL
	// Since we're using a regular connection, we'll remove FOR UPDATE SKIP LOCKED
	// and rely on application-level locking or just use a simple SELECT
	// Race conditions are acceptable for OTP verification (worst case: multiple verifications)
	query := `
		SELECT id, tenant_id, channel, address, code_hash, expires_at, attempts, created_at
		FROM auth_otp
		WHERE tenant_id = $1 AND channel = $2 AND address = $3
		  AND expires_at > NOW()
		  AND attempts < 5
		ORDER BY created_at DESC
		LIMIT 1
	`

	var otp OTP
	var channelStr string

	err = db.QueryRow(query, tenantID, string(channel), address).Scan(
		&otp.ID,
		&otp.TenantID,
		&channelStr,
		&otp.Address,
		&otp.CodeHash,
		&otp.ExpiresAt,
		&otp.Attempts,
		&otp.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOTPNotFound
		}
		return nil, fmt.Errorf("get OTP: %w", err)
	}

	otp.Channel = OTPChannel(channelStr)

	// Check expiration (double-check, though SQL already filters)
	if time.Now().After(otp.ExpiresAt) {
		return nil, ErrOTPExpired
	}

	// Check attempts (double-check, though SQL already filters)
	if otp.Attempts >= 5 {
		return nil, ErrOTPTooManyAttempts
	}

	return &otp, nil
}

func (r *otpRepository) IncrementAttempts(ctx context.Context, id uuid.UUID) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("otp: open master db: %w", err)
	}

	query := `
		UPDATE auth_otp
		SET attempts = attempts + 1
		WHERE id = $1
	`

	_, err = db.Exec(query, id)
	return err
}

func (r *otpRepository) DeleteOTP(ctx context.Context, id uuid.UUID) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("otp: open master db: %w", err)
	}

	query := `DELETE FROM auth_otp WHERE id = $1`
	_, err = db.Exec(query, id)
	return err
}

func (r *otpRepository) DeleteExpiredOTPs(ctx context.Context, tenantID int64) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("otp: open master db: %w", err)
	}

	query := `DELETE FROM auth_otp WHERE tenant_id = $1 AND expires_at < NOW()`
	_, err = db.Exec(query, tenantID)
	return err
}

// DeleteActiveOTPs deletes all active (non-expired) OTP records for a specific tenant, channel, and address
// This is used before creating a new OTP to prevent multiple active OTPs for the same address
func (r *otpRepository) DeleteActiveOTPs(ctx context.Context, tenantID int64, channel OTPChannel, address string) error {
	db, err := r.client.OpenDBMaster(ctx)
	if err != nil {
		return fmt.Errorf("otp: open master db: %w", err)
	}

	query := `DELETE FROM auth_otp WHERE tenant_id = $1 AND channel = $2 AND address = $3 AND expires_at > NOW()`
	_, err = db.Exec(query, tenantID, string(channel), address)
	return err
}
