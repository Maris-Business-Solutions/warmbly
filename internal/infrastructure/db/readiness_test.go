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
