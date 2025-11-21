package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/firstkb/sc-api/internal/config"
)

// TestMiddlewareChain_Mock проверяет, что базовая цепочка middleware не ломает health-route.
func TestMiddlewareChain_Mock(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	srv, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.httpServer.Handler.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not respond in time")
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rr.Code, rr.Body.String())
	}
}
