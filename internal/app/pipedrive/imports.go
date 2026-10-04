package pipedrive

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// maxListImport bounds one filter import; a bigger filter imports in parts.
const maxListImport = 25000

// Lists returns the company's saved people filters, most recently changed
// first. Pipedrive's filters are what its users build lists with.
func (s *Service) Lists(ctx context.Context, orgID uuid.UUID, query, cursor string, limit int) (*models.CRMListsResult, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	offset := 0
	if cursor != "" {
		raw, derr := base64.RawURLEncoding.DecodeString(cursor)
		n, err := strconv.Atoi(strings.TrimPrefix(string(raw), "filter:"))
		if derr != nil || err != nil || n < 0 || !strings.HasPrefix(string(raw), "filter:") {
			return nil, errx.New(errx.BadRequest, "invalid cursor")
		}
		offset = n
	}
	filters, err := o.Client.PersonFilters(ctx)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var matched []Filter
	for _, f := range filters {
		if f.ActiveFlag && (q == "" || strings.Contains(strings.ToLower(f.Name), q)) {
			matched = append(matched, f)
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].UpdateTime > matched[j].UpdateTime })
	out := &models.CRMListsResult{Data: []models.CRMList{}}
	end := min(offset+limit, len(matched))
	for i := offset; i < end; i++ {
		f := matched[i]
		out.Data = append(out.Data, models.CRMList{ExternalID: id(f.ID), Name: f.Name, Size: -1, Dynamic: true, UpdatedAt: f.UpdateTime})
	}
	out.Pagination = models.Pagination{HasMore: end < len(matched)}
	if out.Pagination.HasMore {
		c := base64.RawURLEncoding.EncodeToString([]byte("filter:" + strconv.Itoa(end)))
		out.Pagination.NextCursor = &c
	}
	return out, nil
}

// importRow is one filter member headed for Warmbly.
type importRow struct {
	ID                                              int64
	Email, FirstName, LastName, Company, Phone, Own string
	Labels                                          []string
	OptedOut                                        bool
}

// filterName is a filter's name, read from the list.
func (s *Service) filterName(ctx context.Context, o *org, filterID string) (string, *errx.Error) {
	filters, err := o.Client.PersonFilters(ctx)
	if err != nil {
		return "", s.userError(ctx, o, err)
	}
	for _, f := range filters {
		if id(f.ID) == filterID {
			return f.Name, nil
		}
	}
	return "", errx.New(errx.NotFound, "That Pipedrive filter no longer exists.")
}

// collect reads a filter's people and sorts them into kept and skipped.
func (s *Service) collect(ctx context.Context, o *org, req *models.CRMImportRequest) (string, []importRow, *models.CRMImportPreview, *errx.Error) {
	if extID(req.ListID) == 0 {
		return "", nil, nil, errx.New(errx.BadRequest, "choose a Pipedrive filter")
	}
	name, xerr := s.filterName(ctx, o, req.ListID)
	if xerr != nil {
		return "", nil, nil, xerr
	}
	persons, err := o.Client.Persons(ctx, url.Values{"filter_id": {req.ListID}}, maxListImport+1)
	if err != nil {
		return "", nil, nil, s.userError(ctx, o, err)
	}
	truncated := len(persons) > maxListImport
	if truncated {
		persons = persons[:maxListImport]
	}
	preview := &models.CRMImportPreview{ListName: name, Total: len(persons), Truncated: truncated,
		Skipped: []models.CRMImportSkip{}, Sample: []models.CRMImportPerson{}}
	g := o.Config.Guards
	mapped := map[string]bool{}
	if req.ApplyGuards && g.SkipOtherOwners {
		owners, _ := s.d.Repo.ListOwners(ctx, o.ID, provider)
		for _, ow := range owners {
			if ow.UserID != nil {
				mapped[ow.ExternalID] = true
			}
		}
	}
	// Open deals are read from the mirror: a person with a mirrored open deal.
	openDeal := map[int64]bool{}
	if req.ApplyGuards && g.SkipOpenDeals {
		ids := make([]int64, 0, len(persons))
		for _, p := range persons {
			ids = append(ids, p.ID)
		}
		openDeal = s.openDealPeople(ctx, o, ids)
	}
	orgNames := s.orgNames(ctx, o, persons)
	skips := map[string]*models.CRMImportSkip{}
	skip := func(reason, label string) {
		if skips[reason] == nil {
			skips[reason] = &models.CRMImportSkip{Reason: reason, Label: label}
		}
		skips[reason].Count++
	}
	var rows []importRow
	seen := map[string]bool{}
	for i := range persons {
		p := &persons[i]
		r := importRow{ID: p.ID, Email: p.Email(), FirstName: p.FirstName, LastName: p.LastName, Phone: p.Phone(),
			Own: ptrID(p.OwnerID), OptedOut: strings.EqualFold(p.MarketingStatus, "unsubscribed")}
		if p.OrgID != nil {
			r.Company = orgNames[*p.OrgID]
		}
		for _, l := range p.LabelIDs {
			r.Labels = append(r.Labels, id(l))
		}
		if r.FirstName == "" && r.LastName == "" {
			r.FirstName, r.LastName, _ = strings.Cut(strings.TrimSpace(p.Name), " ")
		}
		switch {
		case r.Email == "" || !strings.Contains(r.Email, "@"):
			skip("no_email", "No email address")
			continue
		case seen[r.Email]:
			skip("duplicate", "Duplicate email")
			continue
		}
		seen[r.Email] = true
		if req.ApplyGuards {
			blocked := ""
			for _, l := range r.Labels {
				if slices.Contains(g.SkipLifecycleStages, l) {
					blocked = l
					break
				}
			}
			switch {
			case g.SkipOptedOut && r.OptedOut:
				skip("opted_out", "Unsubscribed in Pipedrive")
				continue
			case blocked != "":
				skip("lifecycle", "Labeled "+s.labelName(ctx, o, blocked))
				continue
			case g.SkipOpenDeals && openDeal[r.ID]:
				skip("open_deal", "Has an open deal")
				continue
			case g.SkipOtherOwners && r.Own != "" && !mapped[r.Own]:
				skip("other_owner", "Owned by someone outside this workspace")
				continue
			}
		}
		rows = append(rows, r)
	}
	preview.Included = len(rows)
	for _, k := range []string{"opted_out", "lifecycle", "open_deal", "other_owner", "no_email", "duplicate"} {
		if sk := skips[k]; sk != nil {
			preview.Skipped = append(preview.Skipped, *sk)
		}
	}
	for i := 0; i < len(rows) && i < 5; i++ {
		preview.Sample = append(preview.Sample, models.CRMImportPerson{Email: rows[i].Email, FirstName: rows[i].FirstName,
			LastName: rows[i].LastName, Company: rows[i].Company})
	}
	return name, rows, preview, nil
}

// openDealPeople reports which persons have an open deal Warmbly mirrors.
func (s *Service) openDealPeople(ctx context.Context, o *org, people []int64) map[int64]bool {
	out := map[int64]bool{}
	local := map[uuid.UUID]int64{}
	var contactIDs []uuid.UUID
	for _, pid := range people {
		if rec, _ := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id(pid)); rec != nil {
			local[rec.ContactID] = pid
			contactIDs = append(contactIDs, rec.ContactID)
		}
	}
	open, err := s.d.Repo.OpenDealContactIDs(ctx, o.ID, contactIDs)
	if err != nil {
		return out
	}
	for cid := range open {
		out[local[cid]] = true
	}
	return out
}

// orgNames reads the organizations a page of persons belongs to.
func (s *Service) orgNames(ctx context.Context, o *org, persons []Person) map[int64]string {
	out := map[int64]string{}
	var ids []string
	seen := map[int64]bool{}
	for _, p := range persons {
		if p.OrgID != nil && !seen[*p.OrgID] {
			seen[*p.OrgID] = true
			ids = append(ids, id(*p.OrgID))
		}
	}
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		var page []Organization
		if _, err := o.Client.call(ctx, "GET", "/api/v2/organizations", url.Values{"ids": {strings.Join(ids[start:end], ",")}, "limit": {"100"}}, nil, &page); err != nil {
			return out
		}
		for _, org := range page {
			out[org.ID] = org.Name
		}
	}
	return out
}

// PreviewImport counts who a filter import brings in and who it skips, and why.
func (s *Service) PreviewImport(ctx context.Context, orgID uuid.UUID, req *models.CRMImportRequest) (*models.CRMImportPreview, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	_, _, preview, xerr := s.collect(ctx, o, req)
	return preview, xerr
}

// Import turns a filter into a contact import draft. The dashboard finishes
// it in the regular import review (campaign, labels, duplicates), so a
// Pipedrive import behaves exactly like any other.
func (s *Service) Import(ctx context.Context, orgID, userID uuid.UUID, req *models.CRMImportRequest) (*models.CRMImportResult, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	if s.d.Importer == nil {
		return nil, errx.InternalError()
	}
	name, rows, preview, xerr := s.collect(ctx, o, req)
	if xerr != nil {
		return nil, xerr
	}
	if len(rows) == 0 {
		return nil, errx.New(errx.BadRequest, "Nobody in this filter can be imported.")
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Email", "First Name", "Last Name", "Company", "Phone"})
	for _, r := range rows {
		_ = w.Write([]string{csvSafe(r.Email), csvSafe(r.FirstName), csvSafe(r.LastName), csvSafe(r.Company), csvSafe(r.Phone)})
	}
	w.Flush()
	filename := "Pipedrive - " + strings.NewReplacer("/", "-", "\\", "-").Replace(name) + ".csv"
	imp, xerr := s.d.Importer.Create(ctx, orgID, userID, &buf, filename)
	if xerr != nil {
		return nil, xerr
	}
	// Link the imported people to their Pipedrive persons as soon as they exist.
	_ = s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: models.CRMJobBackfill,
		DedupeKey: "link-import:" + imp.ID.String(), Subject: "Link " + name,
		Payload:       map[string]any{"link_filter": req.ListID, "import_id": imp.ID.String()},
		NextAttemptAt: time.Now().Add(2 * time.Minute),
	})
	return &models.CRMImportResult{ImportID: imp.ID, Preview: *preview}, nil
}

// csvSafe keeps a Pipedrive value from being read as a formula by a spreadsheet.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// BackfillPreview counts the Warmbly-only records a switch would copy.
func (s *Service) BackfillPreview(ctx context.Context, orgID uuid.UUID) (*models.CRMBackfillPreview, *errx.Error) {
	p, err := s.d.Repo.CountUnlinkedNative(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	return p, nil
}

// StartBackfill queues the one-time copy of Warmbly's own CRM data into Pipedrive.
func (s *Service) StartBackfill(ctx context.Context, orgID uuid.UUID, req *models.CRMBackfillRequest) *errx.Error {
	if _, xerr := s.mustResolve(ctx, orgID); xerr != nil {
		return xerr
	}
	if !req.Deals && !req.Tasks && !req.Notes {
		return errx.New(errx.BadRequest, "choose deals, tasks or notes to copy")
	}
	if err := s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: models.CRMJobBackfill,
		DedupeKey: "backfill", Subject: "Copy Warmbly CRM data to Pipedrive",
		Payload: map[string]any{"deals": req.Deals, "tasks": req.Tasks, "notes": req.Notes},
	}); err != nil {
		return errx.InternalError()
	}
	return nil
}

const backfillBatch = 40

// backfillKinds are copied in order: deals first, so activities can attach.
var backfillKinds = []struct{ flag, object string }{
	{"deals", models.CRMObjectDeal}, {"tasks", models.CRMObjectTask}, {"notes", models.CRMObjectNote},
}

// backfill copies one batch per run and continues the same job with a cursor
// until every chosen type is done, so it never runs twice at once and never
// revisits a record. A record Pipedrive refuses is skipped and counted; an
// outage retries the batch, whose copied records are already linked. A
// filter-link job links imported contacts instead.
func (s *Service) backfill(ctx context.Context, o *org, p map[string]any) error {
	if filterID := str(p, "link_filter"); filterID != "" {
		return s.linkFilter(ctx, o, filterID)
	}
	for _, k := range backfillKinds {
		if on, _ := p[k.flag].(bool); !on {
			continue
		}
		if done, _ := p["done_"+k.flag].(bool); done {
			continue
		}
		ids, err := s.d.Repo.UnlinkedNative(ctx, o.ID, k.object, uuidOf(p, "after_"+k.flag), backfillBatch)
		if err != nil {
			return err
		}
		skipped, _ := p["skipped"].(float64)
		for _, lid := range ids {
			if err := s.backfillOne(ctx, o, k.object, lid); err != nil {
				if _, retry := s.classify(ctx, o, err); retry {
					return err
				}
				skipped++
				log.Info().Err(err).Str("org_id", o.ID.String()).Str("object", k.object).Msg("pipedrive: backfill skipped a record")
			}
			p["after_"+k.flag] = lid.String()
		}
		p["skipped"] = skipped
		if len(ids) < backfillBatch {
			p["done_"+k.flag] = true
		}
		return &errContinue{payload: p}
	}
	s.notify(ctx, o.ID, "", "deal", "task", "note")
	return nil
}

func (s *Service) backfillOne(ctx context.Context, o *org, object string, localID uuid.UUID) error {
	switch object {
	case models.CRMObjectDeal:
		return s.backfillDeal(ctx, o, localID)
	case models.CRMObjectTask:
		return s.syncLocalTask(ctx, o, localID, false)
	default:
		return s.syncLocalNote(ctx, o, localID)
	}
}

// backfillDeal moves a Warmbly-only deal onto the first Pipedrive pipeline
// (matching its stage by name, else the first stage) and creates it there;
// won and lost carry over as the deal's status.
func (s *Service) backfillDeal(ctx context.Context, o *org, localID uuid.UUID) error {
	deal, err := s.d.CRM.GetDeal(ctx, o.ID, localID)
	if err != nil || deal == nil {
		return nil
	}
	pipes, err := s.d.CRM.ListPipelines(ctx, o.ID)
	if err != nil {
		return err
	}
	var oldStage string
	for _, pl := range pipes {
		for _, st := range pl.Stages {
			if st.ID == deal.StageID {
				oldStage = st.Name
			}
		}
	}
	plinks, _ := s.d.Repo.ListLinks(ctx, o.ID, provider, models.CRMObjectPipeline)
	if len(plinks) == 0 {
		return errx.NewWithIdentifier(errx.Conflict, "crm_stage_unknown", "Pipedrive has no pipeline to copy deals into.")
	}
	linked := map[uuid.UUID]bool{}
	for _, l := range plinks {
		linked[l.LocalID] = true
	}
	var target *models.Pipeline
	for i := range pipes {
		if linked[pipes[i].ID] && (target == nil || pipes[i].Position < target.Position) {
			target = &pipes[i]
		}
	}
	if target == nil || len(target.Stages) == 0 {
		return errx.NewWithIdentifier(errx.Conflict, "crm_stage_unknown", "The Pipedrive pipeline has no stages to copy deals into.")
	}
	if linked[deal.PipelineID] {
		return s.pushIfUnlinked(ctx, o, deal)
	}
	stageID := target.Stages[0].ID
	for _, st := range target.Stages {
		if strings.EqualFold(strings.TrimSpace(st.Name), strings.TrimSpace(oldStage)) {
			stageID = st.ID
			break
		}
	}
	if err := s.d.Repo.MoveDeal(ctx, o.ID, deal.ID, target.ID, stageID); err != nil {
		return err
	}
	deal.PipelineID, deal.StageID = target.ID, stageID
	return s.pushIfUnlinked(ctx, o, deal)
}

func (s *Service) pushIfUnlinked(ctx context.Context, o *org, deal *models.Deal) error {
	if l, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectDeal, deal.ID); err != nil || l != nil {
		return err
	}
	if xerr := s.PushDealCreate(ctx, o.ID, deal); xerr != nil {
		return xerr
	}
	return nil
}

// linkFilter links the Warmbly contacts a filter import created to their
// Pipedrive persons, so their sends log against the right person straight away.
func (s *Service) linkFilter(ctx context.Context, o *org, filterID string) error {
	persons, err := o.Client.Persons(ctx, url.Values{"filter_id": {filterID}}, maxListImport)
	if err != nil {
		return err
	}
	for start := 0; start < len(persons); start += 200 {
		end := min(start+200, len(persons))
		var page []Person
		for _, p := range persons[start:end] {
			if rec, _ := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id(p.ID)); rec == nil {
				page = append(page, p)
			}
		}
		if err := s.linkPersons(ctx, o, page, nil); err != nil {
			return err
		}
	}
	s.notify(ctx, o.ID, "", "contact")
	return nil
}
