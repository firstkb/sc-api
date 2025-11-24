package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/firstkb/sc-api/internal/migrate"
)

// ApplyMigrations runs SQL migrations for all known application databases.
// It reuses the master connection and base connection settings from the client,
// while delegating migration logic to the shared migrate.Runner.
func (c *Client) ApplyMigrations(ctx context.Context, logger *slog.Logger) error {
	if c.masterDB == nil {
		return fmt.Errorf("sqlserver: master DB is not initialized")
	}
	if logger == nil {
		logger = c.logger
	}

	dbCfg := migrate.DBConfig{
		Host:     c.baseParms.Host,
		Port:     c.baseParms.Port,
		Username: c.baseParms.User,
		Password: c.baseParms.Password,
		SSLMode: func() string {
			if strings.TrimSpace(c.baseParms.SSLMode) == "" {
				return "disable"
			}
			return c.baseParms.SSLMode
		}(),
	}

	return migrate.ApplyAllWithAutoDiscovery(ctx, c.masterDB, dbCfg, logger)
}
