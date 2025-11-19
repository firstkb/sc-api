package internal

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/firstkb/sc-api/cmd/scapi/internal/server"
	"github.com/firstkb/sc-api/internal/config"
)

const (
	FlagHost = "host"
)

type HostedService struct { // implements hosting.HostedService
	logger *slog.Logger
	server server.Server
}

func (svc *HostedService) Run(ctx context.Context, config *config.Config, logger *slog.Logger) error {
	svc.logger = logger

	svr, err := server.NewServer(config, logger)
	if err != nil {
		return err
	}

	go func() { svr.Run(ctx) }()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		svc.Stop(logger)
	}()
	wg.Wait()

	return nil
}

func (svc *HostedService) Stop(logger *slog.Logger) {
	//TODO: add panic handler

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc.server.Stop(shutdownCtx)
}
