package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/sqlserver"

	"github.com/firstkb/sc-api/internal/config"
)

type Config struct {
	HostApp string `json:"hostapp"`
	Origin  string `json:"origin"`
	DbHost  string `json:"host"`
	DbPort  string `json:"port"`
	DbUser  string `json:"username"`
	DbPass  string `json:"password"`
	DbName  string `json:"dbname"`
}

type Server struct {
	config     *Config
	logger     *slog.Logger
	httpServer *http.Server
	sqlClient  *sqlserver.Client
	pingsvc    *pingsvc.PingService
}

func NewServer(config *config.Config, logger *slog.Logger) (*Server, error) {
	var c Config
	config.Unmarshal("", &c)

	server := &Server{
		logger: logger,
		config: &c,
	}

	if err := server.initialize(); err != nil {
		return nil, err
	}

	handler := server.addRoutes()
	handler = server.addMiddleware(handler, c.Origin)

	server.httpServer = &http.Server{
		Addr:    server.config.HostApp,
		Handler: handler,
	}

	pingsvc, err := pingsvc.NewService(server.sqlClient, config, logger)
	if err != nil {
		return nil, err
	}

	server.pingsvc = pingsvc

	return server, nil
}

func (srv *Server) Run(ctx context.Context) {
	srv.logger.Info(fmt.Sprintf("API server started, listening on %s", srv.httpServer.Addr))

	if err := srv.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		srv.logger.Error(fmt.Sprintf("Error listening and serving: %s", err))
	}

	ctx.Done()
}

func (srv *Server) Stop(ctx context.Context) {
	if srv.httpServer == nil {
		return
	}

	if err := srv.httpServer.Shutdown(ctx); err != nil {
		srv.logger.Error(fmt.Sprintf("Error shutting down API server: %s", err))
	}
}

func (srv *Server) initialize() error {
	if srv.config.DbHost == "" || srv.config.DbPort == "" || srv.config.DbName == "" || srv.config.DbUser == "" || srv.config.DbPass == "" {
		return errors.New("database connection vars must be specified")
	}

	conn := fmt.Sprintf("server=%s;port=%s;database=%s;user id=%s;password=%s;", srv.config.DbHost, srv.config.DbPort, srv.config.DbName, srv.config.DbUser, srv.config.DbPass)

	client, err := sqlserver.NewClient(conn, srv.logger)
	if err != nil {
		return fmt.Errorf("failed to create a SQL Server client: %v", err)
	}

	srv.sqlClient = client

	return nil
}
