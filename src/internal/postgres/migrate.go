package postgres

import (
	"context"
	"fmt"
	"log/slog"
)

// ApplyMigrations runs SQL migrations for all known application databases.
// It leverages postgres.Migrator that reuses Client internals (instances, pool settings).
func (c *Client) ApplyMigrations(ctx context.Context, logger *slog.Logger, opts ...MigratorOption) error {
	if c == nil || c.masterDB == nil {
		return fmt.Errorf("postgres: master DB is not initialized")
	}

	options := append([]MigratorOption{WithLogger(logger)}, opts...)
	migrator, err := NewMigrator(c, options...)
	if err != nil {
		return err
	}

	return migrator.ApplyAll(ctx)
}
