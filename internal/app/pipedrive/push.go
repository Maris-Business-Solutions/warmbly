package pipedrive

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// Write-through: a dashboard change goes to Pipedrive first, so a refusal (a
// missing permission, a deleted record) reaches the person who made it.
// Records written by automations go through the outbox instead (EnqueuePush).

// syncBulkLimit is how many records a bulk change writes while the request
// waits; Pipedrive has no batch endpoint, so larger ones go through the outbox.
const syncBulkLimit = 25

// PipelinesManaged refuses pipeline edits in Pipedrive mode.
func (s *Service) PipelinesManaged() *errx.Error {
	return errx.NewWithIdentifier(errx.Conflict, "crm_managed_externally",
		"Pipelines are managed in Pipedrive. Edit them there and they update here within minutes.")
}

// TaskTypesManaged refuses task type edits in Pipedrive mode.
func (s *Service) TaskTypesManaged() *errx.Error {
	return errx.NewWithIdentifier(errx.Conflict, "crm_managed_externally",
		"Task types are Pipedrive's activity types while Pipedrive is your CRM. Edit them in Pipedrive's company settings.")
}

// stageInfo resolves a local stage to its Pipedrive stage and pipeline ids.
func (s *Service) stageInfo(ctx context.Context, o *org, stageID uuid.UUID) (int64, int64, *errx.Error) {
	l, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectStage, stageID)
	if err != nil {
		return 0, 0, errx.InternalError()
	}
	if l == nil {
		return 0, 0, errx.NewWithIdentifier(errx.BadRequest, "crm_stage_unknown",
			"That stage is not one of your Pipedrive stages. Refresh the page and pick a Pipedrive stage.")
	}
	pipe, _ := l.Meta["pipeline"].(string)
	return extID(l.ExternalID), extID(pipe), nil
}

func (s *Service) ownerFor(ctx context.Context, o *org, userID *uuid.UUID) string {
	if userID == nil {
		return ""
	}
	ext, _ := s.d.Repo.OwnerForUser(ctx, o.ID, provider, *userID)
	return ext
}

func dealBody(name string, value *float64, currency string, closeDate *time.Time) map[string]any {
	body := map[string]any{"title": name}
	if value != nil {
		body["value"] = *value
	}
	if c := strings.ToUpper(strings.TrimSpace(currency)); len(c) == 3 {
		body["currency"] = c
	}
	if closeDate != nil {
		body["expected_close_date"] = pdDate(*closeDate)
	}
	return body
}

// PushDealCreate creates a deal that was just written locally and links it.
func (s *Service) PushDealCreate(ctx context.Context, orgID uuid.UUID, deal *models.Deal) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	stageExt, pipeExt, xerr := s.stageInfo(ctx, o, deal.StageID)
	if xerr != nil {
		return xerr
	}
	body := dealBody(deal.Name, deal.Value, deal.Currency, deal.ExpectedCloseDate)
	body["stage_id"] = stageExt
	if pipeExt != 0 {
		body["pipeline_id"] = pipeExt
	}
	owner := s.ownerFor(ctx, o, deal.AssignedTo)
	if deal.ContactID != nil {
		ext, rec, err := s.ensureContact(ctx, o, *deal.ContactID)
		if err != nil {
			return s.userError(ctx, o, err)
		}
		if ext != "" {
			body["person_id"] = extID(ext)
		}
		if rec != nil {
			if rec.CompanyExternalID != "" {
				body["org_id"] = extID(rec.CompanyExternalID)
			}
			if owner == "" {
				owner = rec.OwnerExternalID
			}
		}
	}
	if owner != "" {
		body["owner_id"] = extID(owner)
	}
	switch deal.Status {
	case models.DealStatusWon:
		body["status"] = "won"
	case models.DealStatusLost:
		body["status"] = "lost"
		if deal.LostReason != nil && *deal.LostReason != "" {
			body["lost_reason"] = *deal.LostReason
		}
	}
	created, err := o.Client.CreateDeal(ctx, body)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectDeal, LocalID: deal.ID, ExternalID: id(created.ID), Meta: ownerMeta(owner)}); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, orgID, contactIDString(deal.ContactID), "deal")
	return nil
}

// PushDealUpdate writes a deal edit to Pipedrive before it lands locally. Won
// and lost are a deal's status in Pipedrive, as in Warmbly, so the edit
// passes through unchanged.
func (s *Service) PushDealUpdate(ctx context.Context, orgID uuid.UUID, before *models.Deal, data *models.UpdateDeal) (*models.UpdateDeal, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectDeal, before.ID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if link == nil {
		merged := *before
		applyDealUpdate(&merged, data)
		if xerr := s.PushDealCreate(ctx, orgID, &merged); xerr != nil {
			return nil, xerr
		}
		return data, nil
	}
	body := map[string]any{}
	if data.Name != nil {
		body["title"] = *data.Name
	}
	if data.Value != nil {
		body["value"] = *data.Value
	}
	if data.Currency != nil && len(strings.TrimSpace(*data.Currency)) == 3 {
		body["currency"] = strings.ToUpper(strings.TrimSpace(*data.Currency))
	}
	if data.ExpectedCloseDate != nil {
		body["expected_close_date"] = pdDate(*data.ExpectedCloseDate)
	}
	if data.AssignedTo != nil {
		owner := s.ownerFor(ctx, o, data.AssignedTo)
		if owner == "" {
			return nil, errx.NewWithIdentifier(errx.BadRequest, "crm_owner_unmapped",
				"That member is not a Pipedrive user yet. Match them to a Pipedrive user in Integrations > Pipedrive.")
		}
		body["owner_id"] = extID(owner)
	}
	if data.StageID != nil {
		stageExt, pipeExt, xerr := s.stageInfo(ctx, o, *data.StageID)
		if xerr != nil {
			return nil, xerr
		}
		body["stage_id"] = stageExt
		if pipeExt != 0 {
			body["pipeline_id"] = pipeExt
		}
	}
	if data.Status != nil && *data.Status != string(before.Status) {
		body["status"] = *data.Status
		if *data.Status == string(models.DealStatusLost) && data.LostReason != nil && *data.LostReason != "" {
			body["lost_reason"] = *data.LostReason
		}
	} else if data.LostReason != nil && before.Status == models.DealStatusLost {
		body["lost_reason"] = *data.LostReason
	}
	if data.ContactID != nil && (before.ContactID == nil || *before.ContactID != *data.ContactID) {
		if ext, _, err := s.ensureContact(ctx, o, *data.ContactID); err == nil && ext != "" {
			body["person_id"] = extID(ext)
		}
	}
	if len(body) > 0 {
		if _, err := o.Client.UpdateDeal(ctx, extID(link.ExternalID), body); err != nil {
			return nil, s.userError(ctx, o, err)
		}
	}
	s.notify(ctx, orgID, contactIDString(before.ContactID), "deal")
	return data, nil
}

func applyDealUpdate(d *models.Deal, u *models.UpdateDeal) {
	if u.StageID != nil {
		d.StageID = *u.StageID
	}
	if u.ContactID != nil {
		d.ContactID = u.ContactID
	}
	if u.Name != nil {
		d.Name = *u.Name
	}
	if u.Value != nil {
		d.Value = u.Value
	}
	if u.Currency != nil {
		d.Currency = *u.Currency
	}
	if u.ExpectedCloseDate != nil {
		d.ExpectedCloseDate = u.ExpectedCloseDate
	}
	if u.AssignedTo != nil {
		d.AssignedTo = u.AssignedTo
	}
	if u.Status != nil {
		d.Status = models.DealStatus(*u.Status)
	}
	if u.LostReason != nil {
		d.LostReason = u.LostReason
	}
}

// PushDealDelete deletes the deal in Pipedrive.
func (s *Service) PushDealDelete(ctx context.Context, orgID, dealID uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectDeal, []uuid.UUID{dealID})
}

func (s *Service) deleteExternal(ctx context.Context, o *org, objectType string, ext int64) error {
	var err error
	switch objectType {
	case models.CRMObjectDeal:
		err = o.Client.DeleteDeal(ctx, ext)
	case models.CRMObjectTask:
		err = o.Client.DeleteActivity(ctx, ext)
	case models.CRMObjectNote:
		err = o.Client.DeleteNote(ctx, ext)
	}
	if ae, ok := AsAPIError(err); ok && ae.NotFound() {
		return nil
	}
	return err
}

func (s *Service) pushDelete(ctx context.Context, orgID uuid.UUID, objectType string, ids []uuid.UUID) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, objectType, ids)
	if err != nil {
		return errx.InternalError()
	}
	if len(links) == 0 {
		return nil
	}
	if len(links) > syncBulkLimit {
		var ext []string
		for _, l := range links {
			ext = append(ext, l.ExternalID)
		}
		for start := 0; start < len(ext); start += 100 {
			end := min(start+100, len(ext))
			if err := s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
				OrganizationID: orgID, Provider: provider, Kind: models.CRMJobPushTask,
				Subject: "Delete " + objectType + "s", Payload: map[string]any{"delete": objectType, "external_ids": ext[start:end]},
			}); err != nil {
				return errx.InternalError()
			}
		}
	} else {
		for _, l := range links {
			if err := s.deleteExternal(ctx, o, objectType, extID(l.ExternalID)); err != nil {
				return s.userError(ctx, o, err)
			}
		}
	}
	for localID := range links {
		_ = s.d.Repo.DeleteLinkByLocal(ctx, orgID, provider, objectType, localID)
	}
	s.notify(ctx, orgID, "", objectType)
	return nil
}

// activityBody is a Warmbly task as a Pipedrive activity.
func (s *Service) activityBody(ctx context.Context, o *org, t *models.CRMTask) map[string]any {
	body := map[string]any{
		"subject": t.Title,
		"type":    s.activityKey(ctx, o, t.Type),
		"done":    t.Status == models.CRMTaskStatusCompleted || t.Status == models.CRMTaskStatusCancelled,
	}
	if t.Description != nil {
		body["note"] = textToHTML(*t.Description)
	}
	setDue(body, t.DueDate)
	return body
}

// setDue writes a due date, with a time only when the task has one.
func setDue(body map[string]any, due *time.Time) {
	if due == nil {
		return
	}
	body["due_date"] = pdDate(*due)
	if u := due.UTC(); u.Hour() != 0 || u.Minute() != 0 {
		body["due_time"] = pdClock(u)
	}
}

// PushTaskCreate creates an activity for a task just written locally.
func (s *Service) PushTaskCreate(ctx context.Context, orgID uuid.UUID, task *models.CRMTask) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	body := s.activityBody(ctx, o, task)
	owner := s.ownerFor(ctx, o, task.AssignedTo)
	if task.ContactID != nil {
		ext, rec, err := s.ensureContact(ctx, o, *task.ContactID)
		if err != nil {
			return s.userError(ctx, o, err)
		}
		if ext != "" {
			body["participants"] = []Participant{{PersonID: extID(ext), Primary: true}}
		}
		if rec != nil {
			if rec.CompanyExternalID != "" {
				body["org_id"] = extID(rec.CompanyExternalID)
			}
			if owner == "" {
				owner = rec.OwnerExternalID
			}
		}
	}
	if task.DealID != nil {
		if l, _ := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectDeal, *task.DealID); l != nil {
			body["deal_id"] = extID(l.ExternalID)
		}
	}
	if owner != "" {
		body["owner_id"] = extID(owner)
	}
	created, err := o.Client.CreateActivity(ctx, body)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectTask, LocalID: task.ID, ExternalID: id(created.ID), Meta: ownerMeta(owner)}); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, orgID, contactIDString(task.ContactID), "task")
	return nil
}

// PushTaskUpdate writes a task edit to Pipedrive before it lands locally.
// Priority stays Warmbly's: Pipedrive activities carry none by default.
func (s *Service) PushTaskUpdate(ctx context.Context, orgID uuid.UUID, before *models.CRMTask, data *models.UpdateCRMTask) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectTask, before.ID)
	if err != nil {
		return errx.InternalError()
	}
	if link == nil {
		merged := *before
		applyTaskUpdate(&merged, data)
		return s.PushTaskCreate(ctx, orgID, &merged)
	}
	body := map[string]any{}
	if data.Title != nil {
		body["subject"] = *data.Title
	}
	if data.Description != nil {
		body["note"] = textToHTML(*data.Description)
	}
	if data.DueDate != nil {
		setDue(body, data.DueDate)
	}
	if data.Type != nil {
		body["type"] = s.activityKey(ctx, o, *data.Type)
	}
	if data.Status != nil {
		st := models.CRMTaskStatus(*data.Status)
		body["done"] = st == models.CRMTaskStatusCompleted || st == models.CRMTaskStatusCancelled
	}
	if data.AssignedTo != nil {
		owner := s.ownerFor(ctx, o, data.AssignedTo)
		if owner == "" {
			return errx.NewWithIdentifier(errx.BadRequest, "crm_owner_unmapped",
				"That member is not a Pipedrive user yet. Match them to a Pipedrive user in Integrations > Pipedrive.")
		}
		body["owner_id"] = extID(owner)
	}
	if len(body) == 0 {
		return nil
	}
	if _, err := o.Client.UpdateActivity(ctx, extID(link.ExternalID), body); err != nil {
		return s.userError(ctx, o, err)
	}
	s.notify(ctx, orgID, contactIDString(before.ContactID), "task")
	return nil
}

func applyTaskUpdate(t *models.CRMTask, u *models.UpdateCRMTask) {
	if u.Title != nil {
		t.Title = *u.Title
	}
	if u.Description != nil {
		t.Description = u.Description
	}
	if u.DueDate != nil {
		t.DueDate = u.DueDate
	}
	if u.Priority != nil {
		t.Priority = models.CRMTaskPriority(*u.Priority)
	}
	if u.Type != nil {
		t.Type = *u.Type
	}
	if u.Status != nil {
		t.Status = models.CRMTaskStatus(*u.Status)
	}
	if u.AssignedTo != nil {
		t.AssignedTo = u.AssignedTo
	}
}

// PushTasksBulk marks many activities done or not done. A few are written
// now; more are queued, each written from its saved state a moment later.
func (s *Service) PushTasksBulk(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, status, _ *string) *errx.Error {
	if status == nil {
		return nil
	}
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectTask, ids)
	if err != nil {
		return errx.InternalError()
	}
	st := models.CRMTaskStatus(*status)
	done := st == models.CRMTaskStatusCompleted || st == models.CRMTaskStatusCancelled
	if len(links) <= syncBulkLimit {
		for _, l := range links {
			if _, err := o.Client.UpdateActivity(ctx, extID(l.ExternalID), map[string]any{"done": done}); err != nil {
				return s.userError(ctx, o, err)
			}
		}
		s.notify(ctx, orgID, "", "task")
		return nil
	}
	for localID := range links {
		if err := s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
			OrganizationID: orgID, Provider: provider, Kind: models.CRMJobPushTask,
			DedupeKey: "task-state:" + localID.String(), Subject: "task " + localID.String()[:8],
			Payload:       map[string]any{"local_id": localID.String(), "update": true},
			NextAttemptAt: time.Now().Add(5 * time.Second),
		}); err != nil {
			return errx.InternalError()
		}
	}
	return nil
}

// PushTasksDelete deletes activities in Pipedrive.
func (s *Service) PushTasksDelete(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectTask, ids)
}

// PushNoteCreate creates a note on the person that was just written locally.
func (s *Service) PushNoteCreate(ctx context.Context, orgID uuid.UUID, note *models.ContactNote) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	ext, rec, err := s.ensureContact(ctx, o, note.ContactID)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if ext == "" {
		return errx.NewWithIdentifier(errx.NotFound, "crm_contact_missing",
			"This contact is not in Pipedrive, and creating people is turned off in the Pipedrive settings.")
	}
	body := map[string]any{"content": textToHTML(note.Content), "person_id": extID(ext)}
	if rec != nil && rec.CompanyExternalID != "" {
		body["org_id"] = extID(rec.CompanyExternalID)
	}
	created, err := o.Client.CreateNote(ctx, body)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectNote, LocalID: note.ID, ExternalID: id(created.ID)}); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, orgID, note.ContactID.String(), "note")
	return nil
}

// PushNoteUpdate writes an edited note to Pipedrive.
func (s *Service) PushNoteUpdate(ctx context.Context, orgID, noteID uuid.UUID, content string) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectNote, noteID)
	if err != nil {
		return errx.InternalError()
	}
	if link == nil {
		return nil
	}
	if err := o.Client.UpdateNote(ctx, extID(link.ExternalID), textToHTML(content)); err != nil {
		return s.userError(ctx, o, err)
	}
	return nil
}

// PushNoteDelete deletes a note in Pipedrive.
func (s *Service) PushNoteDelete(ctx context.Context, orgID, noteID uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectNote, []uuid.UUID{noteID})
}

// EnqueuePush queues a record written outside the dashboard (automations,
// reply tasks) for Pipedrive. A no-op outside Pipedrive mode.
func (s *Service) EnqueuePush(ctx context.Context, orgID uuid.UUID, objectType string, localID uuid.UUID) {
	if !s.Active(ctx, orgID) {
		return
	}
	kind := map[string]string{
		models.CRMObjectDeal: models.CRMJobPushDeal,
		models.CRMObjectTask: models.CRMJobPushTask,
		models.CRMObjectNote: models.CRMJobPushNote,
	}[objectType]
	if kind == "" {
		return
	}
	if err := s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
		OrganizationID: orgID, Provider: provider, Kind: kind,
		DedupeKey: kind + ":" + localID.String(),
		Subject:   objectType + " " + localID.String()[:8],
		Payload:   map[string]any{"local_id": localID.String()},
	}); err != nil {
		log.Warn().Err(err).Str("org_id", orgID.String()).Msg("pipedrive: could not queue a record")
	}
}

// ---------- read decoration ----------

// DecorateDeals attaches Pipedrive links and owner names to deals.
func (s *Service) DecorateDeals(ctx context.Context, orgID uuid.UUID, deals []models.Deal) {
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil || len(deals) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(deals))
	for i := range deals {
		ids[i] = deals[i].ID
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectDeal, ids)
	if err != nil {
		return
	}
	names := s.ownerNames(ctx, orgID)
	for i := range deals {
		l, ok := links[deals[i].ID]
		if !ok {
			continue
		}
		ref := &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: o.dealURL(l.ExternalID), SyncedAt: l.SyncedAt}
		if owner, _ := l.Meta["owner"].(string); owner != "" && deals[i].AssignedTo == nil {
			ref.OwnerName = names[owner]
		}
		deals[i].External = ref
	}
}

// DecorateTasks attaches Pipedrive links and owner names to tasks.
func (s *Service) DecorateTasks(ctx context.Context, orgID uuid.UUID, tasks []models.CRMTask) {
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil || len(tasks) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(tasks))
	var dealIDs []uuid.UUID
	for i := range tasks {
		ids[i] = tasks[i].ID
		if tasks[i].DealID != nil {
			dealIDs = append(dealIDs, *tasks[i].DealID)
		}
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectTask, ids)
	if err != nil {
		return
	}
	dealLinks, _ := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectDeal, dealIDs)
	names := s.ownerNames(ctx, orgID)
	for i := range tasks {
		l, ok := links[tasks[i].ID]
		if !ok {
			continue
		}
		url := o.activitiesURL()
		if tasks[i].DealID != nil {
			if dl, ok := dealLinks[*tasks[i].DealID]; ok {
				url = o.dealURL(dl.ExternalID)
			}
		}
		ref := &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: url, SyncedAt: l.SyncedAt}
		if owner, _ := l.Meta["owner"].(string); owner != "" && tasks[i].AssignedTo == nil {
			ref.OwnerName = names[owner]
		}
		tasks[i].External = ref
	}
}

// DecorateNotes attaches Pipedrive links to notes.
func (s *Service) DecorateNotes(ctx context.Context, orgID uuid.UUID, notes []models.ContactNote) {
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil || len(notes) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(notes))
	for i := range notes {
		ids[i] = notes[i].ID
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectNote, ids)
	if err != nil {
		return
	}
	var personURL string
	if rec, _ := s.d.Repo.GetContactRecord(ctx, orgID, notes[0].ContactID, provider); rec != nil {
		personURL = o.personURL(rec.ExternalID)
	}
	for i := range notes {
		if l, ok := links[notes[i].ID]; ok {
			notes[i].External = &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: personURL, SyncedAt: l.SyncedAt}
		}
	}
}

// DecoratePipelines marks Pipedrive pipelines and their stages' probability.
func (s *Service) DecoratePipelines(ctx context.Context, orgID uuid.UUID, pipes []models.Pipeline) {
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil || len(pipes) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(pipes))
	var stageIDs []uuid.UUID
	for _, p := range pipes {
		ids = append(ids, p.ID)
		for _, st := range p.Stages {
			stageIDs = append(stageIDs, st.ID)
		}
	}
	plinks, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectPipeline, ids)
	if err != nil {
		return
	}
	slinks, _ := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectStage, stageIDs)
	for i := range pipes {
		if l, ok := plinks[pipes[i].ID]; ok {
			pipes[i].External = &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: o.pipelineURL(l.ExternalID), SyncedAt: l.SyncedAt}
		}
		for j := range pipes[i].Stages {
			if l, ok := slinks[pipes[i].Stages[j].ID]; ok {
				pipes[i].Stages[j].Probability = probabilityOf(l.Meta)
			}
		}
	}
}

func (s *Service) ownerNames(ctx context.Context, orgID uuid.UUID) map[string]string {
	out := map[string]string{}
	owners, err := s.d.Repo.ListOwners(ctx, orgID, provider)
	if err != nil {
		return out
	}
	for _, o := range owners {
		out[o.ExternalID] = o.DisplayName()
	}
	return out
}
