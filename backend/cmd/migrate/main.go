package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/config"
	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/db"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) != 2 {
		logger.Error("usage: migrate up|down|status")
		os.Exit(2)
	}

	cfg, err := config.LoadDatabase()
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ensureSchemaMigrations(ctx, database); err != nil {
		logger.Error("ensure schema migrations", "error", err)
		os.Exit(1)
	}

	migrations, err := loadMigrations("migrations")
	if err != nil {
		logger.Error("load migrations", "error", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "up":
		err = migrateUp(ctx, database, migrations, logger)
	case "down":
		err = migrateDown(ctx, database, migrations, logger)
	case "status":
		err = printStatus(ctx, database, migrations)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		logger.Error("migration command failed", "command", os.Args[1], "error", err)
		os.Exit(1)
	}
}

type migration struct {
	Version string
	Name    string
	Path    string
	UpSQL   string
	DownSQL string
}

func ensureSchemaMigrations(ctx context.Context, database *sql.DB) error {
	_, err := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	return err
}

func loadMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		upSQL, downSQL, err := splitMigration(string(content))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, migration{
			Version: version,
			Name:    name,
			Path:    path,
			UpSQL:   upSQL,
			DownSQL: downSQL,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func splitMigration(content string) (string, string, error) {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"

	upIndex := strings.Index(content, upMarker)
	downIndex := strings.Index(content, downMarker)
	if upIndex == -1 || downIndex == -1 || downIndex <= upIndex {
		return "", "", errors.New("migration must contain ordered goose Up and Down markers")
	}

	upSQL := strings.TrimSpace(content[upIndex+len(upMarker) : downIndex])
	downSQL := strings.TrimSpace(content[downIndex+len(downMarker):])
	if upSQL == "" || downSQL == "" {
		return "", "", errors.New("migration up and down SQL must be non-empty")
	}

	return upSQL, downSQL, nil
}

func parseMigrationName(filename string) (string, string, error) {
	name := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid migration filename %q", filename)
	}
	return parts[0], parts[1], nil
}

func migrateUp(ctx context.Context, database *sql.DB, migrations []migration, logger *slog.Logger) error {
	applied, err := appliedVersions(ctx, database)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}

		if err := applyMigration(ctx, database, migration, true); err != nil {
			return err
		}
		logger.Info("migration applied", "version", migration.Version, "name", migration.Name)
	}

	return nil
}

func migrateDown(ctx context.Context, database *sql.DB, migrations []migration, logger *slog.Logger) error {
	applied, err := appliedVersions(ctx, database)
	if err != nil {
		return err
	}

	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if !applied[migration.Version] {
			continue
		}

		if err := applyMigration(ctx, database, migration, false); err != nil {
			return err
		}
		logger.Info("migration reverted", "version", migration.Version, "name", migration.Name)
		return nil
	}

	logger.Info("no applied migrations to revert")
	return nil
}

func applyMigration(ctx context.Context, database *sql.DB, migration migration, up bool) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := migration.UpSQL
	if !up {
		query = migration.DownSQL
	}

	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("execute migration %s: %w", migration.Path, err)
	}

	if up {
		_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, migration.Version, migration.Name)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, migration.Version)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func appliedVersions(ctx context.Context, database *sql.DB) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func printStatus(ctx context.Context, database *sql.DB, migrations []migration) error {
	applied, err := appliedVersions(ctx, database)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		state := "pending"
		if applied[migration.Version] {
			state = "applied"
		}
		fmt.Printf("%s %s %s\n", migration.Version, state, migration.Name)
	}

	return nil
}
