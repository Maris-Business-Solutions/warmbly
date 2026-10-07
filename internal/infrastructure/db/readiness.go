package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type migrationQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func requiredMigrationVersion() (int64, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return 0, err
	}
	var required int64
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		v, err := strconv.ParseInt(strings.SplitN(entry.Name(), "_", 2)[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid embedded migration filename: %w", err)
		}
		required = max(required, v)
	}
	return required, nil
}

// WaitForMigrations keeps consumers idle until the backend finishes its migrations.
func WaitForMigrations(ctx context.Context, pool migrationQuerier) error {
	required, err := requiredMigrationVersion()
	if err != nil {
		return err
	}
	return waitForMigrations(ctx, pool, required, 2*time.Second)
}

func waitForMigrations(ctx context.Context, pool migrationQuerier, required int64, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	state := "migration state unavailable"
	var queryErr error
	for ctx.Err() == nil {
		var version int64
		var dirty bool
		if err := pool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err == nil {
			queryErr = nil
			if version >= required && !dirty && ctx.Err() == nil {
				return nil
			}
			state = fmt.Sprintf("database migration %d (dirty=%t)", version, dirty)
		} else {
			queryErr = err
			state = "migration state unavailable"
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	return fmt.Errorf("%s; consumer requires migration %d or newer and a clean schema: upgrade the backend and check its migration logs: %w", state, required, errors.Join(ctx.Err(), queryErr))
}
