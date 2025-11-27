package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Migration describes a single SQL file that should be applied sequentially.
type Migration struct {
	Version string
	Name    string
	Path    string
}

type databaseRef struct {
	Name         string
	InstanceCode string
}

// Migrator coordinates loading *.sql migrations and applying them to every tenant DB.
type Migrator struct {
	client        *Client
	logger        *slog.Logger
	migrationsDir string
	extraDBs      []databaseRef
	version       string
}

// MigratorOption modifies Migrator behaviour (directory, logger, additional DBs, etc.).
type MigratorOption func(*Migrator)

const (
	migrationStatusRunning   = "running"
	migrationStatusCompleted = "completed"
	migrationStatusFailed    = "failed"
)

// NewMigrator constructs Migrator bound to a postgres Client.
func NewMigrator(client *Client, opts ...MigratorOption) (*Migrator, error) {
	if client == nil {
		return nil, fmt.Errorf("postgres: client is required for migrations")
	}

	m := &Migrator{
		client:        client,
		logger:        client.logger,
		migrationsDir: discoverMigrationsDir(),
		version:       strings.TrimSpace(os.Getenv("MIGRATION_VERSION")),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}

	if m.logger == nil {
		m.logger = slog.Default()
	}
	if strings.TrimSpace(m.migrationsDir) == "" {
		return nil, fmt.Errorf("postgres: migrations directory is required")
	}

	return m, nil
}

// WithLogger overrides logger used by migrator.
func WithLogger(logger *slog.Logger) MigratorOption {
	return func(m *Migrator) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// WithMigrationsDir sets a custom directory with *.sql migrations.
func WithMigrationsDir(dir string) MigratorOption {
	return func(m *Migrator) {
		if strings.TrimSpace(dir) != "" {
			m.migrationsDir = dir
		}
	}
}

// WithExtraDatabase schedules an additional DB (name + optional instance code) for migrations.
func WithExtraDatabase(name, instanceCode string) MigratorOption {
	return func(m *Migrator) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		m.extraDBs = append(m.extraDBs, databaseRef{
			Name:         name,
			InstanceCode: strings.TrimSpace(instanceCode),
		})
	}
}

// WithSandboxDatabase is a helper for the legacy sandbox database (uses base instance).
func WithSandboxDatabase(name string) MigratorOption {
	return WithExtraDatabase(name, "")
}

// WithMigrationVersion enables migration run tracking in master DB to skip duplicate runs.
func WithMigrationVersion(version string) MigratorOption {
	return func(m *Migrator) {
		m.version = strings.TrimSpace(version)
	}
}

// ApplyAll refreshes instances, discovers target DBs and applies pending migrations sequentially.
func (m *Migrator) ApplyAll(ctx context.Context) error {
	if m.client == nil || m.client.masterDB == nil {
		return fmt.Errorf("postgres: master DB is required for migrations")
	}

	if err := m.client.LoadInstancesFromMaster(ctx); err != nil {
		m.logger.Warn("migrator: failed to refresh db instances", "error", err)
	}

	migrations, err := m.loadMigrations()
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		m.logger.Info("migrator: no migrations found", "dir", m.migrationsDir)
		return nil
	}

	dbs, err := m.listDatabases(ctx)
	if err != nil {
		return err
	}
	if len(dbs) == 0 {
		m.logger.Warn("migrator: no application databases detected")
		return nil
	}

	runID, skip, err := m.beginMigrationRun(ctx)
	if err != nil {
		return err
	}
	if skip {
		m.logger.Info("migrator: version already applied, skipping migrations", "version", m.version)
		return nil
	}

	var appliedDBs []string
	var applyErr error
	defer func() {
		if runID == 0 {
			return
		}
		status := migrationStatusCompleted
		errText := ""
		if applyErr != nil {
			status = migrationStatusFailed
			errText = applyErr.Error()
		}
		if err := m.finishMigrationRun(ctx, runID, status, appliedDBs, errText); err != nil {
			m.logger.Warn("migrator: failed to update migration_runs", "error", err)
		}
	}()

	for _, db := range dbs {
		if err := m.applyToDatabase(ctx, db, migrations); err != nil {
			m.logger.Error("migrator: failed to apply migrations",
				"db", db.Name, "instance", db.InstanceCode, "error", err)
			if applyErr == nil {
				applyErr = err
			}
			continue
		}
		appliedDBs = append(appliedDBs, db.Name)
	}

	return applyErr
}

func (m *Migrator) loadMigrations() ([]Migration, error) {
	dir := strings.TrimSpace(m.migrationsDir)
	if dir == "" {
		return nil, fmt.Errorf("migrator: migrations directory is required")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migrator: read dir: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		migrations = append(migrations, Migration{
			Version: strings.TrimSuffix(name, ".sql"),
			Name:    name,
			Path:    filepath.Join(dir, name),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

func (m *Migrator) listDatabases(ctx context.Context) ([]databaseRef, error) {
	const q = `
SELECT DISTINCT tdb.db_name, COALESCE(di.code, '') AS instance_code
  FROM tenant_db tdb
  LEFT JOIN db_instance di ON di.id = tdb.db_instance_id
 WHERE tdb.db_name IS NOT NULL
   AND length(trim(tdb.db_name)) > 0
 ORDER BY 1`

	rows, err := m.client.masterDB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("migrator: list databases: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var dbs []databaseRef

	appendDB := func(ref databaseRef) {
		ref.Name = strings.TrimSpace(ref.Name)
		key := fmt.Sprintf("%s/%s", strings.TrimSpace(ref.InstanceCode), ref.Name)
		if ref.Name == "" || seen[key] {
			return
		}
		seen[key] = true
		dbs = append(dbs, ref)
	}

	for _, extra := range m.extraDBs {
		appendDB(extra)
	}

	for rows.Next() {
		var name, instanceCode string
		if err := rows.Scan(&name, &instanceCode); err != nil {
			return nil, fmt.Errorf("migrator: scan database row: %w", err)
		}
		appendDB(databaseRef{
			Name:         name,
			InstanceCode: instanceCode,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrator: iterate databases: %w", err)
	}

	return dbs, nil
}

func (m *Migrator) applyToDatabase(ctx context.Context, ref databaseRef, migrations []Migration) error {
	db, err := m.client.getOrOpenDB(ref.Name, ref.InstanceCode)
	if err != nil {
		return fmt.Errorf("open db %s/%s: %w", ref.InstanceCode, ref.Name, err)
	}

	if err := ensureSchemaMigrations(ctx, db); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	for _, mig := range migrations {
		if applied[mig.Version] {
			continue
		}
		if err := applyMigration(ctx, db, mig); err != nil {
			return fmt.Errorf("apply %s: %w", mig.Name, err)
		}
		m.logger.Info("migrator: applied migration",
			"db", ref.Name, "instance", ref.InstanceCode, "migration", mig.Name)
	}

	return nil
}

func ensureSchemaMigrations(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     TEXT PRIMARY KEY,
  applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	const q = `SELECT version FROM schema_migrations`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[strings.TrimSpace(version)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, mig Migration) error {
	content, err := os.ReadFile(mig.Path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	const insert = `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`
	if _, err := tx.ExecContext(ctx, insert, mig.Version, time.Now()); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// discoverMigrationsDir determines migrations directory using MIGRATIONS_DIR or common defaults.
func discoverMigrationsDir() string {
	if envDir := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); envDir != "" {
		return envDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "src/migrations/postgres/app"
	}

	candidates := []string{
		filepath.Join(cwd, "src", "migrations", "postgres", "app"),
		filepath.Join(cwd, "migrations", "postgres", "app"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
		}
	}

	return "src/migrations/postgres/app"
}

func (m *Migrator) beginMigrationRun(ctx context.Context) (int64, bool, error) {
	version := strings.TrimSpace(m.version)
	if version == "" {
		return 0, false, nil
	}

	tx, err := m.client.masterDB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()

	var lastStatus string
	err = tx.QueryRowContext(ctx, `
SELECT status
  FROM migration_runs
 WHERE version = $1
 ORDER BY started_at DESC
 LIMIT 1`, version).Scan(&lastStatus)
	switch {
	case err == nil:
		switch lastStatus {
		case migrationStatusCompleted:
			return 0, true, nil
		case migrationStatusRunning:
			return 0, false, fmt.Errorf("migrator: migrations for version %s are already running", version)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, err
	}

	var runID int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO migration_runs (version, status)
VALUES ($1, $2)
RETURNING id`, version, migrationStatusRunning).Scan(&runID); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return runID, false, nil
}

func (m *Migrator) finishMigrationRun(ctx context.Context, runID int64, status string, applied []string, errText string) error {
	if runID == 0 {
		return nil
	}

	var appliedVal interface{}
	cleanApplied := uniqueSortedStrings(applied)
	if len(cleanApplied) > 0 {
		appliedVal = pq.StringArray(cleanApplied)
	}

	var errVal interface{}
	if e := strings.TrimSpace(errText); e != "" {
		errVal = e
	}

	_, execErr := m.client.masterDB.ExecContext(ctx, `
UPDATE migration_runs
   SET status = $1,
       completed_at = now(),
       applied_dbs = $2,
       error = $3
 WHERE id = $4`, status, appliedVal, errVal, runID)
	return execErr
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		seen[v] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
