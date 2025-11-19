package sqlserver

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
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
	version []byte // rowversion for dbnames
	updated time.Time
}

var dbnamesUpdateInterval time.Duration = 5 * time.Minute

func NewClient(connStr string, logger *slog.Logger) (*Client, error) {
	connector, err := mssql.NewConnector(connStr)
	if err != nil {
		return nil, fmt.Errorf("error creating connector: %v", err)
	}

	connector.SessionInitSQL = "SET ANSI_NULLS, ANSI_PADDING, ANSI_WARNINGS, QUOTED_IDENTIFIER, CONCAT_NULL_YIELDS_NULL, ARITHABORT ON"

	client := &Client{
		db:      sql.OpenDB(connector),
		logger:  logger,
		version: make([]byte, 8),
	}

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
	if c != nil || c.db != nil {
		return c.db.Close()
	}

	return nil
}

func (c *Client) OpenDB(ctx context.Context, clientId string) (*Database, error) {
	name := c.getDbName(clientId)
	if name == "" {
		return nil, fmt.Errorf("invalid client id: '%s'", clientId)
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

func (c *Client) getDbName(clientId string) string {
	var name string

	if c.dbnames != nil {
		name = c.dbnames[clientId]
	}

	if name == "" {
		if c.loadDbNames() {
			name = c.dbnames[clientId]
		}
	}

	return name
}

func (c *Client) GetDbNames() map[string]string {
	return c.dbnames
}

func (c *Client) loadDbNames() bool {
	if time.Since(c.updated) < dbnamesUpdateInterval {
		return false
	}

	version, err := c.getMaxVersion()
	if err != nil {
		c.logger.Error(fmt.Sprintf("sqlclient get dbnames version: %v", err))
		return false
	}

	if bytes.Equal(c.version, version) {
		c.updated = time.Now()
		return false
	}

	query := `
	SELECT client_id, database_name
	FROM client_config
	`

	rows, err := c.db.Query(query)
	if err != nil {
		c.logger.Error(fmt.Sprintf("sqlclient get dbnames: %v", err))
		return false
	}
	defer rows.Close()

	dbnames := make(map[string]string)
	for rows.Next() {
		var clientID, dbName string
		if err := rows.Scan(&clientID, &dbName); err != nil {
			c.logger.Error(fmt.Sprintf("sqlclient scan dbnames: %v", err))
			return false
		}
		dbnames[clientID] = dbName
	}

	if err = rows.Err(); err != nil {
		c.logger.Error(fmt.Sprintf("sqlclient rows dbnames: %v", err))
		return false
	}
	c.logger.Debug("client map update")
	if len(dbnames) == 0 {
		return false
	}

	c.dbnames = dbnames
	c.version = version
	c.updated = time.Now()

	return true
}

func (c *Client) getMaxVersion() ([]byte, error) {
	query := `
	SELECT MAX(rowversion) 
	FROM client_config
	`

	var version []byte
	err := c.db.QueryRow(query).Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get max version: %v", err)
	}

	return version, nil
}
