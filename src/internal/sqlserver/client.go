package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/firstkb/sc-api/internal/migrate"

	_ "github.com/lib/pq"
)

type poolConfig struct {
	MaxIdle     int
	MaxOpen     int
	MaxLifetime time.Duration
}

type Client struct {
	masterDB *sql.DB
	logger   *slog.Logger

	dbnames map[string]string
	updated time.Time

	poolsMu   sync.RWMutex
	pools     map[string]*sql.DB
	poolCfg   poolConfig
	baseParms dsnParams
}

var dbnamesUpdateInterval time.Duration = 5 * time.Minute

// Option configures the Client.
type Option func(*Client)

// WithPoolConfig overrides default pool settings for master and tenant DBs.
func WithPoolConfig(maxIdle, maxOpen int, maxLifetime time.Duration) Option {
	return func(c *Client) {
		if maxIdle > 0 {
			c.poolCfg.MaxIdle = maxIdle
		}
		if maxOpen > 0 {
			c.poolCfg.MaxOpen = maxOpen
		}
		if maxLifetime > 0 {
			c.poolCfg.MaxLifetime = maxLifetime
		}
	}
}

func NewClient(connStr string, logger *slog.Logger, opts ...Option) (*Client, error) {
	client := &Client{
		logger: logger,
		poolCfg: poolConfig{
			MaxIdle:     10,
			MaxOpen:     50,
			MaxLifetime: 30 * time.Minute,
		},
		pools: make(map[string]*sql.DB),
	}

	for _, opt := range opts {
		opt(client)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening postgres connection: %v", err)
	}

	client.masterDB = db

	client.masterDB.SetMaxIdleConns(client.poolCfg.MaxIdle)
	client.masterDB.SetMaxOpenConns(client.poolCfg.MaxOpen)
	client.masterDB.SetConnMaxLifetime(client.poolCfg.MaxLifetime)

	// Cache base connection parameters to build DSN for tenant databases.
	client.baseParms = parseDSN(connStr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.masterDB.PingContext(ctx); err != nil {
		client.logger.Error(fmt.Sprintf("database connection failed: %v", err))
	} else {
		client.loadDbNames()
	}

	return client, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	// Close master DB.
	if c.masterDB != nil {
		_ = c.masterDB.Close()
	}

	// Close all tenant pools.
	c.poolsMu.Lock()
	for name, db := range c.pools {
		if db != nil {
			if err := db.Close(); err != nil {
				c.logger.Warn("failed to close tenant DB", "db", name, "error", err)
			}
		}
	}
	c.pools = make(map[string]*sql.DB)
	c.poolsMu.Unlock()

	return nil
}

func (c *Client) OpenDB(ctx context.Context, tenantId string) (*Database, error) {
	name := c.getDbName(tenantId)
	if name == "" {
		return nil, fmt.Errorf("invalid tenant id: '%s'", tenantId)
	}

	db, err := c.getOrOpenDB(name)
	if err != nil {
		return nil, err
	}

	return &Database{
		client: c,
		ctx:    ctx,
		db:     db,
		Name:   name,
	}, nil
}

func (c *Client) OpenDBMaster(ctx context.Context) (*Database, error) {
	return &Database{
		client: c,
		ctx:    ctx,
		db:     c.masterDB,
		Name:   "",
	}, nil
}

func (c *Client) getDbName(tenantId string) string {
	var name string

	if c.dbnames != nil {
		name = c.dbnames[tenantId]
	}

	if name == "" {
		if c.loadDbNames() {
			name = c.dbnames[tenantId]
		}
	}

	return name
}

func (c *Client) GetDbNames() map[string]string {
	return c.dbnames
}

func (c *Client) loadDbNames() bool {
	// Throttle refreshes to avoid excessive load on master DB.
	if !c.updated.IsZero() && time.Since(c.updated) < dbnamesUpdateInterval {
		return false
	}

	var (
		rows *sql.Rows
		err  error
	)

	// First load: full snapshot of all tenants.
	// Subsequent loads: incremental by updated_at to avoid re-reading entire table.
	if c.updated.IsZero() {
		query := `
	SELECT CAST(id AS text) AS tenant_id, db_name, updated_at
	FROM tenant
	`
		rows, err = c.masterDB.Query(query)
	} else {
		query := `
	SELECT CAST(id AS text) AS tenant_id, db_name, updated_at
	FROM tenant
	WHERE updated_at > $1
	`
		rows, err = c.masterDB.Query(query, c.updated)
	}

	if err != nil {
		c.logger.Error(fmt.Sprintf("sqlclient get dbnames: %v", err))
		return false
	}
	defer rows.Close()

	// Initialize map on first use; subsequent loads merge updates.
	if c.dbnames == nil {
		c.dbnames = make(map[string]string)
	}

	maxUpdated := c.updated
	updatedAny := false

	for rows.Next() {
		var (
			tenantID  string
			dbName    string
			updatedAt time.Time
		)
		if err := rows.Scan(&tenantID, &dbName, &updatedAt); err != nil {
			c.logger.Error(fmt.Sprintf("sqlclient scan dbnames: %v", err))
			return false
		}
		c.dbnames[tenantID] = dbName
		if updatedAt.After(maxUpdated) {
			maxUpdated = updatedAt
		}
		updatedAny = true
	}

	if err = rows.Err(); err != nil {
		c.logger.Error(fmt.Sprintf("sqlclient rows dbnames: %v", err))
		return false
	}

	// If this is the very first load and we didn't get any tenants,
	// keep behaviour compatible with previous version (no cache, so next call will retry).
	if c.updated.IsZero() && len(c.dbnames) == 0 {
		return false
	}

	// Only advance the cursor if we actually saw newer rows.
	if updatedAny {
		c.updated = maxUpdated
	} else if c.updated.IsZero() {
		// No rows at all, but we attempted initial load — prevent hammering master DB.
		c.updated = time.Now()
	}

	c.logger.Debug("sqlclient dbnames map updated", "size", len(c.dbnames), "updatedAt", c.updated)

	return true
}

// GetPhysicalDbNames returns a de-duplicated list of physical database names
// for all known tenants. It uses the internal cache and may trigger a refresh
// if the cache is stale.
func (c *Client) GetPhysicalDbNames() []string {
	// Best-effort refresh; ignore result, as cache may already be warm.
	_ = c.loadDbNames()

	result := make([]string, 0)
	if c.dbnames == nil {
		return result
	}

	seen := make(map[string]struct{})
	for _, name := range c.dbnames {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}

	return result
}

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

// getOrOpenDB returns an existing *sql.DB for the given database name or
// lazily creates a new one using the same connection parameters as master DB
// but with overridden dbname.
func (c *Client) getOrOpenDB(dbName string) (*sql.DB, error) {
	name := strings.TrimSpace(dbName)
	if name == "" {
		return nil, fmt.Errorf("tenant db name is required")
	}

	// Fast path: try read lock first.
	c.poolsMu.RLock()
	if db, ok := c.pools[name]; ok && db != nil {
		c.poolsMu.RUnlock()
		return db, nil
	}
	c.poolsMu.RUnlock()

	// Slow path: create under write lock.
	c.poolsMu.Lock()
	defer c.poolsMu.Unlock()

	// Re-check after acquiring write lock to avoid duplicate creation.
	if db, ok := c.pools[name]; ok && db != nil {
		return db, nil
	}

	if c.baseParms.Host == "" {
		return nil, fmt.Errorf("cannot open tenant db '%s': base connection parameters are not initialized", name)
	}

	dsn := c.baseParms.WithDBName(name).String()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open tenant db '%s': %w", name, err)
	}

	db.SetMaxIdleConns(c.poolCfg.MaxIdle)
	db.SetMaxOpenConns(c.poolCfg.MaxOpen)
	db.SetConnMaxLifetime(c.poolCfg.MaxLifetime)
	// TODO db.Exec("SET search_path = public, tenant_" + name)

	// Verify connectivity with a short ping.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping tenant db '%s': %w", name, err)
	}

	c.pools[name] = db
	return db, nil
}

// dsnParams represents parsed key=value style PostgreSQL DSN.
type dsnParams struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// parseDSN parses a simple space-separated key=value DSN string into dsnParams.
// It is intentionally conservative and only extracts known keys, ignoring others.
func parseDSN(s string) dsnParams {
	var p dsnParams

	fields := strings.Fields(s)
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "host":
			p.Host = val
		case "port":
			p.Port = val
		case "user", "username":
			p.User = val
		case "password":
			p.Password = val
		case "dbname":
			p.DBName = val
		case "sslmode":
			p.SSLMode = val
		}
	}

	return p
}

// WithDBName returns a copy of dsnParams with DBName overridden.
func (p dsnParams) WithDBName(name string) dsnParams {
	p.DBName = name
	return p
}

// String reconstructs a DSN string from dsnParams.
// Unknown parameters from original DSN are not preserved by design.
func (p dsnParams) String() string {
	parts := make([]string, 0, 6)
	if p.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", p.Host))
	}
	if p.Port != "" {
		parts = append(parts, fmt.Sprintf("port=%s", p.Port))
	}
	if p.DBName != "" {
		parts = append(parts, fmt.Sprintf("dbname=%s", p.DBName))
	}
	if p.User != "" {
		parts = append(parts, fmt.Sprintf("user=%s", p.User))
	}
	if p.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", p.Password))
	}
	if p.SSLMode != "" {
		parts = append(parts, fmt.Sprintf("sslmode=%s", p.SSLMode))
	}
	return strings.Join(parts, " ")
}
