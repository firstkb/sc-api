package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	dbname = "{db}"
)

var (
	longRunningQueryThreshold time.Duration = 2 * time.Second
)

type Database struct {
	client *Client
	ctx    context.Context
	db     *sql.DB
	Name   string
}

func (d *Database) Exec(query string, args ...any) (sql.Result, error) {
	query = d.setDbName(query)

	var executionTime time.Duration

	result, err := runWithRetry(func() (sql.Result, error) {
		start := time.Now()
		result, err := d.db.ExecContext(d.ctx, query, args...)
		executionTime = time.Since(start)
		return result, err
	})

	if d.shouldTrace(err, executionTime) {
		d.traceQuery(err, executionTime, query, args...)
	}

	return result, err
}

func (d *Database) Query(query string, args ...any) (*sql.Rows, error) {
	query = d.setDbName(query)

	var executionTime time.Duration

	rows, err := runWithRetry(func() (*sql.Rows, error) {
		start := time.Now()
		rows, err := d.db.QueryContext(d.ctx, query, args...)
		executionTime = time.Since(start)
		return rows, err
	})

	if d.shouldTrace(err, executionTime) {
		d.traceQuery(err, executionTime, query, args...)
	}

	return rows, err
}

func (d *Database) QueryRow(query string, args ...any) *sql.Row {
	query = d.setDbName(query)

	var executionTime time.Duration

	row, _ := runWithRetry(func() (*sql.Row, error) {
		start := time.Now()
		row := d.db.QueryRowContext(d.ctx, query, args...)
		executionTime = time.Since(start)
		return row, nil
	})

	var rowErr error
	if row != nil {
		rowErr = row.Err()
	}
	if d.shouldTrace(rowErr, executionTime) {
		d.traceQuery(rowErr, executionTime, query, args...)
	}

	return row
}

func (d *Database) Select(query string, params ...any) ([]map[string]any, error) {
	rows, err := d.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %v", err)
	}

	results := make([]map[string]any, 0)

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		rowMap := make(map[string]any)
		for i, col := range columns {
			rowMap[col] = d.normalizeDBValue(values[i])
		}

		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error encountered during rows iteration: %v", err)
	}

	return results, nil
}

func (d *Database) setDbName(query string) string {
	if d.Name == "" || !strings.Contains(query, dbname) {
		return query
	}

	return strings.ReplaceAll(query, dbname, d.Name)
}

func (d *Database) shouldTrace(err error, executionTime time.Duration) bool {
	if err != nil || executionTime > longRunningQueryThreshold {
		return true
	}
	if d.client != nil && d.client.debug {
		return true
	}
	return false
}

func (d *Database) traceQuery(err error, executionTime time.Duration, query string, args ...any) {
	executionMs := executionTime.Milliseconds()
	dbName := d.Name
	switch {
	case err != nil:
		d.client.logger.Debug("sql error", "db", dbName, "execution_ms", executionMs, "query", query, "params", args, "error", err)
	case executionTime > longRunningQueryThreshold:
		d.client.logger.Warn(
			"sql slow query",
			"db", dbName,
			"threshold_ms", longRunningQueryThreshold.Milliseconds(),
			"execution_ms", executionMs,
			"query", query,
			"params", args,
		)
	default:
		d.client.logger.Debug("sql query", "db", dbName, "execution_ms", executionMs, "query", query, "params", args)
	}
}

func (d *Database) normalizeDBValue(val any) any {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case []byte:
		// text/uuid and similar types are often returned as []byte by pq;
		// convert to string so JSON-логирование и API-ответы были человекочитаемыми.
		return string(v)
	default:
		return val
	}
}
