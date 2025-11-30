package authhandler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/firstkb/sc-api/cmd/scapi/internal/authsvc"
	"github.com/firstkb/sc-api/internal/httpx/apperr"
)

type Handler struct {
	service *authsvc.AuthService
}

func New(service *authsvc.AuthService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RequestOTP(ctx context.Context, r *http.Request, req authsvc.OTPRequest) (any, error) {
	req.IP = extractIP(r)
	req.UserAgent = r.Header.Get("User-Agent")

	if err := h.service.RequestOTP(ctx, req, r); err != nil {
		return nil, mapError(err)
	}

	return map[string]string{"status": "ok"}, nil
}

func (h *Handler) VerifyOTP(ctx context.Context, r *http.Request, req authsvc.OTPVerifyRequest) (*authsvc.TokenResponse, error) {
	req.IP = extractIP(r)
	req.UserAgent = r.Header.Get("User-Agent")

	resp, err := h.service.VerifyOTP(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return resp, nil
}

func (h *Handler) Refresh(ctx context.Context, r *http.Request, req authsvc.RefreshRequest) (*authsvc.TokenResponse, error) {
	req.IP = extractIP(r)
	req.UserAgent = r.Header.Get("User-Agent")

	resp, err := h.service.Refresh(ctx, req)
	if err != nil {
		return nil, mapError(err)
	}
	return resp, nil
}

func (h *Handler) Logout(ctx context.Context, r *http.Request, req authsvc.LogoutRequest) (any, error) {
	if err := h.service.Logout(ctx, req); err != nil {
		return nil, mapError(err)
	}
	return map[string]string{"status": "ok"}, nil
}

func (h *Handler) CleanupExpiredOTPs(ctx context.Context, _ *http.Request, _ authsvc.CleanupExpiredTokensRequest) (any, error) {
	if err := h.service.CleanupExpiredOTPs(ctx); err != nil {
		return nil, mapError(err)
	}
	return map[string]string{"status": "ok"}, nil
}

func (h *Handler) CleanupExpiredRefreshTokens(ctx context.Context, _ *http.Request, _ authsvc.CleanupExpiredRefreshTokensRequest) (any, error) {
	if err := h.service.CleanupExpiredRefreshTokens(ctx); err != nil {
		return nil, mapError(err)
	}
	return map[string]string{"status": "ok"}, nil
}

func mapError(err error) *apperr.AppError {
	switch {
	case errors.Is(err, authsvc.ErrRateLimited):
		return apperr.New("AUTH_RATE_LIMIT", http.StatusTooManyRequests, "rate limit exceeded")
	case errors.Is(err, authsvc.ErrInvalidInput):
		return apperr.New("AUTH_INVALID_INPUT", http.StatusBadRequest, "invalid input")
	case errors.Is(err, authsvc.ErrTenantMissing):
		return apperr.New("AUTH_TENANT_MISSING", http.StatusBadRequest, "tenant not found")
	case errors.Is(err, authsvc.ErrUserNotFound):
		return apperr.New("AUTH_USER_NOT_FOUND", http.StatusNotFound, "user not found")
	case errors.Is(err, authsvc.ErrUserInactive):
		return apperr.New("AUTH_USER_INACTIVE", http.StatusForbidden, "user inactive")
	case errors.Is(err, authsvc.ErrOTPInvalid):
		return apperr.New("AUTH_OTP_INVALID", http.StatusForbidden, "invalid code")
	case errors.Is(err, authsvc.ErrUnauthorized):
		return apperr.New("AUTH_UNAUTHORIZED", http.StatusUnauthorized, "unauthorized")
	default:
		return apperr.New("AUTH_INTERNAL", http.StatusInternalServerError, "internal error")
	}
}

func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
