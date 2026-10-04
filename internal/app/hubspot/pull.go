package hubspot

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Pull keeps the mirror current: owners and pipelines on a slow beat, deals,
// tasks and contacts incrementally by last-modified time. Webhooks only make a
// record's next pull come sooner, so a missed delivery is never a missed change.

const (
	pullInterval      = time.Minute
	metaRefreshEvery  = 30 * time.Minute
	pagesPerPull      = 20
	contactPagesTick  = 10
	cursorDeals       = "deals"
	cursorTasks       = "tasks"
	cursorContacts    = "contacts"
	holdSourceCRM     = "crm"
	initialTaskWindow = 90 * 24 * time.Hour
)

// RunPuller pulls every HubSpot workspace once a minute until ctx ends. Each
// workspace is claimed in Redis, so several consumers share the work.
func (s *Service) RunPuller(ctx context.Context) {
	t := time.NewTicker(pullInterval)
	defer t.Stop()
	for {
		s.pullAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) pullAll(ctx context.Context) {
	rows, err := s.d.Repo.ListProviderOrgs(ctx, provider)
	if err != nil {
		log.Warn().Err(err).Msg("hubspot: list workspaces")
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		if !s.claim(ctx, "pull:"+row.OrganizationID.String(), 4*time.Minute) {
			continue
		}
		o, err := s.resolve(ctx, row.OrganizationID)
		if err != nil || o == nil {
			s.release(ctx, "pull:"+row.OrganizationID.String())
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Str("org_id", o.ID.String()).Msg("hubspot: pull panicked")
				}
			}()
			s.pullOrg(pctx, o, false)
		}()
		cancel()
		s.release(ctx, "pull:"+row.OrganizationID.String())
	}
}

func (s *Service) claim(ctx context.Context, key string, ttl time.Duration) bool {
	if s.d.Cache == nil {
		return true
	}
	ok, err := s.d.Cache.SetNX(ctx, "hubspot:lock:"+key, 1, ttl).Result()
	return err != nil || ok
}

func (s *Service) release(ctx context.Context, key string) {
	if s.d.Cache != nil {
		s.d.Cache.Del(ctx, "hubspot:lock:"+key)
	}
}

// due reports whether a slow refresh is due, and claims it.
func (s *Service) due(ctx context.Context, key string, every time.Duration, force bool) bool {
	if force || s.d.Cache == nil {
		return true
	}
	ok, err := s.d.Cache.SetNX(ctx, "hubspot:due:"+key, 1, every).Result()
	return err != nil || ok
}

func (s *Service) pullOrg(ctx context.Context, o *org, full bool) {
	key := o.ID.String()
	if s.due(ctx, "owners:"+key, metaRefreshEvery, full) {
		s.recordRun(ctx, o, "owners", nil, s.pullOwners(ctx, o))
	}
	if s.due(ctx, "pipelines:"+key, metaRefreshEvery, full) {
		s.recordRun(ctx, o, "pipelines", nil, s.pullPipelines(ctx, o))
	}
	if o.Config.WriteProperties && s.due(ctx, "props:"+key, 6*time.Hour, full) {
		_ = s.ensureProperties(ctx, o)
	}
	s.pullDeals(ctx, o)
	s.pullTasks(ctx, o)
	s.pullContacts(ctx, o)
}

func (s *Service) recordRun(ctx context.Context, o *org, objectType string, at *time.Time, err error) {
	msg := ""
	if err != nil {
		msg = s.userError(ctx, o, err).Message
	}
	_ = s.d.Repo.SetCursor(ctx, o.ID, provider, objectType, at, msg)
}

func (s *Service) pullOwners(ctx context.Context, o *org) error {
	owners, err := o.Client.Owners(ctx)
	if err != nil {
		return err
	}
	out := make([]models.CRMOwner, 0, len(owners))
	for _, ow := range owners {
		out = append(out, models.CRMOwner{ExternalID: ow.ID, Email: ow.Email, FirstName: ow.FirstName, LastName: ow.LastName, Archived: ow.Archived})
	}
	if err := s.d.Repo.ReplaceOwners(ctx, o.ID, provider, out); err != nil {
		return err
	}
	s.notify(ctx, o.ID, "", "owner")
	return nil
}

func (s *Service) pullPipelines(ctx context.Context, o *org) error {
	pipes, err := o.Client.DealPipelines(ctx)
	if err != nil {
		return err
	}
	var keep []string
	for _, p := range pipes {
		if p.Archived || (len(o.Config.DealPipelines) > 0 && !slices.Contains(o.Config.DealPipelines, p.ID)) {
			continue
		}
		localID, err := s.d.Repo.MirrorPipeline(ctx, o.ID, provider, p.ID, p.Label, p.DisplayOrder)
		if err != nil {
			return err
		}
		keep = append(keep, p.ID)
		var keepStages []string
		stages := append([]Stage(nil), p.Stages...)
		slices.SortStableFunc(stages, func(a, b Stage) int { return a.DisplayOrder - b.DisplayOrder })
		for i, st := range stages {
			if st.Archived {
				continue
			}
			meta := stageMetaFrom(st)
			if _, err := s.d.Repo.MirrorStage(ctx, o.ID, localID, provider, st.ID, st.Label, stageColor(i, meta), i, meta.toMap()); err != nil {
				return err
			}
			keepStages = append(keepStages, st.ID)
		}
		if err := s.d.Repo.PruneStages(ctx, o.ID, localID, provider, keepStages); err != nil {
			return err
		}
	}
	if err := s.d.Repo.PrunePipelines(ctx, o.ID, provider, keep); err != nil {
		return err
	}
	s.notify(ctx, o.ID, "", "pipeline")
	return nil
}

// modifiedFilter is "changed at or after the cursor", in epoch milliseconds.
func modifiedFilter(prop string, cursor *time.Time) []Filter {
	if cursor == nil {
		return nil
	}
	return []Filter{{PropertyName: prop, Operator: "GTE", Value: strconv.FormatInt(cursor.UnixMilli(), 10)}}
}

// advance moves a cursor past a page. A full page that did not move it (more
// than a page of records modified in the same millisecond) steps one ms on,
// so the pull cannot spin on the same page.
func advance(cursor *time.Time, last *time.Time, fullPage bool) *time.Time {
	if last == nil {
		return cursor
	}
	if cursor != nil && !last.After(*cursor) && fullPage {
		next := cursor.Add(time.Millisecond)
		return &next
	}
	return last
}

func (s *Service) pullDeals(ctx context.Context, o *org) {
	cursor, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorDeals)
	if err != nil {
		return
	}
	first := cursor == nil
	for page := 0; page < pagesPerPull; page++ {
		filters := modifiedFilter("hs_lastmodifieddate", cursor)
		if len(o.Config.DealPipelines) > 0 {
			filters = append(filters, Filter{PropertyName: "pipeline", Operator: "IN", Values: o.Config.DealPipelines})
		}
		var groups [][]Filter
		if len(filters) > 0 {
			groups = [][]Filter{filters}
		}
		res, err := o.Client.Search(ctx, "deals", groups, "hs_lastmodifieddate", dealProps, "", 100)
		if err != nil {
			s.recordRun(ctx, o, cursorDeals, nil, err)
			return
		}
		if len(res.Results) == 0 {
			break
		}
		last, err := s.mirrorDeals(ctx, o, res.Results, !first)
		if err != nil {
			s.recordRun(ctx, o, cursorDeals, nil, err)
			return
		}
		cursor = advance(cursor, last, len(res.Results) == 100)
		_ = s.d.Repo.SetCursor(ctx, o.ID, provider, cursorDeals, cursor, "")
		s.notify(ctx, o.ID, "", "deal")
		if len(res.Results) < 100 {
			break
		}
	}
	s.recordRun(ctx, o, cursorDeals, cursor, nil)
}

// mirrorDeals writes a page of deals and returns the latest modified time.
// applyExit is false on the very first pull, so deals that existed before the
// workspace switched to HubSpot do not stop anyone's campaigns.
func (s *Service) mirrorDeals(ctx context.Context, o *org, deals []Object, applyExit bool) (*time.Time, error) {
	ids := make([]string, 0, len(deals))
	for _, d := range deals {
		ids = append(ids, d.ID)
	}
	assoc, err := o.Client.BatchAssociations(ctx, "deals", "contacts", ids)
	if err != nil {
		return nil, err
	}
	var contactExt []string
	for _, cs := range assoc {
		contactExt = append(contactExt, cs...)
	}
	local, err := s.localContacts(ctx, o, contactExt)
	if err != nil {
		return nil, err
	}
	var last *time.Time
	pipelinesRefreshed := false
	for _, d := range deals {
		mod := parseHSTime(d.Prop("hs_lastmodifieddate"))
		if mod != nil && (last == nil || mod.After(*last)) {
			last = mod
		}
		var contactID *uuid.UUID
		for _, ext := range assoc[d.ID] {
			if id, ok := local[ext]; ok {
				contactID = &id
				break
			}
		}
		created, mirrored, err := s.mirrorDeal(ctx, o, &d, contactID)
		if err != nil {
			return last, err
		}
		if !mirrored && !pipelinesRefreshed {
			pipelinesRefreshed = true
			if err := s.pullPipelines(ctx, o); err == nil {
				if created, mirrored, err = s.mirrorDeal(ctx, o, &d, contactID); err != nil {
					return last, err
				}
			}
		}
		if applyExit && created && mirrored && contactID != nil && o.Config.ExitRules.DealCreated && recentlyCreated(&d) {
			s.holdContact(ctx, o, *contactID, "A deal was opened in HubSpot: "+d.Prop("dealname"))
		}
	}
	return last, nil
}

// recentlyCreated keeps the deal exit rule to deals opened now, not history a
// pull happens to meet for the first time.
func recentlyCreated(d *Object) bool {
	created := parseHSTime(d.Prop("createdate"))
	return created != nil && time.Since(*created) < 48*time.Hour
}

// mirrorDeal writes one deal. mirrored is false when its stage is not one
// Warmbly knows (a pipeline the workspace chose not to mirror).
func (s *Service) mirrorDeal(ctx context.Context, o *org, d *Object, contactID *uuid.UUID) (created, mirrored bool, err error) {
	stage, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectStage, d.Prop("dealstage"))
	if err != nil || stage == nil {
		return false, false, err
	}
	pipe, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectPipeline, d.Prop("pipeline"))
	if err != nil || pipe == nil {
		return false, false, err
	}
	meta := stageMetaFromMap(stage.Meta)
	in := &repository.MirrorDeal{
		OrganizationID: o.ID,
		Provider:       provider,
		ExternalID:     d.ID,
		PipelineID:     pipe.LocalID,
		StageID:        stage.LocalID,
		ContactID:      contactID,
		Name:           firstNonEmpty(d.Prop("dealname"), "Untitled deal"),
		Currency:       d.Prop("deal_currency_code"),
		Status:         meta.status(),
		CreatedAt:      parseHSTime(d.Prop("createdate")),
		Meta:           ownerMeta(d.Prop("hubspot_owner_id")),
	}
	// numeric(12,2) holds under ten billion; a larger amount is left blank
	// rather than failing the page it arrived on.
	if v, perr := strconv.ParseFloat(d.Prop("amount"), 64); perr == nil && v > -1e10 && v < 1e10 {
		in.Value = &v
	}
	if cd := parseHSTime(d.Prop("closedate")); cd != nil {
		in.CloseDate = cd
		if meta.Closed {
			in.ClosedAt = cd
		}
	}
	if in.AssignedTo, err = s.d.Repo.UserForOwner(ctx, o.ID, provider, d.Prop("hubspot_owner_id")); err != nil {
		return false, false, err
	}
	_, created, err = s.d.Repo.MirrorDeal(ctx, in)
	return created, err == nil, err
}

// localContacts maps HubSpot contact ids to Warmbly contacts, linking any
// unlinked ones that match by email.
func (s *Service) localContacts(ctx context.Context, o *org, ext []string) (map[string]uuid.UUID, error) {
	out := map[string]uuid.UUID{}
	var unknown []string
	seen := map[string]bool{}
	for _, id := range ext {
		if seen[id] {
			continue
		}
		seen[id] = true
		rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			out[id] = rec.ContactID
		} else {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) == 0 {
		return out, nil
	}
	objs, err := o.Client.ReadObjects(ctx, "contacts", unknown, s.readProps(o))
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(objs))
	for _, ob := range objs {
		emails = append(emails, ob.Prop("email"))
	}
	byEmail, err := s.d.Repo.ContactIDsByEmail(ctx, o.ID, emails)
	if err != nil {
		return nil, err
	}
	for i := range objs {
		id, ok := byEmail[strings.ToLower(strings.TrimSpace(objs[i].Prop("email")))]
		if !ok {
			continue
		}
		if _, _, err := s.storeContactRecord(ctx, o, id, &objs[i]); err != nil {
			return nil, err
		}
		out[objs[i].ID] = id
	}
	return out, nil
}

// pullTasks mirrors the tasks owned by HubSpot users who are workspace members:
// each member's HubSpot task list appears on their Warmbly Tasks page.
func (s *Service) pullTasks(ctx context.Context, o *org) {
	owners, err := s.d.Repo.ListOwners(ctx, o.ID, provider)
	if err != nil {
		return
	}
	var mapped []string
	for _, ow := range owners {
		if ow.UserID != nil && !ow.Archived {
			mapped = append(mapped, ow.ExternalID)
		}
	}
	if len(mapped) == 0 {
		return
	}
	cursor, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorTasks)
	if err != nil {
		return
	}
	for page := 0; page < pagesPerPull; page++ {
		filters := []Filter{{PropertyName: "hubspot_owner_id", Operator: "IN", Values: mapped}}
		if cursor != nil {
			filters = append(filters, modifiedFilter("hs_lastmodifieddate", cursor)...)
		} else {
			since := time.Now().Add(-initialTaskWindow)
			filters = append(filters, Filter{PropertyName: "hs_lastmodifieddate", Operator: "GTE", Value: strconv.FormatInt(since.UnixMilli(), 10)})
		}
		res, err := o.Client.Search(ctx, "tasks", [][]Filter{filters}, "hs_lastmodifieddate", taskProps, "", 100)
		if err != nil {
			s.recordRun(ctx, o, cursorTasks, nil, err)
			return
		}
		if len(res.Results) == 0 {
			if cursor == nil {
				now := time.Now()
				cursor = &now
			}
			break
		}
		last, err := s.mirrorTasks(ctx, o, res.Results)
		if err != nil {
			s.recordRun(ctx, o, cursorTasks, nil, err)
			return
		}
		cursor = advance(cursor, last, len(res.Results) == 100)
		_ = s.d.Repo.SetCursor(ctx, o.ID, provider, cursorTasks, cursor, "")
		s.notify(ctx, o.ID, "", "task")
		if len(res.Results) < 100 {
			break
		}
	}
	s.recordRun(ctx, o, cursorTasks, cursor, nil)
}

func (s *Service) mirrorTasks(ctx context.Context, o *org, tasks []Object) (*time.Time, error) {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	contactAssoc, err := o.Client.BatchAssociations(ctx, "tasks", "contacts", ids)
	if err != nil {
		return nil, err
	}
	dealAssoc, err := o.Client.BatchAssociations(ctx, "tasks", "deals", ids)
	if err != nil {
		return nil, err
	}
	var contactExt []string
	for _, cs := range contactAssoc {
		contactExt = append(contactExt, cs...)
	}
	local, err := s.localContacts(ctx, o, contactExt)
	if err != nil {
		return nil, err
	}
	fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	var last *time.Time
	for i := range tasks {
		t := &tasks[i]
		if mod := parseHSTime(t.Prop("hs_lastmodifieddate")); mod != nil && (last == nil || mod.After(*last)) {
			last = mod
		}
		var contactID, dealID *uuid.UUID
		for _, ext := range contactAssoc[t.ID] {
			if id, ok := local[ext]; ok {
				contactID = &id
				break
			}
		}
		for _, ext := range dealAssoc[t.ID] {
			if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectDeal, ext); l != nil {
				id := l.LocalID
				dealID = &id
				break
			}
		}
		if err := s.mirrorTask(ctx, o, t, contactID, dealID, fallback); err != nil {
			return last, err
		}
	}
	return last, nil
}

func (s *Service) mirrorTask(ctx context.Context, o *org, t *Object, contactID, dealID *uuid.UUID, fallback uuid.UUID) error {
	owner := t.Prop("hubspot_owner_id")
	assigned, err := s.d.Repo.UserForOwner(ctx, o.ID, provider, owner)
	if err != nil {
		return err
	}
	createdBy := fallback
	if assigned != nil {
		createdBy = *assigned
	}
	in := &repository.MirrorTask{
		OrganizationID: o.ID,
		Provider:       provider,
		ExternalID:     t.ID,
		ContactID:      contactID,
		DealID:         dealID,
		AssignedTo:     assigned,
		CreatedBy:      createdBy,
		Title:          firstNonEmpty(t.Prop("hs_task_subject"), "Untitled task"),
		DueDate:        parseHSTime(t.Prop("hs_timestamp")),
		Priority:       localTaskPriority(t.Prop("hs_task_priority")),
		Type:           localTaskType(t.Prop("hs_task_type")),
		Status:         localTaskStatus(t.Prop("hs_task_status")),
		CreatedAt:      parseHSTime(t.Prop("hs_createdate")),
		Meta:           ownerMeta(owner),
	}
	if body := htmlToText(t.Prop("hs_task_body")); body != "" {
		in.Description = &body
	}
	if in.Status == models.CRMTaskStatusCompleted {
		in.CompletedAt = firstTime(parseHSTime(t.Prop("hs_task_completion_date")), parseHSTime(t.Prop("hs_lastmodifieddate")))
	}
	_, _, err = s.d.Repo.MirrorTask(ctx, in)
	return err
}

func firstTime(ts ...*time.Time) *time.Time {
	for _, t := range ts {
		if t != nil {
			return t
		}
	}
	now := time.Now()
	return &now
}

// pullContacts reads contacts changed in HubSpot and updates the ones Warmbly
// has: owner, lifecycle stage, lead status, opt-out and mapped fields.
func (s *Service) pullContacts(ctx context.Context, o *org) {
	cursor, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorContacts)
	if err != nil {
		return
	}
	if cursor == nil {
		// Nothing to catch up on: contacts link as Warmbly meets them.
		now := time.Now()
		s.recordRun(ctx, o, cursorContacts, &now, nil)
		return
	}
	touched := false
	for page := 0; page < contactPagesTick; page++ {
		res, err := o.Client.Search(ctx, "contacts", [][]Filter{modifiedFilter("lastmodifieddate", cursor)}, "lastmodifieddate", s.readProps(o), "", 100)
		if err != nil {
			s.recordRun(ctx, o, cursorContacts, nil, err)
			return
		}
		if len(res.Results) == 0 {
			break
		}
		var last *time.Time
		emails := make([]string, 0, len(res.Results))
		for _, c := range res.Results {
			emails = append(emails, c.Prop("email"))
		}
		byEmail, err := s.d.Repo.ContactIDsByEmail(ctx, o.ID, emails)
		if err != nil {
			return
		}
		for i := range res.Results {
			c := &res.Results[i]
			if mod := parseHSTime(c.Prop("lastmodifieddate")); mod != nil && (last == nil || mod.After(*last)) {
				last = mod
			}
			var contactID uuid.UUID
			if rec, _ := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, c.ID); rec != nil {
				contactID = rec.ContactID
			} else if id, ok := byEmail[strings.ToLower(strings.TrimSpace(c.Prop("email")))]; ok {
				contactID = id
			} else {
				continue
			}
			if err := s.applyContact(ctx, o, contactID, c); err != nil {
				s.recordRun(ctx, o, cursorContacts, nil, err)
				return
			}
			touched = true
		}
		cursor = advance(cursor, last, len(res.Results) == 100)
		_ = s.d.Repo.SetCursor(ctx, o.ID, provider, cursorContacts, cursor, "")
		if len(res.Results) < 100 {
			break
		}
	}
	if touched {
		s.notify(ctx, o.ID, "", "contact")
	}
	s.recordRun(ctx, o, cursorContacts, cursor, nil)
}

// applyContact stores a pulled contact, copies the fields HubSpot owns onto
// the Warmbly contact, and runs the exit rules on what changed.
func (s *Service) applyContact(ctx context.Context, o *org, contactID uuid.UUID, c *Object) error {
	rec, prev, err := s.storeContactRecord(ctx, o, contactID, c)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	custom := map[string]string{}
	for field, prop := range o.Config.FieldMap {
		// No direction is two-way: the dashboard stores "both" by leaving it out.
		if dir := o.Config.FieldDirection[field]; dir == models.CRMFieldPush {
			continue
		}
		v := c.Prop(prop)
		if v == "" {
			continue
		}
		if strings.HasPrefix(field, "custom:") {
			custom[strings.TrimPrefix(field, "custom:")] = v
		} else if field != "email" {
			fields[field] = v
		}
	}
	if err := s.d.Repo.UpdateContactFields(ctx, o.ID, contactID, fields, custom); err != nil {
		return err
	}
	rules := o.Config.ExitRules
	if prev != nil && rec.LifecycleStage != prev.LifecycleStage && slices.Contains(rules.LifecycleStages, rec.LifecycleStage) {
		label := s.optionLabel(ctx, o, "lifecyclestage", rec.LifecycleStage)
		s.holdContact(ctx, o, contactID, "Lifecycle stage became "+label+" in HubSpot")
	}
	if rec.OptedOut && (prev == nil || !prev.OptedOut) && rules.OptedOut {
		s.suppressOptOut(ctx, o, contactID, c.Prop("email"))
	}
	return nil
}

// holdContact parks the contact in every campaign, like a person's pause.
func (s *Service) holdContact(ctx context.Context, o *org, contactID uuid.UUID, reason string) {
	if s.d.Holds == nil {
		return
	}
	held, err := s.d.Holds.HoldLeadEverywhere(ctx, contactID, nil, reason, holdSourceCRM)
	if err != nil {
		log.Warn().Err(err).Str("contact_id", contactID.String()).Msg("hubspot: exit rule could not hold the contact")
		return
	}
	if len(held) > 0 {
		s.notify(ctx, o.ID, contactID.String(), "contact")
	}
}

func (s *Service) suppressOptOut(ctx context.Context, o *org, contactID uuid.UUID, email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if s.d.Suppress == nil || email == "" {
		return
	}
	if err := s.d.Suppress.UpsertSuppressedRecipient(ctx, &models.SuppressedRecipient{
		OrganizationID: o.ID,
		Email:          email,
		Kind:           models.SuppressionKindEmail,
		Reason:         "Opted out of email in HubSpot",
		Source:         models.DeliverabilityEventUnsubscribe,
		Metadata:       map[string]interface{}{"provider": string(provider), "contact_id": contactID.String()},
	}); err != nil {
		log.Warn().Err(err).Msg("hubspot: could not suppress an opted-out contact")
	}
}

// refreshContact pulls one contact with its deals, tasks and notes.
func (s *Service) refreshContact(ctx context.Context, o *org, contactID uuid.UUID, ext string) error {
	obj, err := o.Client.GetObject(ctx, "contacts", ext, s.readProps(o), []string{"deals", "tasks", "notes"})
	if err != nil {
		if ae, ok := AsAPIError(err); ok && ae.NotFound() {
			// Deleted or merged away in HubSpot: the contact is no longer linked.
			return s.d.Repo.DeleteContactRecord(ctx, o.ID, provider, ext)
		}
		return err
	}
	if err := s.applyContact(ctx, o, contactID, obj); err != nil {
		return err
	}
	if ids := obj.Associated("deals"); len(ids) > 0 {
		deals, err := o.Client.ReadObjects(ctx, "deals", ids, dealProps)
		if err != nil {
			return err
		}
		for i := range deals {
			if _, _, err := s.mirrorDeal(ctx, o, &deals[i], &contactID); err != nil {
				return err
			}
		}
	}
	fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
	if err != nil {
		return err
	}
	if ids := obj.Associated("tasks"); len(ids) > 0 {
		tasks, err := o.Client.ReadObjects(ctx, "tasks", ids, taskProps)
		if err != nil {
			return err
		}
		for i := range tasks {
			if err := s.mirrorTask(ctx, o, &tasks[i], &contactID, nil, fallback); err != nil {
				return err
			}
		}
	}
	if ids := obj.Associated("notes"); len(ids) > 0 {
		notes, err := o.Client.ReadObjects(ctx, "notes", ids, []string{"hs_note_body", "hs_timestamp", "hubspot_owner_id", "hs_createdate"})
		if err != nil {
			return err
		}
		for i := range notes {
			n := &notes[i]
			author := fallback
			if u, _ := s.d.Repo.UserForOwner(ctx, o.ID, provider, n.Prop("hubspot_owner_id")); u != nil {
				author = *u
			}
			body := htmlToText(n.Prop("hs_note_body"))
			if body == "" {
				continue
			}
			if _, _, err := s.d.Repo.MirrorNote(ctx, &repository.MirrorNote{
				OrganizationID: o.ID, Provider: provider, ExternalID: n.ID, ContactID: contactID, UserID: author,
				Content: body, CreatedAt: firstTime(parseHSTime(n.Prop("hs_timestamp")), parseHSTime(n.Prop("hs_createdate"))),
			}); err != nil {
				return err
			}
		}
	}
	s.notify(ctx, o.ID, contactID.String(), "contact", "deal", "task", "note")
	return nil
}

// gone reports a read HubSpot answered with "not found" or an archived record.
func gone(obj *Object, err error) bool {
	if err != nil {
		ae, ok := AsAPIError(err)
		return ok && ae.NotFound()
	}
	return obj != nil && obj.Archived
}

// refreshObject pulls one record HubSpot told us changed. A mirror is deleted
// only when HubSpot itself answers that the record is gone.
func (s *Service) refreshObject(ctx context.Context, o *org, objectType, ext string) error {
	switch objectType {
	case "contact":
		rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, ext)
		if err != nil || rec == nil {
			return err
		}
		return s.refreshContact(ctx, o, rec.ContactID, ext)
	case "deal":
		d, err := o.Client.GetObject(ctx, "deals", ext, dealProps, []string{"contacts"})
		if gone(d, err) {
			return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectDeal, ext)
		}
		if err != nil {
			return err
		}
		if len(o.Config.DealPipelines) > 0 && !slices.Contains(o.Config.DealPipelines, d.Prop("pipeline")) {
			return nil
		}
		local, err := s.localContacts(ctx, o, d.Associated("contacts"))
		if err != nil {
			return err
		}
		var contactID *uuid.UUID
		for _, id := range local {
			contactID = &id
			break
		}
		created, _, err := s.mirrorDeal(ctx, o, d, contactID)
		if err == nil && created && contactID != nil && o.Config.ExitRules.DealCreated && recentlyCreated(d) {
			s.holdContact(ctx, o, *contactID, "A deal was opened in HubSpot: "+d.Prop("dealname"))
		}
		s.notify(ctx, o.ID, contactIDString(contactID), "deal")
		return err
	case "task":
		t, err := o.Client.GetObject(ctx, "tasks", ext, taskProps, []string{"contacts", "deals"})
		if gone(t, err) {
			return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectTask, ext)
		}
		if err != nil {
			return err
		}
		known, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectTask, ext)
		if err != nil {
			return err
		}
		mapped, err := s.d.Repo.UserForOwner(ctx, o.ID, provider, t.Prop("hubspot_owner_id"))
		if err != nil {
			return err
		}
		if known == nil && mapped == nil {
			// Not a member's task and not one Warmbly shows: nothing to mirror.
			return nil
		}
		local, err := s.localContacts(ctx, o, t.Associated("contacts"))
		if err != nil {
			return err
		}
		var contactID, dealID *uuid.UUID
		for _, id := range local {
			contactID = &id
			break
		}
		for _, ext := range t.Associated("deals") {
			if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectDeal, ext); l != nil {
				id := l.LocalID
				dealID = &id
				break
			}
		}
		fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
		if err != nil {
			return err
		}
		if err := s.mirrorTask(ctx, o, t, contactID, dealID, fallback); err != nil {
			return err
		}
		s.notify(ctx, o.ID, contactIDString(contactID), "task")
		return nil
	default:
		return fmt.Errorf("unknown object type %q", objectType)
	}
}
