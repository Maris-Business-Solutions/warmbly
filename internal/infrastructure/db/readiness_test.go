package db

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type schemaRow func(...any) error

func (r schemaRow) Scan(dest ...any) error { return r(dest...) }

type schemaProbe struct {
	versions []int
	dirty    bool
	visible  bool
	err      error
	calls    int
}

func (p *schemaProbe) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	return schemaRow(func(dest ...any) error {
		if p.err != nil {
			return p.err
		}
		if strings.Contains(query, "to_regclass") {
			*dest[0].(*bool) = p.visible
			return nil
		}
		*dest[0].(*int) = p.versions[min(p.calls, len(p.versions)-1)]
		*dest[1].(*bool) = p.dirty
		p.calls++
		return nil
	})
}

func TestSchemaReadinessWaitsForBackend(t *testing.T) {
	p := &schemaProbe{versions: []int{264, 265}, visible: true}
	if err := waitForSchema(context.Background(), p, 265, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if p.calls != 2 {
		t.Fatalf("readiness calls=%d, want 2", p.calls)
	}
}

func TestSchemaReadinessFailures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe schemaProbe
		want  string
	}{
		{"dirty", schemaProbe{versions: []int{265}, dirty: true}, "dirty=true"},
		{"old", schemaProbe{versions: []int{264}}, "require clean version"},
		{"uninitialized", schemaProbe{err: &pgconn.PgError{Code: "42P01"}}, "not initialized"},
		{"namespace mismatch", schemaProbe{versions: []int{265}}, "not visible"},
		{"query failure", schemaProbe{err: errors.New("unavailable")}, "query failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()
			if err := waitForSchema(ctx, &tc.probe, 265, time.Millisecond); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %s", err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForSchema(ctx, &schemaProbe{versions: []int{1}}, 265, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}

func TestRequiredSchemaIncludesEveryEmbeddedMigration(t *testing.T) {
	version, err := requiredMigrationVersion()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	last := paths[len(paths)-1]
	if !strings.HasPrefix(last, "migrations/000") || !strings.Contains(last, "_") || version < 265 {
		t.Fatalf("latest=%s required=%d", last, version)
	}
}

type migrationState struct {
	version int
	dirty   bool
	err     error
}

func (s migrationState) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	*dest[0].(*int) = s.version
	*dest[1].(*bool) = s.dirty
	return nil
}

type migrationStates struct {
	states []migrationState
	calls  int
	cancel context.CancelFunc
}

func (s *migrationStates) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	if strings.Contains(query, "to_regclass") {
		return schemaRow(func(dest ...any) error {
			*dest[0].(*bool) = true
			return nil
		})
	}
	state := s.states[min(s.calls, len(s.states)-1)]
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	return state
}

func TestSchemaReadinessMigrationProgress(t *testing.T) {
	for _, tc := range []struct {
		name   string
		states []migrationState
	}{
		{"current", []migrationState{{version: 265}}},
		{"newer", []migrationState{{version: 266}}},
		{"upgrade", []migrationState{{version: 264}, {version: 265, dirty: true}, {version: 265}}},
		{"fresh install", []migrationState{{err: pgx.ErrNoRows}, {version: 265}}},
		{"uninitialized", []migrationState{{err: &pgconn.PgError{Code: "42P01"}}, {version: 265}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := &migrationStates{states: tc.states}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := waitForSchema(ctx, pool, 265, time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if pool.calls != len(tc.states) {
				t.Fatalf("queried %d times, want %d", pool.calls, len(tc.states))
			}
		})
	}
}

func TestSchemaReadinessHonorsCanceledContext(t *testing.T) {
	for _, state := range []migrationState{{version: 264}, {version: 265, dirty: true}, {err: pgx.ErrNoRows}, {version: 265}} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		pool := &migrationStates{states: []migrationState{state}}
		err := waitForSchema(ctx, pool, 265, time.Millisecond)
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "upgrade the backend") {
			t.Fatalf("expected actionable cancellation, got %v", err)
		}
		if pool.calls != 0 {
			t.Fatalf("canceled readiness check queried the database %d times", pool.calls)
		}
	}
}

func TestSchemaReadinessPreservesDatabaseError(t *testing.T) {
	schemaErr := &pgconn.PgError{Code: "42P01", Message: "relation schema_migrations does not exist"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := &migrationStates{states: []migrationState{{err: schemaErr}}, cancel: cancel}
	err := waitForSchema(ctx, pool, 265, time.Millisecond)
	if !errors.Is(err, schemaErr) || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), schemaErr.Message) {
		t.Fatalf("original database error was lost: %v", err)
	}
}

func TestSchemaReadinessTimeoutPreservesObservedSchemaState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := &migrationStates{states: []migrationState{{version: 265, dirty: true}}, cancel: cancel}
	err := waitForSchema(ctx, pool, 265, time.Millisecond)
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "version 265 (dirty=true)") {
		t.Fatalf("observed migration state was lost: %v", err)
	}
}

func TestSchemaReadinessCanceledQueryPreservesPreviousFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := &migrationStates{states: []migrationState{{err: pgx.ErrNoRows}, {err: context.Canceled}}}
	pool.cancel = func() {
		if pool.calls == 2 {
			cancel()
		}
	}
	err := waitForSchema(ctx, pool, 265, time.Millisecond)
	if !errors.Is(err, pgx.ErrNoRows) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query masked the previous migration error: %v", err)
	}
}
