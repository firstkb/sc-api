package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"log/slog"
)

// getTestConnString returns a connection string for tests or empty string if not configured.
// If SCAPI_TEST_PG_CONN is not set, tests depending on a real database are skipped.
func getTestConnString(tb testing.TB) string {
	tb.Helper()
	conn := os.Getenv("SCAPI_TEST_PG_CONN")
	if conn == "" {
		tb.Skip("SCAPI_TEST_PG_CONN is not set, skipping PostgreSQL integration tests")
	}
	return conn
}

func TestNewClientAndLoadDbNames(t *testing.T) {
	connStr := getTestConnString(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	client, err := NewClient(connStr, logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// First call should populate cache (full load).
	if ok := client.loadDbNames(); !ok {
		t.Log("loadDbNames returned false on initial load (no tenants yet or query error)")
	}

	initialUpdated := client.updated

	// Second call within interval should be throttled and return false without error.
	if ok := client.loadDbNames(); ok {
		t.Errorf("expected throttled loadDbNames to return false, got true")
	}

	// Sanity: GetPhysicalDbNames should not panic and should be consistent with internal map.
	names := client.GetPhysicalDbNames()
	if client.dbnames == nil && len(names) != 0 {
		t.Errorf("expected empty physical db names when internal map is nil, got %d", len(names))
	}

	// updated timestamp should be set after first successful call.
	if !initialUpdated.IsZero() && client.updated.Before(initialUpdated) {
		t.Errorf("expected updated timestamp to move forward or stay equal, got %v -> %v", initialUpdated, client.updated)
	}
}

func TestDatabaseExecQueryLifecycle(t *testing.T) {
	connStr := getTestConnString(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	client, err := NewClient(connStr, logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Use OpenDBMaster to avoid dependency on tenant mapping.
	db, err := client.OpenDBMaster(context.Background())
	if err != nil {
		t.Fatalf("failed to open master db: %v", err)
	}

	// Use a simple sanity check query that should work on any PostgreSQL database.
	const q = "SELECT 1"

	row := db.QueryRow(q)
	var v int
	if err := row.Scan(&v); err != nil {
		// If the error is "relation does not exist" or similar, it is still a valid round-trip.
		if err == sql.ErrNoRows {
			return
		}
		t.Fatalf("failed to execute test query: %v", err)
	}
}
