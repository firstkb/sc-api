package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/postgres"
)

type Config struct {
	Identity IdentityConfig `json:"identity"`
}

type IdentityConfig struct {
	HMACKey string `json:"hmackey"`
	HMACKID int16  `json:"hmackid"`
}

type Service struct {
	repository *Repository
	logger     *slog.Logger
	hmacKey    []byte
	hmacKID    int16
}

var (
	ErrInvalidSubjectType = errors.New("invalid subject type")
	ErrEmptyValue         = errors.New("identity value cannot be empty")
)

func NewService(sqlClient *postgres.Client, cfg *config.Config, logger *slog.Logger) (*Service, error) {
	var c Config
	_ = cfg.Unmarshal("", &c)

	keyBytes, err := decodeKey(c.Identity.HMACKey)
	if err != nil {
		return nil, fmt.Errorf("decode identity hmac key: %w", err)
	}
	if len(keyBytes) == 0 {
		return nil, errors.New("identity hmac key must not be empty")
	}

	repo := NewRepository(sqlClient)
	return &Service{
		repository: repo,
		logger:     logger,
		hmacKey:    keyBytes,
		hmacKID:    c.Identity.HMACKID,
	}, nil
}

func (s *Service) FindOrCreateSubject(ctx context.Context, subjectType SubjectType, rawValue string, isPrimary bool) (*Subject, error) {
	normalized, err := normalizeSubjectValue(subjectType, rawValue)
	if err != nil {
		return nil, err
	}

	hash := s.computeHMAC(normalized)

	subj, err := s.repository.FindSubject(ctx, subjectType, hash)
	if err != nil {
		return nil, err
	}
	if subj != nil {
		return subj, nil
	}

	subj = &Subject{
		Type:      subjectType,
		HMAC:      hash,
		HMACKID:   s.hmacKID,
		IsPrimary: isPrimary,
		Status:    "active",
	}
	return s.repository.CreateSubject(ctx, subj)
}

func (s *Service) FindSubject(ctx context.Context, subjectType SubjectType, rawValue string) (*Subject, error) {
	normalized, err := normalizeSubjectValue(subjectType, rawValue)
	if err != nil {
		return nil, err
	}

	hash := s.computeHMAC(normalized)
	return s.repository.FindSubject(ctx, subjectType, hash)
}

func (s *Service) GrantAccess(ctx context.Context, tenantID int64, tenantUserID string, subjectType SubjectType, contactValue string, role string, level int, status string) (*Membership, error) {
	subj, err := s.FindOrCreateSubject(ctx, subjectType, contactValue, false)
	if err != nil {
		return nil, err
	}

	membership := &Membership{
		TenantID:     tenantID,
		IdentityID:   subj.ID,
		TenantUserID: tenantUserID,
		Role:         role,
		Level:        level,
		Status:       status,
	}
	return s.repository.UpsertMembership(ctx, membership)
}

func (s *Service) RevokeAccess(ctx context.Context, tenantID int64, identityID int64, tenantUserID string) error {
	return s.repository.DeleteMembership(ctx, tenantID, identityID, tenantUserID)
}

func (s *Service) ListMemberships(ctx context.Context, identityID int64) ([]*Membership, error) {
	return s.repository.ListMembershipsByIdentity(ctx, identityID)
}

func (s *Service) computeHMAC(value string) []byte {
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func decodeKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return []byte(raw), nil
}
