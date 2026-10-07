package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type migrationState struct {
	version int64
	dirty   bool
	err     error
}

func (s migrationState) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	*dest[0].(*int64) = s.version
	*dest[1].(*bool) = s.dirty
	return nil
}

type migrationStates struct {
	states []migrationState
	calls  int
	cancel context.CancelFunc
}

func (s *migrationStates) QueryRow(context.Context, string, ...any) pgx.Row {
	state := s.states[min(s.calls, len(s.states)-1)]
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	return state
}

func TestWaitForMigrations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		states []migrationState
	}{
		{"current", []migrationState{{version: 265}}},
		{"newer", []migrationState{{version: 266}}},
		{"upgrade", []migrationState{{version: 264}, {version: 265, dirty: true}, {version: 265}}},
		{"fresh install", []migrationState{{err: pgx.ErrNoRows}, {version: 265}}},
		{"unavailable", []migrationState{{err: errors.New("missing schema_migrations")}, {version: 265}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := &migrationStates{states: tc.states}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := waitForMigrations(ctx, pool, 265, time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if pool.calls != len(tc.states) {
				t.Fatalf("queried %d times, want %d", pool.calls, len(tc.states))
			}
		})
	}
}

func TestWaitForMigrationsTimeout(t *testing.T) {
	for _, state := range []migrationState{{version: 264}, {version: 265, dirty: true}, {err: pgx.ErrNoRows}, {version: 265}} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		pool := &migrationStates{states: []migrationState{state}}
		err := waitForMigrations(ctx, pool, 265, time.Millisecond)
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "upgrade the backend") {
			t.Fatalf("expected actionable cancellation, got %v", err)
		}
		if pool.calls != 0 {
			t.Fatalf("canceled readiness check queried the database %d times", pool.calls)
		}
	}
}

func TestWaitForMigrationsPreservesDatabaseError(t *testing.T) {
	schemaErr := errors.New("relation schema_migrations does not exist")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := &migrationStates{states: []migrationState{{err: schemaErr}}, cancel: cancel}
	err := waitForMigrations(ctx, pool, 265, time.Millisecond)
	if !errors.Is(err, schemaErr) || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), schemaErr.Error()) {
		t.Fatalf("original database error was lost: %v", err)
	}
}

func TestWaitForMigrationsTimeoutPreservesObservedSchemaState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := &migrationStates{states: []migrationState{{version: 265, dirty: true}}, cancel: cancel}
	err := waitForMigrations(ctx, pool, 265, time.Millisecond)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "migration 265 (dirty=true)") {
		t.Fatalf("observed migration state was lost: %v", err)
	}
}

func TestRequiredMigrationVersionTracksEmbeddedFiles(t *testing.T) {
	v, err := requiredMigrationVersion()
	if err != nil || v < 265 {
		t.Fatalf("required migration = %d, err=%v", v, err)
	}
}
