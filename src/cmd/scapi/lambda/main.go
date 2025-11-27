package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/firstkb/sc-api/cmd/scapi/internal/server"
	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/hosting"
	"github.com/firstkb/sc-api/internal/options"
)

var (
	Version = "0.0.0"
	Build   = "lambda"
)

var defaultConfiguration []byte

var (
	handlerMu      sync.Mutex
	handlerAdapter *httpadapter.HandlerAdapterV2

	loadRuntimeConfig = defaultRuntimeLoader
	bootstrapHTTP     = defaultBootstrap
)

type handlerProvider interface {
	Handler() http.Handler
}

func init() {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err == nil {
		time.Local = loc
	}
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	adapter, err := ensureHandler(ctx)
	if err != nil {
		slog.Error("lambda handler bootstrap failed", "error", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       http.StatusText(http.StatusInternalServerError),
		}, err
	}

	return adapter.ProxyWithContext(ctx, req)
}

func ensureHandler(ctx context.Context) (*httpadapter.HandlerAdapterV2, error) {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	if handlerAdapter != nil {
		return handlerAdapter, nil
	}

	cfg, logger, err := loadRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := bootstrapHTTP(cfg, logger)
	if err != nil {
		return nil, err
	}

	handlerAdapter = httpadapter.NewV2(srv.Handler())
	return handlerAdapter, nil
}

func defaultRuntimeLoader(_ context.Context) (*config.Config, *slog.Logger, error) {
	host := newServiceHost()

	opts, err := options.NewOptions(host.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("options: %w", err)
	}

	host.Initialize(opts, defaultConfiguration, envPrefixes(host.Name))

	if host.Config == nil {
		return nil, nil, errors.New("configuration is not initialized")
	}

	return host.Config, host.Logger, nil
}

func defaultBootstrap(cfg *config.Config, logger *slog.Logger) (handlerProvider, error) {
	srv, err := server.Bootstrap(cfg, logger)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func newServiceHost() *hosting.ServiceHost {
	host := &hosting.ServiceHost{
		Name:        "SCAPI",
		DisplayName: "SafeConstructors API",
		Description: "SafeConstructors API Server",
		Version:     Version,
		Logger:      slog.Default(),
	}

	if Build != "" {
		host.Version = Version + "." + Build
		os.Setenv("MIGRATION_VERSION", host.Version)
	}

	return host
}

func envPrefixes(serviceName string) []string {
	return []string{serviceName + "_", "AWS_"}
}
