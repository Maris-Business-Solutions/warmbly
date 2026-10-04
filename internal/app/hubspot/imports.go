package hubspot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// maxListImport bounds one list import; a bigger list imports in parts.
const maxListImport = 25000

// Lists returns the portal's contact lists, newest first.
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
		n, err := strconv.Atoi(strings.TrimPrefix(string(raw), "list:"))
		if derr != nil || err != nil || n < 0 || !strings.HasPrefix(string(raw), "list:") {
			return nil, errx.New(errx.BadRequest, "invalid cursor")
		}
		offset = n
	}
	lists, more, next, err := o.Client.SearchLists(ctx, strings.TrimSpace(query), offset, limit)
	if err != nil {
		return nil, s.listsError(ctx, o, err)
	}
	out := &models.CRMListsResult{Data: make([]models.CRMList, 0, len(lists))}
	for _, l := range lists {
		size, _ := strconv.ParseInt(l.Additional.Size, 10, 64)
		out.Data = append(out.Data, models.CRMList{
			ExternalID: l.ListID, Name: l.Name, Size: size,
			Dynamic: strings.EqualFold(l.ProcessingType, "DYNAMIC"), UpdatedAt: l.UpdatedAt,
		})
	}
	out.Pagination = models.Pagination{HasMore: more}
	if more {
		c := base64.RawURLEncoding.EncodeToString([]byte("list:" + strconv.Itoa(next)))
		out.Pagination.NextCursor = &c
	}
	return out, nil
}

func (s *Service) listsError(ctx context.Context, o *org, err error) *errx.Error {
	if ae, ok := AsAPIError(err); ok && ae.Status == 403 {
		return errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required",
			"Reconnect HubSpot and allow list access to import from HubSpot lists.")
	}
	return s.userError(ctx, o, err)
}

// importRow is one list member headed for Warmbly.
type importRow struct {
	ID, Email, FirstName, LastName, Company, Phone, JobTitle, Lifecycle, Owner string
	OptedOut                                                                   bool
}

// collect reads a list's members and sorts them into kept and skipped.
func (s *Service) collect(ctx context.Context, o *org, req *models.CRMImportRequest) (string, []importRow, *models.CRMImportPreview, *errx.Error) {
	list, err := o.Client.GetList(ctx, req.ListID)
	if err != nil {
		return "", nil, nil, s.listsError(ctx, o, err)
	}
	ids, truncated, err := o.Client.ListMemberIDs(ctx, req.ListID, maxListImport)
	if err != nil {
		return "", nil, nil, s.listsError(ctx, o, err)
	}
	props := []string{"email", "firstname", "lastname", "company", "phone", "jobtitle", "lifecyclestage", "hubspot_owner_id", "hs_email_optout"}
	objs, err := o.Client.ReadObjects(ctx, "contacts", ids, props)
	if err != nil {
		return "", nil, nil, s.userError(ctx, o, err)
	}
	preview := &models.CRMImportPreview{ListName: list.Name, Total: len(objs), Truncated: truncated, Skipped: []models.CRMImportSkip{}, Sample: []models.CRMImportPerson{}}
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
	// Open deals are read from the mirror: a contact with a mirrored open deal.
	openDeal := map[string]bool{}
	if req.ApplyGuards && g.SkipOpenDeals {
		assoc, aerr := o.Client.BatchAssociations(ctx, "contacts", "deals", ids)
		if aerr == nil {
			for contact, deals := range assoc {
				for _, d := range deals {
					if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectDeal, d); l != nil {
						if deal, _ := s.d.CRM.GetDeal(ctx, o.ID, l.LocalID); deal != nil && deal.Status == models.DealStatusOpen {
							openDeal[contact] = true
						}
					}
				}
			}
		}
	}
	skips := map[string]*models.CRMImportSkip{}
	skip := func(reason, label string) {
		if skips[reason] == nil {
			skips[reason] = &models.CRMImportSkip{Reason: reason, Label: label}
		}
		skips[reason].Count++
	}
	var rows []importRow
	seen := map[string]bool{}
	for _, ob := range objs {
		r := importRow{
			ID: ob.ID, Email: strings.ToLower(strings.TrimSpace(ob.Prop("email"))), FirstName: ob.Prop("firstname"),
			LastName: ob.Prop("lastname"), Company: ob.Prop("company"), Phone: ob.Prop("phone"), JobTitle: ob.Prop("jobtitle"),
			Lifecycle: ob.Prop("lifecyclestage"), Owner: ob.Prop("hubspot_owner_id"),
			OptedOut: strings.EqualFold(ob.Prop("hs_email_optout"), "true"),
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
			switch {
			case g.SkipOptedOut && r.OptedOut:
				skip("opted_out", "Opted out of email in HubSpot")
				continue
			case len(g.SkipLifecycleStages) > 0 && slices.Contains(g.SkipLifecycleStages, r.Lifecycle):
				skip("lifecycle", "Lifecycle stage is "+s.optionLabel(ctx, o, "lifecyclestage", r.Lifecycle))
				continue
			case g.SkipOpenDeals && openDeal[r.ID]:
				skip("open_deal", "Has an open deal")
				continue
			case g.SkipOtherOwners && r.Owner != "" && !mapped[r.Owner]:
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
	return list.Name, rows, preview, nil
}

// PreviewImport counts who a list import brings in and who it skips, and why.
func (s *Service) PreviewImport(ctx context.Context, orgID uuid.UUID, req *models.CRMImportRequest) (*models.CRMImportPreview, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	_, _, preview, xerr := s.collect(ctx, o, req)
	return preview, xerr
}

// Import turns a list into a contact import draft. The dashboard finishes it
// in the regular import review (campaign, labels, duplicates), so a HubSpot
// import behaves exactly like any other.
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
		return nil, errx.New(errx.BadRequest, "Nobody in this list can be imported.")
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Email", "First Name", "Last Name", "Company", "Phone", "Job Title"})
	for _, r := range rows {
		_ = w.Write([]string{csvSafe(r.Email), csvSafe(r.FirstName), csvSafe(r.LastName), csvSafe(r.Company), csvSafe(r.Phone), csvSafe(r.JobTitle)})
	}
	w.Flush()
	filename := "HubSpot - " + strings.NewReplacer("/", "-", "\\", "-").Replace(name) + ".csv"
	imp, xerr := s.d.Importer.Create(ctx, orgID, userID, &buf, filename)
	if xerr != nil {
		return nil, xerr
	}
	// Link the imported people to their HubSpot records as soon as they exist.
	_ = s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: models.CRMJobBackfill,
		DedupeKey: "link-import:" + imp.ID.String(), Subject: "Link " + name,
		Payload:       map[string]any{"link_list": req.ListID, "import_id": imp.ID.String()},
		NextAttemptAt: time.Now().Add(2 * time.Minute),
	})
	return &models.CRMImportResult{ImportID: imp.ID, Preview: *preview}, nil
}

// csvSafe keeps a HubSpot value from being read as a formula by a spreadsheet.
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

// StartBackfill queues the one-time copy of Warmbly's own CRM data into HubSpot.
func (s *Service) StartBackfill(ctx context.Context, orgID uuid.UUID, req *models.CRMBackfillRequest) *errx.Error {
	if _, xerr := s.mustResolve(ctx, orgID); xerr != nil {
		return xerr
	}
	if !req.Deals && !req.Tasks && !req.Notes {
		return errx.New(errx.BadRequest, "choose deals, tasks or notes to copy")
	}
	if err := s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: models.CRMJobBackfill,
		DedupeKey: "backfill", Subject: "Copy Warmbly CRM data to HubSpot",
		Payload: map[string]any{"deals": req.Deals, "tasks": req.Tasks, "notes": req.Notes},
	}); err != nil {
		return errx.InternalError()
	}
	return nil
}

const backfillBatch = 40

// backfill copies a batch of Warmbly-only records and requeues itself until
// none are left. A list-link job links imported contacts instead.
func (s *Service) backfill(ctx context.Context, o *org, p map[string]any) error {
	if listID := str(p, "link_list"); listID != "" {
		return s.linkList(ctx, o, listID)
	}
	more := false
	if b, _ := p["deals"].(bool); b {
		ids, err := s.d.Repo.UnlinkedNative(ctx, o.ID, models.CRMObjectDeal, backfillBatch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.backfillDeal(ctx, o, id); err != nil {
				return err
			}
		}
		more = more || len(ids) == backfillBatch
	}
	if b, _ := p["tasks"].(bool); b {
		ids, err := s.d.Repo.UnlinkedNative(ctx, o.ID, models.CRMObjectTask, backfillBatch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.syncLocalTask(ctx, o, id); err != nil {
				return err
			}
		}
		more = more || len(ids) == backfillBatch
	}
	if b, _ := p["notes"].(bool); b {
		ids, err := s.d.Repo.UnlinkedNative(ctx, o.ID, models.CRMObjectNote, backfillBatch)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.syncLocalNote(ctx, o, id); err != nil {
				return err
			}
		}
		more = more || len(ids) == backfillBatch
	}
	if more {
		// The running job still holds its own key, so the next batch takes the
		// other one of a pair; the chain never collides with itself.
		next := "backfill-a"
		if str(p, "_key") == next {
			next = "backfill-b"
		}
		np := map[string]any{"deals": p["deals"], "tasks": p["tasks"], "notes": p["notes"], "_key": next}
		return s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
			OrganizationID: o.ID, Provider: provider, Kind: models.CRMJobBackfill,
			DedupeKey: next, Subject: "Copy Warmbly CRM data to HubSpot", Payload: np,
		})
	}
	s.notify(ctx, o.ID, "", "deal", "task", "note")
	return nil
}

// backfillDeal moves a Warmbly-only deal onto the HubSpot pipeline (matching
// its stage by name, else by status) and creates it there.
func (s *Service) backfillDeal(ctx context.Context, o *org, id uuid.UUID) error {
	deal, err := s.d.CRM.GetDeal(ctx, o.ID, id)
	if err != nil || deal == nil {
		return nil
	}
	pipes, err := s.d.CRM.ListPipelines(ctx, o.ID)
	if err != nil {
		return err
	}
	var oldStage string
	for _, p := range pipes {
		for _, st := range p.Stages {
			if st.ID == deal.StageID {
				oldStage = st.Name
			}
		}
	}
	plinks, _ := s.d.Repo.ListLinks(ctx, o.ID, provider, models.CRMObjectPipeline)
	if len(plinks) == 0 {
		return errx.NewWithIdentifier(errx.Conflict, "crm_stage_unknown", "HubSpot has no deal pipeline to copy deals into.")
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
	if target == nil {
		return nil
	}
	stageID := uuid.Nil
	for _, st := range target.Stages {
		if strings.EqualFold(strings.TrimSpace(st.Name), strings.TrimSpace(oldStage)) {
			stageID = st.ID
			break
		}
	}
	if stageID == uuid.Nil {
		sid, _, ok := s.stageForStatus(ctx, o, target.ID, deal.Status)
		if !ok {
			return nil
		}
		stageID = sid
	}
	if !linked[deal.PipelineID] || deal.StageID != stageID {
		if err := s.d.Repo.MoveDeal(ctx, o.ID, deal.ID, target.ID, stageID); err != nil {
			return err
		}
		deal.PipelineID, deal.StageID = target.ID, stageID
	}
	if xerr := s.PushDealCreate(ctx, o.ID, deal); xerr != nil {
		return xerr
	}
	return nil
}

// linkList links the Warmbly contacts a list import created to their HubSpot
// records, so their sends log against the right person straight away.
func (s *Service) linkList(ctx context.Context, o *org, listID string) error {
	ids, _, err := o.Client.ListMemberIDs(ctx, listID, maxListImport)
	if err != nil {
		return err
	}
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		if _, err := s.localContacts(ctx, o, ids[start:end]); err != nil {
			return err
		}
	}
	s.notify(ctx, o.ID, "", "contact")
	return nil
}
