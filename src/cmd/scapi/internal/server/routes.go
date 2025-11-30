package server

import (
	"context"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/authsvc"
	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/httpx/apperr"
	"github.com/firstkb/sc-api/internal/httpx/handler"
	"github.com/firstkb/sc-api/internal/httpx/router"
)

func (srv *Server) buildRoutes() (*http.ServeMux, *router.Classifier) {
	b := router.NewBuilder()

	b.Handle("HEALTH_STATUS", "GET", "/status", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("OK\n"))
		}))

	b.Handle("HEALTH_LIVE", "GET", "/healthz", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("ok\n"))
		}))

	b.Handle("HEALTH_READY", "GET", "/readyz", router.TierHealth,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("ready\n"))
		}))

	b.Handle("PING_GET", "GET", "/ping", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, r *http.Request, in any) (*pingsvc.Ping, error) {
			info, err := srv.pingsvc.GetPing(ctx)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "PING_GET",
					http.StatusInternalServerError, "cannot get ping", err, srv.FieldsForLog(ctx, r, in)...)
			}
			return info, nil
		}, srv.logger))

	b.Handle("SURVEY_GET", "GET", "/survey/{code}", router.TierPublicTenant,
		handler.HandleJson(func(ctx context.Context, r *http.Request, in any) (string, error) {
			code := r.PathValue("code")
			srv.logger.Info("SURVEY_GET: code", "code", code)
			return code, nil
		}, srv.logger))

	// AUTH ROUTES
	jwksHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if srv.jwksEndpoint != nil {
			jwks, err := srv.jwksEndpoint.GetJWKS()
			if err != nil {
				http.Error(w, "failed to get JWKS", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jwks)
			if err != nil {
				http.Error(w, "failed to write JWKS", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "JWKS not available", http.StatusServiceUnavailable)
		}
	})

	b.Handle("JWKS_GET", "GET", "/.well-known/jwks.json", router.TierHealth, jwksHandler)
	b.Handle("JWKS2_GET", "GET", "/auth/jwks.json", router.TierHealth, jwksHandler)
	b.Handle("OTP_REQUEST", "POST", "/auth/otp/request", router.TierPublic,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.OTPRequest) (any, error) {
			info, err := srv.authHTTP.RequestOTP(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "OTP_REQUEST",
					http.StatusInternalServerError, "cannot request OTP", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))
	b.Handle("OTP_VERIFY", "POST", "/auth/otp/verify", router.TierPublic,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.OTPVerifyRequest) (*authsvc.TokenResponse, error) {
			info, err := srv.authHTTP.VerifyOTP(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "OTP_VERIFY",
					http.StatusInternalServerError, "cannot verify OTP", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))
	b.Handle("REFRESH", "POST", "/auth/refresh", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.RefreshRequest) (*authsvc.TokenResponse, error) {
			info, err := srv.authHTTP.Refresh(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "REFRESH",
					http.StatusInternalServerError, "cannot refresh", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))
	b.Handle("LOGOUT", "POST", "/auth/logout", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.LogoutRequest) (any, error) {
			info, err := srv.authHTTP.Logout(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "LOGOUT",
					http.StatusInternalServerError, "cannot logout", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))

	// TEMPORARY ROUTES FOR ADMINISTRATION
	// TODO: Remove these routes after administration is implemented
	b.Handle("CLEANUP_EXPIRED_OTP_CODES", "POST", "/admin/cleanup/expired-otp-codes", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.CleanupExpiredTokensRequest) (any, error) {
			info, err := srv.authHTTP.CleanupExpiredOTPs(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "CLEANUP_EXPIRED_OTP_CODES",
					http.StatusInternalServerError, "cannot cleanup expired otp codes", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))
	b.Handle("CLEANUP_EXPIRED_REFRESH_TOKENS", "POST", "/admin/cleanup/expired-refresh-tokens", router.TierSecure,
		handler.HandleJson(func(ctx context.Context, r *http.Request, req authsvc.CleanupExpiredRefreshTokensRequest) (any, error) {
			info, err := srv.authHTTP.CleanupExpiredRefreshTokens(ctx, r, req)
			if err != nil {
				return nil, apperr.WrapAndLog(srv.logger, ctx, "CLEANUP_EXPIRED_REFRESH_TOKENS",
					http.StatusInternalServerError, "cannot cleanup expired refresh tokens", err, srv.FieldsForLog(ctx, r, req)...)
			}
			return info, nil
		}, srv.logger))

	return b.Mux(), b.Classifier()
}
