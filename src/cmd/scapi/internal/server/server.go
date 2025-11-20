package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/migrate"
	"github.com/firstkb/sc-api/internal/sqlserver"

	"github.com/firstkb/sc-api/internal/config"
)

type Config struct {
	HostApp string         `json:"hostapp"`
	Origin  string         `json:"origin"`
	DB      DatabaseConfig `json:"db"`
}

type DatabaseConfig struct {
	Host       string `json:"host"`
	Port       string `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	MasterName string `json:"mastername"`
	SSLMode    string `json:"sslmode"`
}

type Server struct {
	config     *Config
	logger     *slog.Logger
	httpServer *http.Server
	sqlClient  *sqlserver.Client
	masterDB   *sql.DB
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

	if srv.masterDB != nil {
		if err := srv.masterDB.Close(); err != nil {
			srv.logger.Error("Error closing master DB", "error", err)
		}
	}
}

func (srv *Server) initialize() error {
	if srv.config.DB.Host == "" || srv.config.DB.Port == "" || srv.config.DB.MasterName == "" || srv.config.DB.Username == "" || srv.config.DB.Password == "" {
		return errors.New("database connection vars must be specified")
	}

	conn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", srv.config.DB.Host, srv.config.DB.Port, srv.config.DB.MasterName, srv.config.DB.Username, srv.config.DB.Password)

	// Master DB for tenant metadata and migrations
	masterDB, err := sql.Open("postgres", conn)
	if err != nil {
		return fmt.Errorf("failed to open master db: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := masterDB.PingContext(ctx); err != nil {
		_ = masterDB.Close()
		return fmt.Errorf("failed to ping master db: %v", err)
	}
	srv.masterDB = masterDB

	client, err := sqlserver.NewClient(conn, srv.logger)
	if err != nil {
		return fmt.Errorf("failed to create a SQL Server client: %v", err)
	}

	srv.sqlClient = client

	// Run migrations for all tenant databases (best-effort, non-fatal on error).
	migCfg := migrate.DBConfig{
		Host:     srv.config.DB.Host,
		Port:     srv.config.DB.Port,
		Username: srv.config.DB.Username,
		Password: srv.config.DB.Password,
		SSLMode:  srv.config.DB.SSLMode,
	}
	migCtx, migCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer migCancel()
	if err := migrate.ApplyAllWithAutoDiscovery(migCtx, srv.masterDB, migCfg, srv.logger); err != nil {
		srv.logger.Error("failed to apply migrations", "error", err)
	}

	return nil
}
