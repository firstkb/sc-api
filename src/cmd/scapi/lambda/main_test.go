package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/firstkb/sc-api/internal/config"
)

func TestHandleRequestSuccess(t *testing.T) {
	reset := stubGlobals(t)
	defer reset()

	loadRuntimeConfig = func(context.Context) (*config.Config, *slog.Logger, error) {
		return nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil
	}

	bootstrapHTTP = func(_ *config.Config, _ *slog.Logger) (handlerProvider, error) {
		return &stubServer{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("ok"))
		})}, nil
	}

	req := events.APIGatewayV2HTTPRequest{
		RawPath: "/ping",
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: http.MethodGet},
		},
	}

	resp, err := handleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestHandleRequestInitError(t *testing.T) {
	reset := stubGlobals(t)
	defer reset()

	loadRuntimeConfig = func(context.Context) (*config.Config, *slog.Logger, error) {
		return nil, nil, errors.New("boom")
	}

	resp, err := handleRequest(context.Background(), events.APIGatewayV2HTTPRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

type stubServer struct {
	handler http.Handler
}

func (s *stubServer) Handler() http.Handler {
	return s.handler
}

func stubGlobals(t *testing.T) func() {
	t.Helper()

	origLoader := loadRuntimeConfig
	origBootstrap := bootstrapHTTP

	handlerMu.Lock()
	handlerAdapter = nil
	handlerMu.Unlock()

	return func() {
		loadRuntimeConfig = origLoader
		bootstrapHTTP = origBootstrap

		handlerMu.Lock()
		handlerAdapter = nil
		handlerMu.Unlock()
	}
}
