package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"blog/internal/config"
	"blog/internal/platform/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationVersion int64 = 1

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("load configuration", "error", err)
		os.Exit(1)
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}
	if direction != "up" && direction != "down" {
		log.Error("invalid migration direction", "expected", "up or down")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := database.Open(ctx, cfg.DatabaseURL, 0, 2)
	if err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrate(ctx, db, direction); err != nil {
		log.Error("migration failed", "direction", direction, "error", err)
		os.Exit(1)
	}
	log.Info("migration complete", "direction", direction, "version", migrationVersion)
}

func migrate(ctx context.Context, db *pgxpool.Pool, direction string) error {
	file := fmt.Sprintf("%06d_init.%s.sql", migrationVersion, direction)
	sql, err := os.ReadFile(filepath.Join("migrations", file))
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('blog_schema_migrations'))`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	var applied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, migrationVersion).Scan(&applied); err != nil {
		return fmt.Errorf("check migration version: %w", err)
	}
	if direction == "up" && applied {
		return nil
	}
	if direction == "down" && !applied {
		return nil
	}

	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("execute %s: %w", file, err)
	}
	if direction == "up" {
		_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migrationVersion)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version=$1`, migrationVersion)
	}
	if err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
