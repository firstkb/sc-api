package sqlserver

import (
	"database/sql"
	"log/slog"
)

func NewClientMock(db *sql.DB, logger *slog.Logger, dbnames map[string]string) *Client {
	return &Client{
		db:      db,
		logger:  logger,
		dbnames: dbnames,
	}
}
