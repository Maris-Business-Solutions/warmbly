package orgtransfer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/infrastructure/db"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// An import only writes rows of the destination workspace, and every reference
// it writes resolves to a row of that workspace.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/app/orgtransfer/ -run LiveImport -v

type tenantFixture struct {
	owner, org, contact, campaign uuid.UUID
}

func liveImportService(t *testing.T) (Service, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("WARMBLY_TEST_DB")
	if dsn == "" {
		t.Skip("WARMBLY_TEST_DB not set")
	}
	handle, err := db.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { handle.Pool.Close() })
	return NewService(repository.NewOrgTransferRepository(handle), nil, nil, nil, InstanceInfo{}), handle.Pool
}

func newTenantFixture(t *testing.T, pool *pgxpool.Pool) tenantFixture {
	t.Helper()
	ctx := context.Background()
	f := tenantFixture{owner: uuid.New(), org: uuid.New(), contact: uuid.New(), campaign: uuid.New()}
	tag := f.org.String()[:8]
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Owner', 'Live', $2, 'x')`,
			[]any{f.owner, "xfer-" + tag + "@test.local"}},
		{`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Transfer', $2, $3)`,
			[]any{f.org, "xfer-" + tag, f.owner}},
		{`INSERT INTO organization_members (organization_id, user_id, role, accepted_at) VALUES ($1, $2, 'owner', NOW())`,
			[]any{f.org, f.owner}},
		{`INSERT INTO contacts (id, user_id, organization_id, email, first_name, last_name, company, phone, custom_fields)
		  VALUES ($1, $2, $3, $4, 'Kept', 'Here', '', '', '{}'::jsonb)`,
			[]any{f.contact, f.owner, f.org, "kept-" + tag + "@test.local"}},
		{`INSERT INTO campaigns (id, user_id, organization_id, name, description, days, updated_at, created_at)
		  VALUES ($1, $2, $3, 'Kept', '', 62, NOW(), NOW())`,
			[]any{f.campaign, f.owner, f.org}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		for _, q := range []string{
			`DELETE FROM campaign_leads WHERE campaign_id IN (SELECT id FROM campaigns WHERE organization_id = $1)`,
			`DELETE FROM campaigns WHERE organization_id = $1`,
			`DELETE FROM contacts WHERE organization_id = $1`,
			`DELETE FROM organization_members WHERE organization_id = $1`,
			`DELETE FROM organizations WHERE id = $1`,
		} {
			if _, err := pool.Exec(c, q, f.org); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
		if _, err := pool.Exec(c, `DELETE FROM users WHERE id = $1`, f.owner); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

// tenantArchive builds an archive holding the given rows per table.
func tenantArchive(t *testing.T, tables map[string][]map[string]any) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	m := Manifest{
		Kind:           ArchiveKind,
		FormatVersion:  models.OrgTransferFormatVersion,
		OrganizationID: uuid.New(),
		ExportedAt:     time.Now(),
	}
	for _, tbl := range Tables {
		rows, ok := tables[tbl.Name]
		if !ok {
			continue
		}
		w, err := zw.Create(dataPath(tbl.Name))
		if err != nil {
			t.Fatal(err)
		}
		cols := map[string]bool{}
		for _, r := range rows {
			line, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(append(line, '\n')); err != nil {
				t.Fatal(err)
			}
			for c := range r {
				cols[c] = true
			}
		}
		mt := ManifestTable{Name: tbl.Name, Group: tbl.Group, Rows: int64(len(rows))}
		for c := range cols {
			mt.Columns = append(mt.Columns, c)
		}
		m.Tables = append(m.Tables, mt)
	}
	w, err := zw.Create(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(w).Encode(m); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func contactRow(id, org uuid.UUID, email string) map[string]any {
	return map[string]any{
		"id": id, "user_id": uuid.New(), "organization_id": org, "email": email,
		"first_name": "Archive", "last_name": "Row", "company": "", "phone": "", "custom_fields": map[string]any{},
	}
}

func TestLiveImportRefusesRowsOwnedByAnotherWorkspace(t *testing.T) {
	svc, pool := liveImportService(t)
	ctx := context.Background()
	other := newTenantFixture(t, pool)
	dest := newTenantFixture(t, pool)

	for _, conflict := range []models.OrgImportConflict{models.OrgImportConflictOverwrite, models.OrgImportConflictSkip} {
		archive := tenantArchive(t, map[string][]map[string]any{
			"contacts": {contactRow(other.contact, other.org, "moved@test.local")},
		})
		_, err := svc.ImportFrom(ctx, dest.org, archive, ImportOptions{Conflict: conflict, ActorUserID: dest.owner}, nil)
		if !errors.Is(err, ErrForeignRows) {
			t.Fatalf("%s: want ErrForeignRows, got %v", conflict, err)
		}
		var org uuid.UUID
		var email string
		if err := pool.QueryRow(ctx, `SELECT organization_id, email FROM contacts WHERE id = $1`, other.contact).Scan(&org, &email); err != nil {
			t.Fatal(err)
		}
		if org != other.org || email == "moved@test.local" {
			t.Fatalf("%s: another workspace's contact was changed: org %s email %s", conflict, org, email)
		}
	}
}

func TestLiveImportRefusesReferencesIntoAnotherWorkspace(t *testing.T) {
	svc, pool := liveImportService(t)
	ctx := context.Background()
	other := newTenantFixture(t, pool)
	dest := newTenantFixture(t, pool)

	fresh := uuid.New()
	archive := tenantArchive(t, map[string][]map[string]any{
		"contacts":       {contactRow(fresh, uuid.New(), "fresh-"+fresh.String()[:8]+"@test.local")},
		"campaign_leads": {{"campaign_id": other.campaign, "contact_id": fresh, "source": "manual"}},
	})
	_, err := svc.ImportFrom(ctx, dest.org, archive, ImportOptions{Conflict: models.OrgImportConflictSkip, ActorUserID: dest.owner}, nil)
	if !errors.Is(err, ErrForeignRows) {
		t.Fatalf("want ErrForeignRows, got %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM campaign_leads WHERE campaign_id = $1`, other.campaign).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a lead landed on another workspace's campaign")
	}
}

func TestLiveImportOverwritesItsOwnRows(t *testing.T) {
	svc, pool := liveImportService(t)
	ctx := context.Background()
	dest := newTenantFixture(t, pool)

	archive := tenantArchive(t, map[string][]map[string]any{
		"contacts":       {contactRow(dest.contact, uuid.New(), "rerun-"+dest.contact.String()[:8]+"@test.local")},
		"campaign_leads": {{"campaign_id": dest.campaign, "contact_id": dest.contact, "source": "manual"}},
	})
	if _, err := svc.ImportFrom(ctx, dest.org, archive, ImportOptions{Conflict: models.OrgImportConflictOverwrite, ActorUserID: dest.owner}, nil); err != nil {
		t.Fatalf("re-running an import into its own workspace: %v", err)
	}
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM contacts WHERE id = $1 AND organization_id = $2`, dest.contact, dest.org).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if email != "rerun-"+dest.contact.String()[:8]+"@test.local" {
		t.Fatalf("own row not overwritten: %s", email)
	}
}
