package hubspot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// Write-through: a dashboard change goes to HubSpot first, so a refusal (a
// required property, a missing permission) reaches the person who made it.
// Records written by automations go through the outbox instead (EnqueuePush).

// PipelinesManaged refuses pipeline edits in HubSpot mode.
func (s *Service) PipelinesManaged() *errx.Error {
	return errx.NewWithIdentifier(errx.Conflict, "crm_managed_externally",
		"Pipelines are managed in HubSpot. Edit them there and they update here within minutes.")
}

// TaskTypesManaged refuses task type edits in HubSpot mode.
func (s *Service) TaskTypesManaged() *errx.Error {
	return errx.NewWithIdentifier(errx.Conflict, "crm_managed_externally",
		"Task types come from HubSpot (To-do, Call, Email, LinkedIn) while HubSpot is your CRM.")
}

// stageInfo resolves a local stage to its HubSpot id and metadata.
func (s *Service) stageInfo(ctx context.Context, o *org, stageID uuid.UUID) (string, stageMeta, *errx.Error) {
	l, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectStage, stageID)
	if err != nil {
		return "", stageMeta{}, errx.InternalError()
	}
	if l == nil {
		return "", stageMeta{}, errx.NewWithIdentifier(errx.BadRequest, "crm_stage_unknown",
			"That stage is not one of your HubSpot stages. Refresh the page and pick a HubSpot stage.")
	}
	return l.ExternalID, stageMetaFromMap(l.Meta), nil
}

func (s *Service) pipelineExternal(ctx context.Context, o *org, pipelineID uuid.UUID) (string, *errx.Error) {
	l, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectPipeline, pipelineID)
	if err != nil {
		return "", errx.InternalError()
	}
	if l == nil {
		return "", errx.NewWithIdentifier(errx.BadRequest, "crm_stage_unknown",
			"That pipeline is not one of your HubSpot pipelines. Refresh the page and pick a HubSpot pipeline.")
	}
	return l.ExternalID, nil
}

func (s *Service) ownerFor(ctx context.Context, o *org, userID *uuid.UUID) string {
	if userID == nil {
		return ""
	}
	ext, _ := s.d.Repo.OwnerForUser(ctx, o.ID, provider, *userID)
	return ext
}

// stageForStatus finds the pipeline stage that means a status: the first
// closed-won or closed-lost stage, or the first open one.
func (s *Service) stageForStatus(ctx context.Context, o *org, pipelineID uuid.UUID, status models.DealStatus) (uuid.UUID, string, bool) {
	pipes, err := s.d.CRM.ListPipelines(ctx, o.ID)
	if err != nil {
		return uuid.Nil, "", false
	}
	var stageIDs []uuid.UUID
	for _, p := range pipes {
		if p.ID == pipelineID {
			for _, st := range p.Stages {
				stageIDs = append(stageIDs, st.ID)
			}
		}
	}
	links, err := s.d.Repo.LinksForLocal(ctx, o.ID, provider, models.CRMObjectStage, stageIDs)
	if err != nil {
		return uuid.Nil, "", false
	}
	for _, id := range stageIDs {
		l, ok := links[id]
		if !ok {
			continue
		}
		if stageMetaFromMap(l.Meta).status() == status {
			return id, l.ExternalID, true
		}
	}
	return uuid.Nil, "", false
}

func dealProperties(name string, value *float64, currency string, closeDate *time.Time) map[string]string {
	props := map[string]string{"dealname": name}
	if value != nil {
		props["amount"] = strconv.FormatFloat(*value, 'f', 2, 64)
	}
	if c := strings.ToUpper(strings.TrimSpace(currency)); len(c) == 3 {
		props["deal_currency_code"] = c
	}
	if closeDate != nil {
		props["closedate"] = hsTime(*closeDate)
	}
	return props
}

// PushDealCreate creates a deal that was just written locally and links it.
func (s *Service) PushDealCreate(ctx context.Context, orgID uuid.UUID, deal *models.Deal) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	pipeExt, xerr := s.pipelineExternal(ctx, o, deal.PipelineID)
	if xerr != nil {
		return xerr
	}
	stageExt, meta, xerr := s.stageInfo(ctx, o, deal.StageID)
	if xerr != nil {
		return xerr
	}
	props := dealProperties(deal.Name, deal.Value, deal.Currency, deal.ExpectedCloseDate)
	props["pipeline"] = pipeExt
	props["dealstage"] = stageExt
	if owner := s.ownerFor(ctx, o, deal.AssignedTo); owner != "" {
		props["hubspot_owner_id"] = owner
	}
	var assocs []Assoc
	var contactExt string
	var rec *models.CRMContactRecord
	if deal.ContactID != nil {
		var err error
		if contactExt, rec, err = s.ensureContact(ctx, o, *deal.ContactID); err != nil {
			return s.userError(ctx, o, err)
		}
		assocs = append(assocs, Assoc{ToID: contactExt, TypeID: assocDealToContact})
		if rec != nil && rec.CompanyExternalID != "" {
			assocs = append(assocs, Assoc{ToID: rec.CompanyExternalID, TypeID: assocDealToCompany})
		}
		if owner := props["hubspot_owner_id"]; owner == "" && rec != nil && rec.OwnerExternalID != "" {
			props["hubspot_owner_id"] = rec.OwnerExternalID
		}
	}
	obj, err := o.Client.Create(ctx, "deals", props, assocs)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectDeal, LocalID: deal.ID, ExternalID: obj.ID, Meta: ownerMeta(props["hubspot_owner_id"])}); err != nil {
		return errx.InternalError()
	}
	deal.Status = meta.status()
	s.notify(ctx, orgID, contactIDString(deal.ContactID), "deal")
	return nil
}

func ownerMeta(owner string) map[string]any {
	if owner == "" {
		return map[string]any{}
	}
	return map[string]any{"owner": owner}
}

func contactIDString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// PushDealUpdate writes a deal edit to HubSpot before it lands locally and
// returns the edit adjusted to HubSpot's model, where won and lost are stages.
func (s *Service) PushDealUpdate(ctx context.Context, orgID uuid.UUID, before *models.Deal, data *models.UpdateDeal) (*models.UpdateDeal, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectDeal, before.ID)
	if err != nil {
		return nil, errx.InternalError()
	}
	adjusted := *data
	props := map[string]string{}
	if data.Name != nil {
		props["dealname"] = *data.Name
	}
	if data.Value != nil {
		props["amount"] = strconv.FormatFloat(*data.Value, 'f', 2, 64)
	}
	if data.Currency != nil && len(strings.TrimSpace(*data.Currency)) == 3 {
		props["deal_currency_code"] = strings.ToUpper(strings.TrimSpace(*data.Currency))
	}
	if data.ExpectedCloseDate != nil {
		props["closedate"] = hsTime(*data.ExpectedCloseDate)
	}
	if data.LostReason != nil {
		props["closed_lost_reason"] = *data.LostReason
	}
	if data.AssignedTo != nil {
		owner := s.ownerFor(ctx, o, data.AssignedTo)
		if owner == "" {
			return nil, errx.NewWithIdentifier(errx.BadRequest, "crm_owner_unmapped",
				"That member is not a HubSpot user yet. Match them to a HubSpot owner in Integrations > HubSpot.")
		}
		props["hubspot_owner_id"] = owner
	}
	if data.StageID != nil {
		stageExt, meta, xerr := s.stageInfo(ctx, o, *data.StageID)
		if xerr != nil {
			return nil, xerr
		}
		props["dealstage"] = stageExt
		st := string(meta.status())
		adjusted.Status = &st
	} else if data.Status != nil && *data.Status != string(before.Status) {
		stageID, stageExt, ok := s.stageForStatus(ctx, o, before.PipelineID, models.DealStatus(*data.Status))
		if !ok {
			return nil, errx.NewWithIdentifier(errx.BadRequest, "crm_stage_unknown",
				fmt.Sprintf("This HubSpot pipeline has no %s stage to move the deal to.", *data.Status))
		}
		props["dealstage"] = stageExt
		adjusted.StageID = &stageID
	}
	if link == nil {
		// Not in HubSpot yet (created before the switch): create it now.
		merged := *before
		applyDealUpdate(&merged, &adjusted)
		if xerr := s.PushDealCreate(ctx, orgID, &merged); xerr != nil {
			return nil, xerr
		}
		return &adjusted, nil
	}
	if len(props) > 0 {
		if _, err := o.Client.Update(ctx, "deals", link.ExternalID, props); err != nil {
			return nil, s.userError(ctx, o, err)
		}
	}
	if data.ContactID != nil && (before.ContactID == nil || *before.ContactID != *data.ContactID) {
		if ext, _, err := s.ensureContact(ctx, o, *data.ContactID); err == nil && ext != "" {
			_ = o.Client.AssociateDefault(ctx, "deals", link.ExternalID, "contacts", ext)
		}
	}
	s.notify(ctx, orgID, contactIDString(before.ContactID), "deal")
	return &adjusted, nil
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
}

// PushDealDelete deletes the deal in HubSpot.
func (s *Service) PushDealDelete(ctx context.Context, orgID, dealID uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectDeal, "deals", []uuid.UUID{dealID})
}

func (s *Service) pushDelete(ctx context.Context, orgID uuid.UUID, objectType, hsType string, ids []uuid.UUID) *errx.Error {
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
	ext := make([]string, 0, len(links))
	for _, l := range links {
		ext = append(ext, l.ExternalID)
	}
	if len(ext) == 1 {
		err = o.Client.Archive(ctx, hsType, ext[0])
	} else {
		err = o.Client.BatchArchive(ctx, hsType, ext)
	}
	if err != nil {
		return s.userError(ctx, o, err)
	}
	for id := range links {
		_ = s.d.Repo.DeleteLinkByLocal(ctx, orgID, provider, objectType, id)
	}
	s.notify(ctx, orgID, "", objectType)
	return nil
}

func taskProperties(title string, desc *string, due *time.Time, priority models.CRMTaskPriority, typ string, status models.CRMTaskStatus) map[string]string {
	props := map[string]string{
		"hs_task_subject":  title,
		"hs_task_status":   hsTaskStatus(status),
		"hs_task_priority": hsTaskPriority(priority),
		"hs_task_type":     hsTaskType(typ),
	}
	if desc != nil {
		props["hs_task_body"] = textToHTML(*desc)
	}
	if due != nil {
		props["hs_timestamp"] = hsTime(*due)
	} else {
		props["hs_timestamp"] = hsTime(time.Now().Add(24 * time.Hour))
	}
	return props
}

// PushTaskCreate creates a task that was just written locally and links it.
func (s *Service) PushTaskCreate(ctx context.Context, orgID uuid.UUID, task *models.CRMTask) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	props := taskProperties(task.Title, task.Description, task.DueDate, task.Priority, task.Type, task.Status)
	owner := s.ownerFor(ctx, o, task.AssignedTo)
	var assocs []Assoc
	if task.ContactID != nil {
		ext, rec, err := s.ensureContact(ctx, o, *task.ContactID)
		if err != nil {
			return s.userError(ctx, o, err)
		}
		assocs = append(assocs, Assoc{ToID: ext, TypeID: assocTaskToContact})
		if owner == "" && rec != nil {
			owner = rec.OwnerExternalID
		}
	}
	if task.DealID != nil {
		if l, _ := s.d.Repo.GetLinkByLocal(ctx, orgID, provider, models.CRMObjectDeal, *task.DealID); l != nil {
			assocs = append(assocs, Assoc{ToID: l.ExternalID, TypeID: assocTaskToDeal})
		}
	}
	if owner != "" {
		props["hubspot_owner_id"] = owner
	}
	obj, err := o.Client.Create(ctx, "tasks", props, assocs)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectTask, LocalID: task.ID, ExternalID: obj.ID, Meta: ownerMeta(owner)}); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, orgID, contactIDString(task.ContactID), "task")
	return nil
}

// PushTaskUpdate writes a task edit to HubSpot before it lands locally.
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
	props := map[string]string{}
	if data.Title != nil {
		props["hs_task_subject"] = *data.Title
	}
	if data.Description != nil {
		props["hs_task_body"] = textToHTML(*data.Description)
	}
	if data.DueDate != nil {
		props["hs_timestamp"] = hsTime(*data.DueDate)
	}
	if data.Priority != nil {
		props["hs_task_priority"] = hsTaskPriority(models.CRMTaskPriority(*data.Priority))
	}
	if data.Type != nil {
		props["hs_task_type"] = hsTaskType(*data.Type)
	}
	if data.Status != nil {
		props["hs_task_status"] = hsTaskStatus(models.CRMTaskStatus(*data.Status))
	}
	if data.AssignedTo != nil {
		owner := s.ownerFor(ctx, o, data.AssignedTo)
		if owner == "" {
			return errx.NewWithIdentifier(errx.BadRequest, "crm_owner_unmapped",
				"That member is not a HubSpot user yet. Match them to a HubSpot owner in Integrations > HubSpot.")
		}
		props["hubspot_owner_id"] = owner
	}
	if len(props) == 0 {
		return nil
	}
	if _, err := o.Client.Update(ctx, "tasks", link.ExternalID, props); err != nil {
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

// PushTasksBulk applies one status or priority to many tasks in HubSpot.
func (s *Service) PushTasksBulk(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, status, priority *string) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectTask, ids)
	if err != nil {
		return errx.InternalError()
	}
	props := map[string]string{}
	if status != nil {
		props["hs_task_status"] = hsTaskStatus(models.CRMTaskStatus(*status))
	}
	if priority != nil {
		props["hs_task_priority"] = hsTaskPriority(models.CRMTaskPriority(*priority))
	}
	if len(props) == 0 || len(links) == 0 {
		return nil
	}
	updates := make([]ObjectUpdate, 0, len(links))
	for _, l := range links {
		updates = append(updates, ObjectUpdate{ID: l.ExternalID, Properties: props})
	}
	if err := o.Client.BatchUpdate(ctx, "tasks", updates); err != nil {
		return s.userError(ctx, o, err)
	}
	s.notify(ctx, orgID, "", "task")
	return nil
}

// PushTasksDelete deletes tasks in HubSpot.
func (s *Service) PushTasksDelete(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectTask, "tasks", ids)
}

// PushNoteCreate creates a note that was just written locally and links it.
func (s *Service) PushNoteCreate(ctx context.Context, orgID uuid.UUID, note *models.ContactNote) *errx.Error {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return xerr
	}
	ext, _, err := s.ensureContact(ctx, o, note.ContactID)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if ext == "" {
		return errx.NewWithIdentifier(errx.NotFound, "crm_contact_missing",
			"This contact is not in HubSpot, and creating contacts is turned off in the HubSpot settings.")
	}
	props := map[string]string{"hs_note_body": textToHTML(note.Content), "hs_timestamp": hsTime(time.Now())}
	if owner := s.ownerFor(ctx, o, &note.UserID); owner != "" {
		props["hubspot_owner_id"] = owner
	}
	assocs := []Assoc{{ToID: ext, TypeID: assocNoteToContact}}
	obj, err := o.Client.Create(ctx, "notes", props, assocs)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	if err := s.d.Repo.ClaimLink(ctx, &models.CRMExternalLink{OrganizationID: orgID, Provider: provider,
		ObjectType: models.CRMObjectNote, LocalID: note.ID, ExternalID: obj.ID}); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, orgID, note.ContactID.String(), "note")
	return nil
}

// PushNoteUpdate writes an edited note to HubSpot.
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
	if _, err := o.Client.Update(ctx, "notes", link.ExternalID, map[string]string{"hs_note_body": textToHTML(content)}); err != nil {
		return s.userError(ctx, o, err)
	}
	return nil
}

// PushNoteDelete deletes a note in HubSpot.
func (s *Service) PushNoteDelete(ctx context.Context, orgID, noteID uuid.UUID) *errx.Error {
	return s.pushDelete(ctx, orgID, models.CRMObjectNote, "notes", []uuid.UUID{noteID})
}

// EnqueuePush queues a record written outside the dashboard (automations,
// reply tasks) for HubSpot. A no-op outside HubSpot mode.
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
		log.Warn().Err(err).Str("org_id", orgID.String()).Msg("hubspot: could not queue a record")
	}
}

// ---------- read decoration ----------

// DecorateDeals attaches HubSpot links and owner names to deals.
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
		ref := &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: o.recordURL("0-3", l.ExternalID), SyncedAt: l.SyncedAt}
		if owner, _ := l.Meta["owner"].(string); owner != "" && deals[i].AssignedTo == nil {
			ref.OwnerName = names[owner]
		}
		deals[i].External = ref
	}
}

// DecorateTasks attaches HubSpot links and owner names to tasks.
func (s *Service) DecorateTasks(ctx context.Context, orgID uuid.UUID, tasks []models.CRMTask) {
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil || len(tasks) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
	}
	links, err := s.d.Repo.LinksForLocal(ctx, orgID, provider, models.CRMObjectTask, ids)
	if err != nil {
		return
	}
	names := s.ownerNames(ctx, orgID)
	for i := range tasks {
		l, ok := links[tasks[i].ID]
		if !ok {
			continue
		}
		ref := &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: o.tasksURL(), SyncedAt: l.SyncedAt}
		if owner, _ := l.Meta["owner"].(string); owner != "" && tasks[i].AssignedTo == nil {
			ref.OwnerName = names[owner]
		}
		tasks[i].External = ref
	}
}

// DecorateNotes attaches HubSpot links to notes.
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
	var contactURL string
	if len(notes) > 0 {
		if rec, _ := s.d.Repo.GetContactRecord(ctx, orgID, notes[0].ContactID, provider); rec != nil {
			contactURL = o.recordURL("0-1", rec.ExternalID)
		}
	}
	for i := range notes {
		if l, ok := links[notes[i].ID]; ok {
			notes[i].External = &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: contactURL, SyncedAt: l.SyncedAt}
		}
	}
}

// DecoratePipelines marks HubSpot pipelines and their stages' won/lost flags.
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
			pipes[i].External = &models.CRMExternalRef{Provider: provider, ExternalID: l.ExternalID, URL: o.pipelineSettingsURL(), SyncedAt: l.SyncedAt}
		}
		for j := range pipes[i].Stages {
			if l, ok := slinks[pipes[i].Stages[j].ID]; ok {
				m := stageMetaFromMap(l.Meta)
				pipes[i].Stages[j].Closed = m.Closed
				pipes[i].Stages[j].Won = m.Won
				pipes[i].Stages[j].Probability = m.Probability
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
