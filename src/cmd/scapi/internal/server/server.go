package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal/pingsvc"
	"github.com/firstkb/sc-api/internal/sqlserver"

	"github.com/firstkb/sc-api/internal/config"
)

type Config struct {
	HostApp string         `json:"hostapp"`
	Origin  string         `json:"origin"`
	DB      DatabaseConfig `json:"db"`
}

type DatabaseConfig struct {
	Host       string     `json:"host"`
	Port       string     `json:"port"`
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	MasterName string     `json:"mastername"`
	SSLMode    string     `json:"sslmode"`
	PoolConfig PoolConfig `json:"pool"`
}

type PoolConfig struct {
	MaxIdle int    `json:"maxidle"`
	MaxOpen int    `json:"maxopen"`
	MaxLife string `json:"maxlifetime"`
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
	if srv.config.DB.Host == "" || srv.config.DB.Port == "" || srv.config.DB.MasterName == "" || srv.config.DB.Username == "" || srv.config.DB.Password == "" {
		return errors.New("database connection vars must be specified")
	}

	conn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", srv.config.DB.Host, srv.config.DB.Port, srv.config.DB.MasterName, srv.config.DB.Username, srv.config.DB.Password)

	// Resolve DB pool settings with sane defaults.
	const (
		defaultPoolMaxIdle = 10
		defaultPoolMaxOpen = 50
		defaultPoolLife    = time.Minute * 30
	)

	poolMaxIdle := srv.config.DB.PoolConfig.MaxIdle
	if poolMaxIdle <= 0 {
		poolMaxIdle = defaultPoolMaxIdle
	}
	poolMaxOpen := srv.config.DB.PoolConfig.MaxOpen
	if poolMaxOpen <= 0 {
		poolMaxOpen = defaultPoolMaxOpen
	}

	poolLife := defaultPoolLife
	if srv.config.DB.PoolConfig.MaxLife != "" {
		if d, err := time.ParseDuration(srv.config.DB.PoolConfig.MaxLife); err == nil {
			poolLife = d
		} else {
			srv.logger.Warn("invalid DB pool max lifetime, using default", "value", srv.config.DB.PoolConfig.MaxLife, "error", err)
		}
	}

	client, err := sqlserver.NewClient(conn, srv.logger,
		sqlserver.WithPoolConfig(poolMaxIdle, poolMaxOpen, poolLife))
	if err != nil {
		return fmt.Errorf("failed to create a SQL Server client: %v", err)
	}

	srv.sqlClient = client

	// Run migrations for all tenant databases (best-effort, non-fatal on error).
	migCtx, migCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer migCancel()
	if err := srv.sqlClient.ApplyMigrations(migCtx, srv.logger); err != nil {
		srv.logger.Error("failed to apply migrations", "error", err)
	}

	return nil
}
