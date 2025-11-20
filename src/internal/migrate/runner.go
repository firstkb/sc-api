package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Runner applies SQL migrations across all app databases listed in tenant(db_name).
// It is intentionally decoupled from CLI and can be invoked from any Go entrypoint.
type Runner struct {
	MasterDB      *sql.DB // connection to master meta database
	MigrationsDir string  // directory with app migrations (*.sql)
	SandboxDBName string  // optional sandbox database name
	Logger        *slog.Logger

	dbFactory DBFactory
}

// DBFactory opens a connection to a specific application database by its name.
// The caller is responsible for configuring DSN, pooling, etc.
type DBFactory func(ctx context.Context, dbName string) (*sql.DB, error)

// NewRunner creates a new migration runner.
// masterDB should point to meta (master) DB; dbFactory is used to open connections to app databases.
func NewRunner(masterDB *sql.DB, migrationsDir string, sandboxDBName string, dbFactory DBFactory, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		MasterDB:      masterDB,
		MigrationsDir: migrationsDir,
		SandboxDBName: sandboxDBName,
		Logger:        logger,
		dbFactory:     dbFactory,
	}
}

// Migration represents a single SQL migration file.
type Migration struct {
	Version string // usually prefix in filename, e.g. "010_app_template"
	Name    string // full filename
	Path    string // absolute or relative path to .sql file
}

// DBConfig holds minimal DB settings required to open tenant connections.
type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	SSLMode  string
}

// ApplyAll discovers all app databases and applies pending migrations to each of them.
// Databases list = [sandboxDBName, if set] + DISTINCT tenant.db_name (WHERE db_name IS NOT NULL).
// It continues on errors, logging them, and returns the first error (if any).
func (r *Runner) ApplyAll(ctx context.Context) error {
	if r.MasterDB == nil {
		return fmt.Errorf("migrate: master DB is required")
	}
	if r.dbFactory == nil {
		return fmt.Errorf("migrate: dbFactory is required")
	}

	migrations, err := r.loadMigrations()
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		r.Logger.Info("migrate: no migrations found, nothing to apply", "dir", r.MigrationsDir)
		return nil
	}

	dbNames, err := r.listAppDatabases(ctx)
	if err != nil {
		return err
	}
	if len(dbNames) == 0 {
		r.Logger.Warn("migrate: no app databases found")
		return nil
	}

	var firstErr error

	for _, dbName := range dbNames {
		if err := r.applyToDatabase(ctx, dbName, migrations); err != nil {
			r.Logger.Error("migrate: failed to apply migrations to database", "db", dbName, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// listAppDatabases returns list of app database names (including sandbox DB if configured).
func (r *Runner) listAppDatabases(ctx context.Context) ([]string, error) {
	const q = `
SELECT DISTINCT db_name
  FROM tenant
 WHERE db_name IS NOT NULL
 ORDER BY db_name`

	rows, err := r.MasterDB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("migrate: list app databases: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var dbs []string

	// Always include sandbox DB as app DB if configured
	if r.SandboxDBName != "" {
		seen[r.SandboxDBName] = true
		dbs = append(dbs, r.SandboxDBName)
	}

	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, fmt.Errorf("migrate: scan db_name: %w", scanErr)
		}
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		dbs = append(dbs, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: iterate app databases: %w", err)
	}

	return dbs, nil
}

// loadMigrations scans MigrationsDir and returns sorted list of *.sql migrations.
func (r *Runner) loadMigrations() ([]Migration, error) {
	if strings.TrimSpace(r.MigrationsDir) == "" {
		return nil, fmt.Errorf("migrate: migrations directory is required")
	}
	entries, err := os.ReadDir(r.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("migrate: read migrations dir: %w", err)
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
		path := filepath.Join(r.MigrationsDir, name)
		version := strings.TrimSuffix(name, ".sql")
		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			Path:    path,
		})
	}

	// Sort by filename (version prefix)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

// applyToDatabase applies pending migrations to a single database.
func (r *Runner) applyToDatabase(ctx context.Context, dbName string, migrations []Migration) error {
	db, err := r.dbFactory(ctx, dbName)
	if err != nil {
		return fmt.Errorf("migrate: open db %s: %w", dbName, err)
	}
	defer db.Close()

	if err := ensureSchemaMigrations(ctx, db); err != nil {
		return fmt.Errorf("migrate: ensure schema_migrations in %s: %w", dbName, err)
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("migrate: load applied migrations in %s: %w", dbName, err)
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("migrate: apply %s to %s: %w", m.Name, dbName, err)
		}
		r.Logger.Info("migrate: applied migration", "db", dbName, "migration", m.Name)
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
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[strings.TrimSpace(v)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m Migration) error {
	content, err := os.ReadFile(m.Path)
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
	if _, err := tx.ExecContext(ctx, insert, m.Version, time.Now()); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ApplyAllWithAutoDiscovery builds a Runner with sensible defaults and applies all migrations.
// It:
//   - discovers migrations dir (MIGRATIONS_DIR or src/migrations/postgres/app relative to cwd),
//   - opens tenant databases using provided DBConfig,
//   - applies pending migrations to each database.
func ApplyAllWithAutoDiscovery(ctx context.Context, masterDB *sql.DB, dbCfg DBConfig, logger *slog.Logger) error {
	if masterDB == nil {
		return fmt.Errorf("migrate: master DB is required")
	}

	migrationsDir := discoverMigrationsDir()

	dbFactory := func(ctx context.Context, dbName string) (*sql.DB, error) {
		name := strings.TrimSpace(dbName)
		if name == "" {
			return nil, fmt.Errorf("tenant db name is required")
		}

		dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
			dbCfg.Host, dbCfg.Port, name, dbCfg.Username, dbCfg.Password, valueOrDefault(dbCfg.SSLMode, "disable"))

		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("open migration db '%s': %w", name, err)
		}

		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping migration db '%s': %w", name, err)
		}

		return db, nil
	}

	runner := NewRunner(masterDB, migrationsDir, "", dbFactory, logger)
	return runner.ApplyAll(ctx)
}

// discoverMigrationsDir determines migrations directory using MIGRATIONS_DIR or common defaults.
func discoverMigrationsDir() string {
	// Default relative path (inside repo)
	dir := "src/migrations/postgres/app"

	if envDir := os.Getenv("MIGRATIONS_DIR"); envDir != "" {
		return envDir
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates := []string{
			filepath.Join(cwd, "src", "migrations", "postgres", "app"),
			filepath.Join(cwd, "migrations", "postgres", "app"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				if abs, err := filepath.Abs(candidate); err == nil {
					return abs
				}
			}
		}
	}

	return dir
}

func valueOrDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
