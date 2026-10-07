package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type schemaReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func requiredMigrationVersion() (int, error) {
	paths, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		return 0, err
	}
	version := 0
	for _, path := range paths {
		base := strings.TrimPrefix(path, "migrations/")
		prefix, _, _ := strings.Cut(base, "_")
		n, err := strconv.Atoi(prefix)
		if err != nil {
			return 0, fmt.Errorf("invalid embedded migration version: %w", err)
		}
		version = max(version, n)
	}
	if version == 0 {
		return 0, errors.New("no embedded migrations")
	}
	return version, nil
}

// WaitForSchema keeps consumers from starting jobs ahead of the backend's migrations.
func WaitForSchema(ctx context.Context, reader schemaReader) error {
	required, err := requiredMigrationVersion()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return waitForSchema(ctx, reader, required, time.Second)
}

func waitForSchema(ctx context.Context, reader schemaReader, required int, interval time.Duration) error {
	state := "migrations have not completed"
	for {
		var version int
		var dirty bool
		err := reader.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty)
		var pgErr *pgconn.PgError
		switch {
		case err == nil:
			state = fmt.Sprintf("database version %d (dirty=%t), require clean version >= %d", version, dirty, required)
			if !dirty && version >= required {
				var visible bool
				if err := reader.QueryRow(ctx, `SELECT to_regclass('warmup_pending_filings') IS NOT NULL AND to_regclass('warmup_recovery_identifiers') IS NOT NULL`).Scan(&visible); err != nil {
					return fmt.Errorf("consumer schema visibility check failed: %w", err)
				}
				if !visible {
					return errors.New("consumer schema version is current but required warmup relations are not visible; check database and search_path")
				}
				return nil
			}
		case errors.Is(err, pgx.ErrNoRows), errors.As(err, &pgErr) && pgErr.Code == "42P01":
			state = "schema_migrations is not initialized"
		default:
			return fmt.Errorf("consumer schema readiness query failed: %w", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("consumer waiting for backend migrations: %s: %w", state, ctx.Err())
		case <-timer.C:
		}
	}
}
