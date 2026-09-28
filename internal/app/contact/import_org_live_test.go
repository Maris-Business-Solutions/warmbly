package contact

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// addMember creates a user who belongs to the fixture's organization.
func (f *importFixture) addMember(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Team', 'Mate', $2, 'x')`,
			[]any{id, "mate-" + id.String()[:8] + "@test.local"}},
		{`INSERT INTO organization_members (organization_id, user_id, role, accepted_at) VALUES ($1, $2, 'member', NOW())`,
			[]any{f.org, id}},
	} {
		if _, err := f.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("add member: %v", err)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = f.pool.Exec(c, `DELETE FROM contacts WHERE user_id = $1`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM organization_members WHERE user_id = $1`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

// otherWorkspace creates a second organization the fixture's user belongs to.
func (f *importFixture) otherWorkspace(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Other', $2, $3)`,
		id, "other-"+id.String()[:8], f.user); err != nil {
		t.Fatalf("other workspace: %v", err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO organization_members (organization_id, user_id, role, accepted_at) VALUES ($1, $2, 'owner', NOW())`,
		id, f.user); err != nil {
		t.Fatalf("other membership: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = f.pool.Exec(c, `DELETE FROM contacts WHERE organization_id = $1`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM organization_members WHERE organization_id = $1`, id)
		_, _ = f.pool.Exec(c, `DELETE FROM organizations WHERE id = $1`, id)
	})
	return id
}

func (f *importFixture) insertContact(t *testing.T, owner, org uuid.UUID, addr, first string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO contacts (id, user_id, organization_id, first_name, last_name, email, company, phone, custom_fields)
		VALUES ($1, $2, $3, $4, '', $5, '', '', '{}'::jsonb)`, id, owner, org, first, addr); err != nil {
		t.Fatalf("insert contact: %v", err)
	}
	return id
}

func (f *importFixture) countByEmail(t *testing.T, org uuid.UUID, addr string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM contacts WHERE organization_id = $1 AND LOWER(email) = $2`, org, addr).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// A contact is the workspace's, whoever created it: importing a list that
// holds a teammate's contact must not create that person a second time.
func TestLiveImportSkipsATeammatesContact(t *testing.T) {
	f := newImportFixture(t)
	mate := f.addMember(t)
	f.insertContact(t, mate, f.org, "dana@teammate.test", "Dana")

	res, msg := f.commit(t, "Email,First Name\ndana@teammate.test,Dana\nnew@teammate.test,New\n", &models.ContactImportCommit{
		Mapping:   []models.ContactImportColumnMapping{col(0, models.ContactImportTargetEmail), col(1, models.ContactImportTargetFirstName)},
		Dedup:     models.ContactImportDedupSkip,
		HasHeader: true,
	})
	if msg != "" {
		t.Fatalf("import rejected: %s", msg)
	}
	if res.Imported != 1 || res.Skipped != 1 {
		t.Fatalf("imported=%d skipped=%d (want 1 and 1)", res.Imported, res.Skipped)
	}
	if n := f.countByEmail(t, f.org, "dana@teammate.test"); n != 1 {
		t.Fatalf("the workspace holds %d contacts for the teammate's address (want 1)", n)
	}
}

func TestLiveImportUpdateReachesATeammatesContact(t *testing.T) {
	f := newImportFixture(t)
	mate := f.addMember(t)
	id := f.insertContact(t, mate, f.org, "dana@teammate.test", "Dana")

	res, msg := f.commit(t, "Email,Company\ndana@teammate.test,Acme\n", &models.ContactImportCommit{
		Mapping:   []models.ContactImportColumnMapping{col(0, models.ContactImportTargetEmail), col(1, models.ContactImportTargetCompany)},
		Dedup:     models.ContactImportDedupUpdate,
		HasHeader: true,
	})
	if msg != "" {
		t.Fatalf("import rejected: %s", msg)
	}
	if res.Updated != 1 || res.Imported != 0 {
		t.Fatalf("updated=%d imported=%d (want 1 and 0)", res.Updated, res.Imported)
	}
	var company, first string
	if err := f.pool.QueryRow(context.Background(), `SELECT company, first_name FROM contacts WHERE id = $1`, id).Scan(&company, &first); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if company != "Acme" || first != "Dana" {
		t.Fatalf("company=%q first=%q (want Acme, Dana kept)", company, first)
	}
}

// One unusable row fails alone; the rows written in the same batch still land.
func TestLiveImportOneBadRowDoesNotFailItsChunk(t *testing.T) {
	f := newImportFixture(t)
	huge := strings.Repeat("x", 11000)
	csv := fmt.Sprintf("Email,Notes\nok1@chunk.test,fine\nbig@chunk.test,%s\nok2@chunk.test,fine\n", huge)

	res, msg := f.commit(t, csv, &models.ContactImportCommit{
		Mapping:   []models.ContactImportColumnMapping{col(0, models.ContactImportTargetEmail), customCol(1, "Notes")},
		HasHeader: true,
	})
	if msg != "" {
		t.Fatalf("import rejected: %s", msg)
	}
	if res.Imported != 2 || res.Failed != 1 {
		t.Fatalf("imported=%d failed=%d (want 2 and 1)", res.Imported, res.Failed)
	}
	if len(res.Errors) != 1 || res.Errors[0].Line != 3 {
		t.Fatalf("errors=%+v (want line 3 only)", res.Errors)
	}
}

// An imported verdict applies to an existing contact too, not only to new ones.
func TestLiveImportUpdateKeepsTheVerdict(t *testing.T) {
	f := newImportFixture(t)
	id := f.insertContact(t, f.user, f.org, "verdict@update.test", "V")

	res, msg := f.commit(t, "Email,Status\nverdict@update.test,valid\n", &models.ContactImportCommit{
		Mapping: []models.ContactImportColumnMapping{
			col(0, models.ContactImportTargetEmail),
			{Index: 1, Target: models.ContactImportTargetVerificationStatus, VerificationProvider: "zerobounce"},
		},
		Dedup:     models.ContactImportDedupUpdate,
		HasHeader: true,
	})
	if msg != "" {
		t.Fatalf("import rejected: %s", msg)
	}
	if res.Updated != 1 {
		t.Fatalf("updated=%d (want 1): %+v", res.Updated, res.Errors)
	}
	var status, source string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT verification_status, verification_source FROM contacts WHERE id = $1`, id).Scan(&status, &source); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "valid" || source != "imported" {
		t.Fatalf("verification_status=%q source=%q (want valid, imported)", status, source)
	}
}

// The unique index is per member, so the same member's contact in another
// workspace cannot be created here, and is never written from here.
func TestLiveImportLeavesAnotherWorkspaceAlone(t *testing.T) {
	f := newImportFixture(t)
	other := f.otherWorkspace(t)
	theirs := f.insertContact(t, f.user, other, "shared@elsewhere.test", "Keep")

	res, msg := f.commit(t, "Email,First Name\nshared@elsewhere.test,Changed\nfresh@elsewhere.test,Fresh\n", &models.ContactImportCommit{
		Mapping:   []models.ContactImportColumnMapping{col(0, models.ContactImportTargetEmail), col(1, models.ContactImportTargetFirstName)},
		Dedup:     models.ContactImportDedupUpdate,
		HasHeader: true,
	})
	if msg != "" {
		t.Fatalf("import rejected: %s", msg)
	}
	if res.Imported != 1 || res.Failed != 1 {
		t.Fatalf("imported=%d failed=%d (want 1 and 1)", res.Imported, res.Failed)
	}

	// The repository refuses the write on its own as well.
	_, xerr := f.repo.Add(context.Background(), f.user.String(), f.org, []models.AddContact{{Email: "shared@elsewhere.test", FirstName: "Changed"}})
	if xerr == nil || xerr.Code != errx.Conflict {
		t.Fatalf("add over another workspace's contact: %v (want a conflict)", xerr)
	}

	var first string
	var org uuid.UUID
	if err := f.pool.QueryRow(context.Background(),
		`SELECT first_name, organization_id FROM contacts WHERE id = $1`, theirs).Scan(&first, &org); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if first != "Keep" || org != other {
		t.Fatalf("the other workspace's contact changed: first=%q org=%s", first, org)
	}
	if n := f.countByEmail(t, f.org, "shared@elsewhere.test"); n != 0 {
		t.Fatalf("this workspace holds %d copies of the other workspace's contact", n)
	}
}

func TestLiveAnalyzeImportBuckets(t *testing.T) {
	f := newImportFixture(t)
	f.insertContact(t, f.user, f.org, "old@analyze.test", "Old")
	rows := []ImportRow{
		{Line: 2, Cells: []string{"old@analyze.test"}},
		{Line: 3, Cells: []string{"new@analyze.test"}},
		{Line: 4, Cells: []string{"NEW@analyze.test"}},
		{Line: 5, Cells: []string{"not-an-address"}},
	}
	got, xerr := f.svc.AnalyzeImport(context.Background(), f.user.String(), f.org, rows,
		[]models.ContactImportColumnMapping{col(0, models.ContactImportTargetEmail)})
	if xerr != nil {
		t.Fatalf("analyze: %s", xerr.Message)
	}
	if got.Rows != 4 || got.Existing != 1 || got.New != 1 || got.DuplicatesInFile != 1 || got.Invalid != 1 {
		t.Fatalf("analysis=%+v", got)
	}
	if len(got.InvalidSamples) != 1 || got.InvalidSamples[0].Line != 5 {
		t.Fatalf("invalid samples=%+v", got.InvalidSamples)
	}
}
