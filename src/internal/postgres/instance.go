package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (c *Client) LoadInstancesFromMaster(ctx context.Context) error {
	rows, err := c.masterDB.QueryContext(ctx, `
		SELECT code, dns, secret_name, updated_at
		FROM db_instance`)
	if err != nil {
		return fmt.Errorf("load instances: %w", err)
	}
	defer rows.Close()

	tmp := make(map[string]instanceCfg)

	for rows.Next() {
		var (
			code, dns, secretName sql.NullString
			updatedAt             time.Time
		)
		if err := rows.Scan(&code, &dns, &secretName, &updatedAt); err != nil {
			return fmt.Errorf("scan instance: %w", err)
		}
		if !code.Valid || code.String == "" {
			continue
		}

		dsn := strings.TrimSpace(dns.String)
		if dsn == "" && secretName.Valid && secretName.String != "" && c.instanceResolv != nil {
			s, err := c.instanceResolv(code.String, secretName.String)
			if err != nil {
				c.logger.Error("instance secret resolve failed", "code", code.String, "error", err)
			} else {
				dsn = s
			}
		}
		if dsn == "" {
			// if dns and secret are empty, skip, will fallback to baseParms
			continue
		}

		params, err := ParseDSN(c.logger, dsn)
		if err != nil {
			c.logger.Error("instance dsn parse failed", "code", code.String, "dsn", dsn, "error", err)
			continue
		}
		// clear dbname — here it will be set by tenant_db
		params.DBName = ""
		tmp[code.String] = instanceCfg{params: params, updatedAt: updatedAt}
		c.logger.Info(fmt.Sprintf("instance loaded %s - %s", code.String, updatedAt.Format(time.DateTime)))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", err)
	}

	c.instancesMu.Lock()
	c.instances = tmp
	c.instancesMu.Unlock()

	//c.logger.Info("instances loaded", "count", len(tmp), "instances", tmp)
	return nil
}

func (c *Client) SeedBaseInstance(code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()
	if _, ok := c.instances[code]; ok {
		return
	}
	p := c.baseParms
	p.DBName = "" // db name is selected by tenant_db
	c.instances[code] = instanceCfg{params: p, updatedAt: time.Now()}
}
