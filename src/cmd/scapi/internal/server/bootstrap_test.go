package server

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/options"
)

func TestBootstrapRequiresDatabaseConfig(t *testing.T) {
	opts, err := options.NewOptions("SCAPI")
	if err != nil {
		t.Fatalf("failed to prepare options: %v", err)
	}

	cfgOpts := config.AddConfigOptions(opts)
	cfg, err := config.GetConfig(cfgOpts, nil)
	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if _, err := Bootstrap(cfg, logger); err == nil {
		t.Fatalf("expected configuration error, got nil")
	} else if !strings.Contains(err.Error(), "database") {
		t.Fatalf("unexpected error: %v", err)
	}
}
