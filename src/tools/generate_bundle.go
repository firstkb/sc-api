package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	archiveDir = "src/migrations/postgres/archive"
	appDir     = "src/migrations/postgres/app"
	bundlePath = "bundle/app_schema_full.sql"
)

// Migration represents a migration file to include in the bundle.
type Migration struct {
	Version string // e.g., "010_app_template"
	Name    string // full filename
	Path    string // full path to file
	Source  string // "archive" or "app"
}

func main() {
	if err := generateBundle(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Generated bundle: %s\n", bundlePath)
}

func generateBundle() error {
	// Load migrations from archive and app directories
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	if len(migrations) == 0 {
		return fmt.Errorf("no migrations found in %s and %s", archiveDir, appDir)
	}

	// Sort by version (filename)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	// Read and concatenate migration files
	var bundleContent strings.Builder
	var migrationVersions []string

	// Write header
	writeHeader(&bundleContent, migrations)

	// Write each migration
	for _, m := range migrations {
		content, err := os.ReadFile(m.Path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.Path, err)
		}

		// Add migration marker
		bundleContent.WriteString("\n-- ============================================\n")
		bundleContent.WriteString(fmt.Sprintf("-- Migration: %s (%s)\n", m.Name, m.Source))
		bundleContent.WriteString("-- ============================================\n\n")
		bundleContent.WriteString(string(content))
		bundleContent.WriteString("\n")

		migrationVersions = append(migrationVersions, m.Version)
	}

	// Write footer with metadata
	writeFooter(&bundleContent, migrationVersions)

	// Calculate checksum
	contentBytes := []byte(bundleContent.String())
	checksum := sha256.Sum256(contentBytes)
	checksumHex := hex.EncodeToString(checksum[:])

	// Replace placeholder in footer
	finalContent := strings.ReplaceAll(
		bundleContent.String(),
		"{{CHECKSUM}}",
		checksumHex,
	)

	// Ensure bundle directory exists
	if err := os.MkdirAll(filepath.Dir(bundlePath), 0755); err != nil {
		return fmt.Errorf("create bundle directory: %w", err)
	}

	// Write bundle file
	if err := os.WriteFile(bundlePath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("write bundle: %w", err)
	}

	// Validate bundle (check for required objects)
	if err := validateBundle(finalContent); err != nil {
		return fmt.Errorf("validate bundle: %w", err)
	}

	fmt.Printf("Bundle generated successfully:\n")
	fmt.Printf("  - Path: %s\n", bundlePath)
	fmt.Printf("  - Migrations: %d\n", len(migrations))
	fmt.Printf("  - Checksum: %s\n", checksumHex)
	fmt.Printf("  - Size: %d bytes\n", len(finalContent))

	return nil
}

func loadMigrations() ([]Migration, error) {
	var migrations []Migration

	// Load from archive directory
	archiveMigrations, err := loadMigrationsFromDir(archiveDir, "archive")
	if err != nil {
		return nil, fmt.Errorf("load archive migrations: %w", err)
	}
	migrations = append(migrations, archiveMigrations...)

	// Load from app directory
	appMigrations, err := loadMigrationsFromDir(appDir, "app")
	if err != nil {
		return nil, fmt.Errorf("load app migrations: %w", err)
	}
	migrations = append(migrations, appMigrations...)

	return migrations, nil
}

func loadMigrationsFromDir(dir, source string) ([]Migration, error) {
	var migrations []Migration

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Directory doesn't exist, return empty (not an error)
		return migrations, nil
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}

		// Skip README and other non-migration files
		if strings.HasPrefix(strings.ToLower(d.Name()), "readme") {
			return nil
		}

		version := strings.TrimSuffix(d.Name(), ".sql")
		migrations = append(migrations, Migration{
			Version: version,
			Name:    d.Name(),
			Path:    path,
			Source:  source,
		})

		return nil
	})

	return migrations, err
}

func writeHeader(b *strings.Builder, migrations []Migration) {
	b.WriteString("-- ============================================\n")
	b.WriteString("-- Golden Schema Bundle: app_schema_full.sql\n")
	b.WriteString("-- ============================================\n")
	b.WriteString("--\n")
	b.WriteString("-- This file contains all app database migrations\n")
	b.WriteString("-- concatenated in order for fast database provisioning.\n")
	b.WriteString("--\n")
	b.WriteString("-- Generated: " + time.Now().Format(time.RFC3339) + "\n")
	b.WriteString("-- Source: migrations from archive/ and app/ directories\n")
	b.WriteString("--\n")
	b.WriteString("-- Usage:\n")
	b.WriteString("--   1. CREATE DATABASE new_db;\n")
	b.WriteString("--   2. psql -d new_db -f bundle/app_schema_full.sql\n")
	b.WriteString("--   3. Run migrations to catch up if bundle is behind HEAD\n")
	b.WriteString("--\n")
	b.WriteString("-- Checksum: {{CHECKSUM}}\n")
	b.WriteString("--\n")
	b.WriteString("-- Migrations included:\n")
	for _, m := range migrations {
		b.WriteString(fmt.Sprintf("--   - %s (%s)\n", m.Name, m.Source))
	}
	b.WriteString("-- ============================================\n\n")
}

func writeFooter(b *strings.Builder, versions []string) {
	b.WriteString("\n-- ============================================\n")
	b.WriteString("-- Bundle End\n")
	b.WriteString("-- ============================================\n")
	b.WriteString("--\n")
	b.WriteString("-- Total migrations: " + fmt.Sprintf("%d", len(versions)) + "\n")
	b.WriteString("-- Versions: " + strings.Join(versions, ", ") + "\n")
	b.WriteString("--\n")
	b.WriteString("-- After applying this bundle, check schema_migrations table\n")
	b.WriteString("-- and apply any additional migrations if needed.\n")
	b.WriteString("-- ============================================\n")
}

// validateBundle checks that the bundle contains required database objects.
func validateBundle(content string) error {
	required := []struct {
		name        string
		description string
		patterns    []string
	}{
		{
			name:        "users table",
			description: "CREATE TABLE.*users",
			patterns:    []string{"CREATE TABLE", "users", "tenant_id"},
		},
		{
			name:        "contacts table",
			description: "CREATE TABLE.*contacts",
			patterns:    []string{"CREATE TABLE", "contacts", "tenant_id"},
		},
		{
			name:        "public_code table",
			description: "CREATE TABLE.*public_code",
			patterns:    []string{"CREATE TABLE", "public_code"},
		},
		{
			name:        "idempotency_keys table",
			description: "CREATE TABLE.*idempotency_keys",
			patterns:    []string{"CREATE TABLE", "idempotency_keys"},
		},
		{
			name:        "RLS policies",
			description: "ROW LEVEL SECURITY",
			patterns:    []string{"ROW LEVEL SECURITY", "ENABLE ROW LEVEL SECURITY"},
		},
		{
			name:        "citext extension",
			description: "CREATE EXTENSION.*citext",
			patterns:    []string{"CREATE EXTENSION", "citext"},
		},
	}

	contentLower := strings.ToLower(content)
	var missing []string

	for _, req := range required {
		found := true
		for _, pattern := range req.patterns {
			if !strings.Contains(contentLower, strings.ToLower(pattern)) {
				found = false
				break
			}
		}
		if !found {
			missing = append(missing, req.name+" ("+req.description+")")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("bundle validation failed: missing required objects:\n  - %s", strings.Join(missing, "\n  - "))
	}

	return nil
}
