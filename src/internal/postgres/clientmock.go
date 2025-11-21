package postgres

import (
	"database/sql"
	"log/slog"
)

func NewClientMock(db *sql.DB, logger *slog.Logger, dbnames map[string]string) *Client {
	return &Client{
		masterDB: db,
		logger:   logger,
		dbnames:  dbnames,
	}
}
