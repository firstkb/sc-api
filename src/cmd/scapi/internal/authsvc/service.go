package authsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/firstkb/sc-api/cmd/scapi/internal/eventsvc"
	"github.com/firstkb/sc-api/cmd/scapi/internal/notifysvc"
	"github.com/firstkb/sc-api/cmd/scapi/internal/tenantsvc"
	"github.com/firstkb/sc-api/internal/auth"
	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/httpx/requestctx"
	"github.com/firstkb/sc-api/internal/identity"
	"github.com/firstkb/sc-api/internal/postgres"
)

var (
	ErrRateLimited   = errors.New("rate limit exceeded")
	ErrInvalidInput  = errors.New("invalid input")
	ErrTenantMissing = errors.New("tenant not found")
	ErrUserNotFound  = errors.New("user not found")
	ErrUserInactive  = errors.New("user inactive")
	ErrOTPInvalid    = errors.New("invalid otp code")
	ErrUnauthorized  = errors.New("unauthorized")
)

type AuthService struct {
	otpRepo        auth.OTPRepository
	refreshRepo    auth.RefreshTokenRepository
	membershipRepo auth.MembershipRepository
	jwtIssuer      auth.JWTIssuer
	notifySvc      *notifysvc.NotifyService
	eventSvc       *eventsvc.EventService
	rateLimiter    auth.RateLimiter
	identitySvc    *identity.Service
	tenants        *tenantsvc.ServiceTenantProvider
	otpTTL         time.Duration
	otpLength      int
	refreshTTL     time.Duration
	accessTTL      time.Duration
	audience       string
	issuer         string
	logger         *slog.Logger
	jwksTTL        time.Duration
}

type Config struct {
	Auth AuthConfig `json:"auth"`
}

type AuthConfig struct {
	OTPTTL            string `json:"otpttl"`
	OTPLength         int    `json:"otplength"`
	RefreshTTL        string `json:"refreshttl"`
	AccessTTL         string `json:"accessttl"`
	Audience          string `json:"audience"`
	Issuer            string `json:"issuer"`
	JWTAlgorithm      string `json:"jwtalg"`
	JWTPrivateKeyPath string `json:"jwtprivatepempath"`
	JWTPublicKeyPath  string `json:"jwtpublicpempath"`
	JWKSCacheTTL      string `json:"jwkscachettl"`
	RateLimitIP       string `json:"ratelimitip"`
}

type OTPRequest struct {
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	IP        string
	UserAgent string
}

type OTPVerifyRequest struct {
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	Code      string  `json:"code"`
	IP        string
	UserAgent string
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	IP           string
	UserAgent    string
}

type LogoutRequest struct {
	AllDevices bool `json:"all_devices"`
}

func NewService(sqlClient *postgres.Client, cfg *config.Config, logger *slog.Logger, tenants *tenantsvc.ServiceTenantProvider) (*AuthService, error) {

	var c Config
	_ = cfg.Unmarshal("", &c)

	notifySvc, err := notifysvc.NewService(sqlClient, cfg, logger)
	if err != nil {
		return nil, err
	}

	eventSvc, err := eventsvc.NewService(sqlClient, cfg, logger)
	if err != nil {
		return nil, err
	}

	identitySvc, err := identity.NewService(sqlClient, cfg, logger)
	if err != nil {
		return nil, err
	}

	otpRepo := auth.NewOTPRepository(sqlClient)
	refreshRepo := auth.NewRefreshTokenRepository(sqlClient)
	membershipRepo := auth.NewMembershipRepository(sqlClient)

	jwtCfg := auth.JWTConfig{
		Algorithm:      strings.ToLower(c.Auth.JWTAlgorithm),
		PrivateKeyPath: c.Auth.JWTPrivateKeyPath,
		PublicKeyPath:  c.Auth.JWTPublicKeyPath,
		Issuer:         c.Auth.Audience,
		Audience:       c.Auth.Audience,
	}
	jwtIssuer, err := auth.NewJWTIssuer(jwtCfg)
	if err != nil {
		return nil, fmt.Errorf("auth: init issuer: %w", err)
	}

	rlCfg := parseRateLimit(c.Auth.RateLimitIP, 30, time.Minute)
	rateLimiter := auth.NewRateLimiter(rlCfg)

	return &AuthService{
		otpRepo:        otpRepo,
		refreshRepo:    refreshRepo,
		membershipRepo: membershipRepo,
		jwtIssuer:      jwtIssuer,
		notifySvc:      notifySvc,
		eventSvc:       eventSvc,
		rateLimiter:    rateLimiter,
		identitySvc:    identitySvc,
		tenants:        tenants,
		otpTTL:         parseDuration(c.Auth.OTPTTL, 10*time.Minute),
		otpLength:      c.Auth.OTPLength,
		refreshTTL:     parseDuration(c.Auth.RefreshTTL, 30*24*time.Hour),
		accessTTL:      parseDuration(c.Auth.AccessTTL, 15*time.Minute),
		audience:       c.Auth.Audience,
		issuer:         c.Auth.Issuer,
		jwksTTL:        parseDuration(c.Auth.JWKSCacheTTL, 10*time.Minute),
		logger:         logger,
	}, nil
}

func (s *AuthService) NewJWKSEndpoint() *auth.JWKSEndpoint {
	return auth.NewJWKSEndpoint(s.jwtIssuer, auth.NewJWKSCache(), s.jwksTTL)
}

func (s *AuthService) RequestOTP(ctx context.Context, req OTPRequest, r *http.Request) error {
	if req.Email == nil && req.Phone == nil {
		return ErrInvalidInput
	}

	if req.IP != "" {
		if ok, err := s.rateLimiter.Check("ip:" + req.IP); err != nil || !ok {
			return ErrRateLimited
		}
	}

	channel, address := resolveChannel(req.Email, req.Phone)
	if address == "" {
		return ErrInvalidInput
	}

	if ok, err := s.rateLimiter.Check("addr:" + address); err != nil || !ok {
		return ErrRateLimited
	}

	membership, tenantCtx, tenantInfo, err := s.resolveMembershipByContact(ctx, channel, address)
	if err != nil {
		return err
	}
	if !isMembershipActive(membership.Status) {
		return ErrUserInactive
	}

	tenantID := membership.TenantID

	code, err := auth.GenerateOTP(s.otpLength)
	if err != nil {
		return err
	}

	hash, err := auth.HashOTP(code)
	if err != nil {
		return err
	}

	_ = s.otpRepo.DeleteActiveOTPs(ctx, tenantID, auth.OTPChannel(channel), address)

	otp := &auth.OTP{
		TenantID:  tenantID,
		Channel:   auth.OTPChannel(channel),
		Address:   address,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(s.otpTTL),
		CreatedAt: time.Now(),
	}
	if err := s.otpRepo.CreateOTP(ctx, otp); err != nil {
		return err
	}

	data := map[string]any{
		"code": code,
		"ttl":  int(s.otpTTL.Minutes()),
	}

	if channel == "email" {
		_, err = s.notifySvc.SendEmail(tenantCtx, tenantInfo.ID, notifysvc.EmailRequest{
			To:       []string{address},
			Template: "otp",
			Locale:   "default",
			Data:     data,
		})
	} else {
		_, err = s.notifySvc.SendSMS(tenantCtx, tenantInfo.ID, notifysvc.SMSRequest{
			To:       address,
			Template: "otp",
			Locale:   "default",
			Data:     data,
		})
	}
	if err != nil {
		s.logger.Warn("notify send failed", "error", err)
	}

	if s.eventSvc != nil {
		eventData := eventsvc.EventData{
			"channel": channel,
			"address": maskAddress(channel, address),
			"code":    code,
		}
		_ = s.eventSvc.Log(tenantCtx, eventsvc.EventTypeOTPRequest, r, eventData)
	}

	return nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, req OTPVerifyRequest) (*TokenResponse, error) {
	if req.Code == "" {
		return nil, ErrInvalidInput
	}

	channel, address := resolveChannel(req.Email, req.Phone)
	if address == "" {
		return nil, ErrInvalidInput
	}

	membership, tenantCtx, _, err := s.resolveMembershipByContact(ctx, channel, address)
	if err != nil {
		return nil, err
	}
	if !isMembershipActive(membership.Status) {
		return nil, ErrUserInactive
	}

	tenantID := membership.TenantID

	otp, err := s.otpRepo.GetOTP(ctx, tenantID, auth.OTPChannel(channel), address)
	if err != nil {
		return nil, ErrOTPInvalid
	}

	valid, err := auth.VerifyOTP(req.Code, otp.CodeHash)
	if err != nil || !valid {
		_ = s.otpRepo.IncrementAttempts(ctx, otp.ID)
		return nil, ErrOTPInvalid
	}

	_ = s.otpRepo.DeleteOTP(ctx, otp.ID)

	userUUID, err := uuid.Parse(membership.TenantUserID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	email := ""
	if req.Email != nil {
		email = strings.TrimSpace(*req.Email)
	}
	phone := ""
	if req.Phone != nil {
		phone = strings.TrimSpace(*req.Phone)
	}

	claims := auth.NewJWTClaims(s.issuer, s.audience, userUUID, tenantID, email, phone, membership.Level, string(membership.Role))
	claims.ExpiresAt = time.Now().Add(s.accessTTL).Unix()

	accessToken, err := s.jwtIssuer.IssueToken(claims)
	if err != nil {
		return nil, err
	}

	refreshToken := generateRefreshToken()
	refreshRecord := &auth.RefreshToken{
		TenantID:  tenantID,
		UserID:    userUUID,
		TokenHash: auth.HashToken(refreshToken),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if err := s.refreshRepo.CreateToken(ctx, refreshRecord); err != nil {
		return nil, err
	}

	if s.eventSvc != nil {
		_ = s.eventSvc.Log(tenantCtx, eventsvc.EventTypeOTPVerify, nil, eventsvc.EventData{
			"channel": channel,
		})
		_ = s.eventSvc.Log(tenantCtx, eventsvc.EventTypeLogin, nil, eventsvc.EventData{
			"method": "otp",
		})
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, req RefreshRequest) (*TokenResponse, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, ErrInvalidInput
	}

	token, err := s.refreshRepo.GetToken(ctx, auth.HashToken(req.RefreshToken))
	if err != nil {
		return nil, ErrUnauthorized
	}

	_ = s.refreshRepo.RevokeToken(ctx, auth.HashToken(req.RefreshToken))

	membership, err := s.membershipRepo.GetMembership(ctx, token.UserID.String(), token.TenantID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if !isMembershipActive(membership.Status) {
		return nil, ErrUserInactive
	}

	tenantCtx, _, terr := s.ensureTenantContext(ctx, membership.TenantID)
	if terr != nil {
		s.logger.Warn("refresh: tenant context fallback", "error", terr)
		tenantCtx = ctx
	}

	var claimEmail string
	if claims, ok := requestctx.Claims(ctx); ok {
		claimEmail = claims.Email
	}

	claims := auth.NewJWTClaims(s.issuer, s.audience, token.UserID, token.TenantID, claimEmail, "", membership.Level, string(membership.Role))
	claims.ExpiresAt = time.Now().Add(s.accessTTL).Unix()

	accessToken, err := s.jwtIssuer.IssueToken(claims)
	if err != nil {
		return nil, err
	}

	newRefresh := generateRefreshToken()
	refreshRecord := &auth.RefreshToken{
		TenantID:  token.TenantID,
		UserID:    token.UserID,
		TokenHash: auth.HashToken(newRefresh),
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if err := s.refreshRepo.CreateToken(ctx, refreshRecord); err != nil {
		return nil, err
	}

	if s.eventSvc != nil {
		_ = s.eventSvc.Log(tenantCtx, eventsvc.EventTypeTokenRefresh, nil, nil)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req LogoutRequest) error {
	claims, ok := requestctx.Claims(ctx)
	if !ok || claims.UserID == "" {
		return ErrUnauthorized
	}

	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return ErrUnauthorized
	}

	tenantID := int64(0)
	if claims.TenantID != "" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(claims.TenantID), 10, 64); err == nil {
			tenantID = parsed
		}
	}

	if tenantID == 0 {
		return ErrTenantMissing
	}

	_ = s.refreshRepo.RevokeUserTokens(ctx, tenantID, userUUID, nil)

	if s.eventSvc != nil {
		_ = s.eventSvc.Log(ctx, eventsvc.EventTypeLogout, nil, eventsvc.EventData{
			"all_devices": req.AllDevices,
		})
	}

	return nil
}

func (s *AuthService) CleanupExpiredOTPs(ctx context.Context) error {
	_, tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.otpRepo.DeleteExpiredOTPs(ctx, tenantID)
}

func (s *AuthService) CleanupExpiredRefreshTokens(ctx context.Context) error {
	_, tenantID, err := tenantFromContext(ctx)
	if err != nil {
		return err
	}
	return s.refreshRepo.DeleteExpiredTokens(ctx, tenantID)
}

func (s *AuthService) ValidateToken(token string) (*auth.JWTClaims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalidInput
	}
	return s.jwtIssuer.ValidateToken(token)
}

func (s *AuthService) resolveMembershipByContact(ctx context.Context, channel, address string) (*auth.Membership, context.Context, requestctx.TenantInfo, error) {
	var subjectType identity.SubjectType
	switch channel {
	case "email":
		subjectType = identity.SubjectTypeEmail
	case "sms":
		subjectType = identity.SubjectTypePhone
	default:
		return nil, ctx, requestctx.TenantInfo{}, ErrInvalidInput
	}

	subject, err := s.identitySvc.FindSubject(ctx, subjectType, address)
	if err != nil {
		return nil, ctx, requestctx.TenantInfo{}, fmt.Errorf("identity lookup: %w", err)
	}
	if subject == nil {
		return nil, ctx, requestctx.TenantInfo{}, ErrUserNotFound
	}

	memberships, err := s.identitySvc.ListMemberships(ctx, subject.ID)
	if err != nil {
		return nil, ctx, requestctx.TenantInfo{}, fmt.Errorf("identity membership list: %w", err)
	}

	var selected *identity.Membership
	for _, m := range memberships {
		if isMembershipActive(m.Status) {
			selected = m
			break
		}
	}
	if selected == nil {
		return nil, ctx, requestctx.TenantInfo{}, ErrUserInactive
	}

	tenantCtx, tenantInfo, err := s.ensureTenantContext(ctx, selected.TenantID)
	if err != nil {
		return nil, ctx, requestctx.TenantInfo{}, err
	}

	return &auth.Membership{
		TenantUserID: selected.TenantUserID,
		TenantID:     selected.TenantID,
		IdentityID:   selected.IdentityID,
		Role:         auth.Role(selected.Role),
		Level:        auth.AccessLevel(selected.Level),
		Status:       selected.Status,
	}, tenantCtx, tenantInfo, nil
}

func (s *AuthService) ensureTenantContext(ctx context.Context, tenantID int64) (context.Context, requestctx.TenantInfo, error) {
	if tenantID == 0 {
		return ctx, requestctx.TenantInfo{}, ErrTenantMissing
	}

	tenantIDStr := strconv.FormatInt(tenantID, 10)
	tenant, err := s.tenants.GetByID(ctx, tenantIDStr)
	if err != nil {
		return ctx, requestctx.TenantInfo{}, fmt.Errorf("tenant lookup: %w", err)
	}

	info := requestctx.TenantInfo{
		ID:             tenant.ID,
		Host:           tenant.Host,
		Status:         tenant.Status,
		Plan:           tenant.Plan,
		DBName:         tenant.DBName,
		DBInstanceID:   tenant.DBInstanceID,
		DBInstanceCode: tenant.DBInstanceCode,
	}
	return requestctx.WithTenant(ctx, info), info, nil
}
