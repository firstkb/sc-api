package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type poolKey struct {
	instance string // instance code, e.g. "S1"
	name     string // db_name
}

type instanceCfg struct {
	params    dsnParams // host, port, user, password, sslmode (without dbname)
	updatedAt time.Time
}

type InstanceResolver func(code, secretName string) (string, error) // returns full DSN

type poolConfig struct {
	MaxIdle     int
	MaxOpen     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

type Client struct {
	masterDB *sql.DB
	logger   *slog.Logger

	poolsMu   sync.RWMutex
	pools     map[poolKey]*sql.DB
	poolCfg   poolConfig
	baseParms dsnParams

	instancesMu    sync.RWMutex
	instances      map[string]instanceCfg
	instanceResolv InstanceResolver

	debug bool
}

var dbnamesUpdateInterval time.Duration = 5 * time.Minute

// Option configures the Client.
type Option func(*Client)

// WithPoolConfig overrides default pool settings for master and tenant DBs.
func WithPoolConfig(maxIdle, maxOpen int, maxLifetime time.Duration, maxIdleTime time.Duration) Option {
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
		if maxIdleTime > 0 {
			c.poolCfg.MaxIdleTime = maxIdleTime
		}
	}
}

func WithDebug(debug bool) Option {
	return func(c *Client) {
		c.debug = debug
	}
}

func NewClient(connStr string, logger *slog.Logger, opts ...Option) (*Client, error) {
	client := &Client{
		logger: logger,
		poolCfg: poolConfig{
			MaxIdle:     10,
			MaxOpen:     50,
			MaxLifetime: 30 * time.Minute,
			MaxIdleTime: 0,
		},
		pools:     make(map[poolKey]*sql.DB),
		instances: make(map[string]instanceCfg),
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
	if client.poolCfg.MaxIdleTime > 0 {
		client.masterDB.SetConnMaxIdleTime(client.poolCfg.MaxIdleTime)
	}

	// Cache base connection parameters to build DSN for tenant databases.
	bp, err := ParseDSN(client.logger, connStr)
	if err != nil {
		return nil, fmt.Errorf("parse base DSN: %w", err)
	}
	client.baseParms = bp

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.masterDB.PingContext(ctx); err != nil {
		client.logger.Error(fmt.Sprintf("database connection failed: %v", err))
	} else {
		//client.loadDbNames()
		if err := client.LoadInstancesFromMaster(ctx); err != nil {
			client.logger.Warn("LoadInstancesFromMaster failed", "error", err)
		}
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
	for k, db := range c.pools {
		if db != nil && db.Close() != nil {
			c.logger.Warn("failed to close tenant DB", "instance", k.instance, "db", k.name)
		}
	}
	c.pools = make(map[poolKey]*sql.DB)
	c.poolsMu.Unlock()

	return nil
}

func WithInstanceResolver(r InstanceResolver) Option {
	return func(c *Client) { c.instanceResolv = r }
}

func (c *Client) OpenDBTenant(ctx context.Context, dbName string, dbInstanceCode string) (*Database, error) {
	db, err := c.getOrOpenDB(dbName, dbInstanceCode)
	if err != nil {
		return nil, fmt.Errorf("open tenant db: %w", err)
	}
	return &Database{
		client: c,
		ctx:    ctx,
		db:     db,
		Name:   dbName,
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

func (c *Client) instanceParams(code string) (dsnParams, bool) {
	if strings.TrimSpace(code) == "" {
		return c.baseParms, true
	}
	c.instancesMu.RLock()
	inst, ok := c.instances[code]
	c.instancesMu.RUnlock()
	if !ok {
		// collect known instances for logging
		c.instancesMu.RLock()
		known := make([]string, 0, len(c.instances))
		for k := range c.instances {
			known = append(known, k)
		}
		c.instancesMu.RUnlock()
		c.logger.Warn("unknown db instance code, fallback to base env",
			"code", code, "known", known)
		return c.baseParms, false
	}
	return inst.params, true
}

// getOrOpenDB returns an existing *sql.DB for the given database name or
// lazily creates a new one using the same connection parameters as master DB
// but with overridden dbname.
func (c *Client) getOrOpenDB(dbName string, instanceCode string) (*sql.DB, error) {
	name := strings.TrimSpace(dbName)
	inst := strings.TrimSpace(instanceCode)
	if name == "" {
		return nil, fmt.Errorf("tenant db name is required")
	}

	key := poolKey{instance: inst, name: name}

	// fast path
	c.poolsMu.RLock()
	if db, ok := c.pools[key]; ok && db != nil {
		c.poolsMu.RUnlock()
		return db, nil
	}
	c.poolsMu.RUnlock()

	// slow path
	c.poolsMu.Lock()
	defer c.poolsMu.Unlock()
	if db, ok := c.pools[key]; ok && db != nil {
		return db, nil
	}

	params, ok := c.instanceParams(inst)
	if !ok {
		c.logger.Warn("unknown db instance code, fallback to base env", "code", inst)
	}
	if params.Host == "" {
		return nil, fmt.Errorf("cannot open db %q: base/instance connection params are empty", name)
	}

	dsn := params.WithDBName(name).String()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s/%s: %w", inst, name, err)
	}

	db.SetMaxIdleConns(c.poolCfg.MaxIdle)
	db.SetMaxOpenConns(c.poolCfg.MaxOpen)
	db.SetConnMaxLifetime(c.poolCfg.MaxLifetime)
	if c.poolCfg.MaxIdleTime > 0 {
		db.SetConnMaxIdleTime(c.poolCfg.MaxIdleTime)
	}

	// ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s/%s: %w", inst, name, err)
	}

	c.pools[key] = db
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
// replace parseDSN with parseKVDSN, add ParseDSN, and reuse parseURIDSN

// ParseDSN detects URI vs key=value and returns dsnParams
func ParseDSN(logger *slog.Logger, s string) (dsnParams, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return dsnParams{}, nil
	}
	if strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://") {
		result, err := parseURIDSN(s)
		return result, err
	}
	result := parseKVDSN(s)
	return result, nil
}

// key=value
func parseKVDSN(s string) dsnParams {
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

func parseURIDSN(s string) (dsnParams, error) {
	u, err := url.Parse(s)
	if err != nil {
		return dsnParams{}, err
	}

	var p dsnParams
	if u.User != nil {
		p.User = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			p.Password = pw
		}
	}
	// host:port
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// может быть без порта
		host = u.Host
	}
	p.Host = host
	p.Port = port

	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		p.DBName = db
	}
	q := u.Query()
	if sm := q.Get("sslmode"); sm != "" {
		p.SSLMode = sm
	}

	return p, nil
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
