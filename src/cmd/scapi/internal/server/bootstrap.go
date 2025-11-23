package server

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	appmw "github.com/firstkb/sc-api/cmd/scapi/internal/server/middleware"
	"github.com/firstkb/sc-api/cmd/scapi/internal/tenantsvc"
	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/httpx/mw"
)

func Bootstrap(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config must not be nil")
	}

	var c Config
	if err := cfg.Unmarshal("", &c); err != nil {
		return nil, err
	}

	server := &Server{
		logger: logger,
		config: &c,
	}

	if err := server.initialize(); err != nil {
		return nil, err
	}

	ps, err := pingsvc.NewService(server.sqlClient, cfg, logger)
	if err != nil {
		return nil, err
	}
	server.pingsvc = ps

	tenant, err := tenantsvc.NewService(server.sqlClient, cfg, logger)
	if err != nil {
		return nil, err
	}
	server.tenants = tenant

	mux, class := server.buildRoutes()
	server.classifier = class
	server.handler = server.buildHTTPHandler(mux)

	return server, nil
}

func (srv *Server) buildHTTPHandler(mux http.Handler) http.Handler {
	handler := appmw.Recover(srv.logger)(mux)
	handler = appmw.RequestID()(handler)
	handler = appmw.Timeout(time.Duration(srv.config.Timeout) * time.Second)(handler)
	handler = appmw.AccessLog(srv.logger, srv.config.MW.AccessLog)(handler)
	handler = appmw.TenantGuard(srv.logger, srv.tenants)(handler)
	handler = appmw.Claims(srv.logger, srv.config.Token.Provider)(handler)

	if srv.config.Origin != "" {
		corsCfg := appmw.CORSConfig{
			AllowedOrigins:   []string{srv.config.Origin},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With"},
			AllowCredentials: true,
			Debug:            false,
		}
		handler = appmw.CORS(srv.logger, corsCfg)(handler)
	}

	return mw.Classifier(srv.classifier)(handler)
}

func (srv *Server) Handler() http.Handler {
	return srv.handler
}
