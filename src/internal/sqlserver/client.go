package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

// TODO: config
var (
	MaxIdleConns    = 100
	MaxOpenConns    = 100
	ConnMaxLifetime = 60 * time.Minute
)

type Client struct {
	db      *sql.DB
	logger  *slog.Logger
	dbnames map[string]string
	updated time.Time
}

var dbnamesUpdateInterval time.Duration = 5 * time.Minute

func NewClient(connStr string, logger *slog.Logger) (*Client, error) {
	client := &Client{
		logger: logger,
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening postgres connection: %v", err)
	}

	client.db = db

	client.db.SetMaxIdleConns(MaxIdleConns)
	client.db.SetMaxOpenConns(MaxOpenConns)
	client.db.SetConnMaxLifetime(ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.db.PingContext(ctx); err != nil {
		client.logger.Error(fmt.Sprintf("database connection failed: %v", err))
	} else {
		client.loadDbNames()
	}

	return client, nil
}

func (c *Client) Close() error {
	if c != nil && c.db != nil {
		return c.db.Close()
	}

	return nil
}

func (c *Client) OpenDB(ctx context.Context, tenantId string) (*Database, error) {
	name := c.getDbName(tenantId)
	if name == "" {
		return nil, fmt.Errorf("invalid tenant id: '%s'", tenantId)
	}

	return &Database{
		client: c,
		ctx:    ctx,
		Name:   name,
	}, nil
}

func (c *Client) OpenDBMaster(ctx context.Context) (*Database, error) {

	return &Database{
		client: c,
		ctx:    ctx,
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
		rows, err = c.db.Query(query)
	} else {
		query := `
	SELECT CAST(id AS text) AS tenant_id, db_name, updated_at
	FROM tenant
	WHERE updated_at > $1
	`
		rows, err = c.db.Query(query, c.updated)
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
