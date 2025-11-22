package server

import (
	"testing"

	"github.com/firstkb/sc-api/internal/config"
	"github.com/firstkb/sc-api/internal/options"
)

const testConfigTOML = `
hostapp = ":0"
origin = "*"
timeout = 1

[token]
provider = "internal"

[db]
host = "127.0.0.1"
port = "1"
username = "test"
password = "test"
mastername = "test"
sslmode = "disable"

[db.pool]
maxidle = 1
maxopen = 1
maxlifetime = "1m"
maxidletime = "1m"
`

func mustTestConfig(t *testing.T) *config.Config {
	t.Helper()

	opts, err := options.NewOptions("SCAPI")
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	cfgOpts := config.AddConfigOptions(opts)
	cfg, err := config.GetConfig(cfgOpts, []byte(testConfigTOML))
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	return cfg
}
