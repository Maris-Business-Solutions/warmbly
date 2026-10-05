package pipedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Pull keeps the mirror current: users and pipelines on a slow beat, deals,
// activities and persons incrementally by update time. Pipedrive meters API
// use per company per day, so webhooks carry the changes and the pull is the
// safety net: every five minutes without webhooks, every fifteen with them.
const (
	pullTick              = time.Minute
	pullEvery             = 5 * time.Minute
	pullEveryWithHooks    = 15 * time.Minute
	metaRefreshEvery      = 6 * time.Hour
	pagesPerPull          = 10
	cursorDeals           = "deals"
	cursorActivities      = "activities"
	cursorPersons         = "persons"
	holdSourceCRM         = "crm"
	initialActivityWindow = 90 * 24 * time.Hour
)

// RunPuller pulls every Pipedrive workspace on its cadence until ctx ends.
// Each workspace is claimed in Redis, so several consumers share the work.
func (s *Service) RunPuller(ctx context.Context) {
	t := time.NewTicker(pullTick)
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
		log.Warn().Err(err).Msg("pipedrive: list workspaces")
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		key := row.OrganizationID.String()
		every := pullEvery
		if s.hooksLive(ctx, row.OrganizationID) {
			every = pullEveryWithHooks
		}
		if !s.due(ctx, "pull:"+key, every, false) {
			continue
		}
		if !s.claim(ctx, "pull:"+key, 4*time.Minute) {
			continue
		}
		o, err := s.resolve(ctx, row.OrganizationID)
		if err != nil || o == nil {
			s.release(ctx, "pull:"+key)
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Str("org_id", o.ID.String()).Msg("pipedrive: pull panicked")
				}
			}()
			s.pullOrg(pctx, o, false)
		}()
		cancel()
		s.release(ctx, "pull:"+key)
	}
}

func (s *Service) claim(ctx context.Context, key string, ttl time.Duration) bool {
	if s.d.Cache == nil {
		return true
	}
	ok, err := s.d.Cache.SetNX(ctx, "pipedrive:lock:"+key, 1, ttl).Result()
	return err != nil || ok
}

func (s *Service) release(ctx context.Context, key string) {
	if s.d.Cache != nil {
		s.d.Cache.Del(ctx, "pipedrive:lock:"+key)
	}
}

// due reports whether a periodic step is due, and claims it.
func (s *Service) due(ctx context.Context, key string, every time.Duration, force bool) bool {
	if force || s.d.Cache == nil {
		return true
	}
	ok, err := s.d.Cache.SetNX(ctx, "pipedrive:due:"+key, 1, every).Result()
	return err != nil || ok
}

func (s *Service) pullOrg(ctx context.Context, o *org, full bool) {
	key := o.ID.String()
	if s.due(ctx, "meta:"+key, metaRefreshEvery, full) {
		s.recordRun(ctx, o, "owners", ptrNow(), s.pullOwners(ctx, o))
		s.recordRun(ctx, o, "pipelines", ptrNow(), s.pullPipelines(ctx, o))
		s.ensureWebhooks(ctx, o)
		if o.Config.WriteProperties {
			_ = s.ensureFields(ctx, o)
		}
	}
	s.pullDeals(ctx, o)
	s.pullActivities(ctx, o)
	s.pullPersons(ctx, o)
}

func (s *Service) recordRun(ctx context.Context, o *org, objectType string, at *time.Time, err error) {
	msg := ""
	if err != nil {
		msg = s.userError(ctx, o, err).Message
		at = nil
	}
	_ = s.d.Repo.SetCursor(ctx, o.ID, provider, objectType, at, msg)
}

func (s *Service) pullOwners(ctx context.Context, o *org) error {
	users, err := o.Client.Users(ctx)
	if err != nil {
		return err
	}
	out := make([]models.CRMOwner, 0, len(users))
	for _, u := range users {
		first, last, _ := strings.Cut(strings.TrimSpace(u.Name), " ")
		out = append(out, models.CRMOwner{ExternalID: id(u.ID), Email: u.Email, FirstName: first, LastName: strings.TrimSpace(last),
			Archived: !u.ActiveFlag || u.IsDeleted})
	}
	if err := s.d.Repo.ReplaceOwners(ctx, o.ID, provider, out); err != nil {
		return err
	}
	s.notify(ctx, o.ID, "", "owner")
	return nil
}

func (s *Service) pullPipelines(ctx context.Context, o *org) error {
	pipes, err := o.Client.Pipelines(ctx)
	if err != nil {
		return err
	}
	stages, err := o.Client.Stages(ctx)
	if err != nil {
		return err
	}
	byPipe := map[int64][]Stage{}
	for _, st := range stages {
		if !st.IsDeleted {
			byPipe[st.PipelineID] = append(byPipe[st.PipelineID], st)
		}
	}
	slices.SortStableFunc(pipes, func(a, b Pipeline) int { return a.OrderNr - b.OrderNr })
	var keep []string
	for i, p := range pipes {
		pid := id(p.ID)
		if p.IsDeleted || (len(o.Config.DealPipelines) > 0 && !slices.Contains(o.Config.DealPipelines, pid)) {
			continue
		}
		localID, err := s.d.Repo.MirrorPipeline(ctx, o.ID, provider, pid, p.Name, i)
		if err != nil {
			return err
		}
		keep = append(keep, pid)
		list := byPipe[p.ID]
		slices.SortStableFunc(list, func(a, b Stage) int { return a.OrderNr - b.OrderNr })
		var keepStages []string
		for j, st := range list {
			meta := stageMeta(st)
			meta["pipeline"] = pid
			if _, err := s.d.Repo.MirrorStage(ctx, o.ID, localID, provider, id(st.ID), st.Name, stagePalette[j%len(stagePalette)], j, meta); err != nil {
				return err
			}
			keepStages = append(keepStages, id(st.ID))
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

// pageV2 reads one page of a v2 collection.
func pageV2[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, string, error) {
	var page []T
	env, err := c.call(ctx, http.MethodGet, path, q, nil, &page)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if env.AdditionalData.NextCursor != nil {
		next = *env.AdditionalData.NextCursor
	}
	return page, next, nil
}

// pageState is where an unfinished pass stopped: the updated_since it was
// read with, Pipedrive's paging cursor, and the newest update time seen.
type pageState struct {
	Since  *time.Time `json:"since,omitempty"`
	Cursor string     `json:"cursor"`
	Latest *time.Time `json:"latest,omitempty"`
}

// incremental pages through records changed since the object's checkpoint,
// oldest first, handing each page to apply. A pass longer than one pull
// continues from Pipedrive's own paging cursor next time, so any number of
// records sharing one update time are all read; the checkpoint only moves
// once a pass is complete.
func incremental[T any](ctx context.Context, s *Service, o *org, cursorName, path string, q url.Values, since *time.Time,
	updated func(T) string, apply func([]T) error) {
	stateKey := "pipedrive:pass:" + o.ID.String() + ":" + cursorName
	var st pageState
	if s.d.Cache != nil {
		if raw, err := s.d.Cache.Get(ctx, stateKey).Bytes(); err == nil && json.Unmarshal(raw, &st) == nil && st.Cursor != "" {
			since = st.Since
		} else {
			st = pageState{}
		}
	}
	st.Since = since
	if since != nil {
		q.Set("updated_since", pdTime(*since))
	}
	q.Set("sort_by", "update_time")
	q.Set("sort_direction", "asc")
	q.Set("limit", "500")
	if st.Cursor != "" {
		q.Set("cursor", st.Cursor)
	}
	for page := 0; page < pagesPerPull; page++ {
		items, next, err := pageV2[T](ctx, o.Client, path, q)
		if err != nil {
			s.recordRun(ctx, o, cursorName, nil, err)
			return
		}
		if err := apply(items); err != nil {
			s.recordRun(ctx, o, cursorName, nil, err)
			return
		}
		for _, it := range items {
			if t := parseTime(updated(it)); t != nil && (st.Latest == nil || t.After(*st.Latest)) {
				st.Latest = t
			}
		}
		st.Cursor = next
		if next == "" || len(items) == 0 {
			break
		}
		q.Set("cursor", next)
	}
	if st.Cursor != "" && s.d.Cache != nil {
		// More to read: the next pull carries on from here.
		_ = s.d.Cache.SetJSON(ctx, stateKey, st, time.Hour)
		_ = s.d.Repo.SetCursor(ctx, o.ID, provider, cursorName, nil, "")
		return
	}
	if s.d.Cache != nil {
		s.d.Cache.Del(ctx, stateKey)
	}
	done := st.Latest
	if done == nil || (since != nil && done.Before(*since)) {
		done = since
	}
	if done == nil {
		done = ptrNow()
	}
	s.recordRun(ctx, o, cursorName, done, nil)
}

func (s *Service) pullDeals(ctx context.Context, o *org) {
	since, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorDeals)
	if err != nil {
		return
	}
	first := since == nil
	q := url.Values{}
	if !first {
		// Deleted deals are only listed for 30 days, which a cursor never outlives.
		q.Set("status", "open,won,lost,deleted")
	}
	touched := false
	incremental(ctx, s, o, cursorDeals, "/api/v2/deals", q, since, func(d Deal) string { return d.UpdateTime },
		func(deals []Deal) error {
			if len(deals) > 0 {
				touched = true
			}
			return s.mirrorDeals(ctx, o, deals, !first)
		})
	if touched {
		s.notify(ctx, o.ID, "", "deal")
	}
}

// mirrorDeals writes a page of deals. applyExit is false on the very first
// pull, so deals that existed before the switch stop nobody's campaigns.
func (s *Service) mirrorDeals(ctx context.Context, o *org, deals []Deal, applyExit bool) error {
	var people []int64
	for _, d := range deals {
		if d.PersonID != nil {
			people = append(people, *d.PersonID)
		}
	}
	local, err := s.localContacts(ctx, o, people)
	if err != nil {
		return err
	}
	pipelinesRefreshed := false
	for i := range deals {
		d := &deals[i]
		var contactID *uuid.UUID
		if d.PersonID != nil {
			if cid, ok := local[*d.PersonID]; ok {
				contactID = &cid
			}
		}
		created, mirrored, err := s.mirrorDeal(ctx, o, d, contactID)
		if err != nil {
			return err
		}
		if !mirrored && !pipelinesRefreshed && !d.IsDeleted && d.Status != "deleted" && s.pipelineWanted(o, d.PipelineID) {
			pipelinesRefreshed = true
			if err := s.pullPipelines(ctx, o); err == nil {
				if created, _, err = s.mirrorDeal(ctx, o, d, contactID); err != nil {
					return err
				}
			}
		}
		if applyExit && created && contactID != nil && o.Config.ExitRules.DealCreated && recentlyCreated(d) {
			s.holdContact(ctx, o, *contactID, "A deal was opened in Pipedrive: "+d.Title)
		}
	}
	return nil
}

func (s *Service) pipelineWanted(o *org, pipelineID int64) bool {
	return len(o.Config.DealPipelines) == 0 || slices.Contains(o.Config.DealPipelines, id(pipelineID))
}

// recentlyCreated keeps the deal exit rule to deals opened now, not history a
// pull happens to meet for the first time.
func recentlyCreated(d *Deal) bool {
	created := parseTime(d.AddTime)
	return created != nil && time.Since(*created) < 48*time.Hour
}

// mirrorDeal writes one deal. mirrored is false when its stage is not one
// Warmbly knows (a pipeline the workspace chose not to mirror).
func (s *Service) mirrorDeal(ctx context.Context, o *org, d *Deal, contactID *uuid.UUID) (created, mirrored bool, err error) {
	if d.IsDeleted || d.Status == "deleted" || !s.pipelineWanted(o, d.PipelineID) {
		return false, true, s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectDeal, id(d.ID))
	}
	stage, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectStage, id(d.StageID))
	if err != nil || stage == nil {
		return false, false, err
	}
	pipe, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectPipeline, id(d.PipelineID))
	if err != nil || pipe == nil {
		return false, false, err
	}
	in := &repository.MirrorDeal{
		OrganizationID: o.ID,
		Provider:       provider,
		ExternalID:     id(d.ID),
		PipelineID:     pipe.LocalID,
		StageID:        stage.LocalID,
		ContactID:      contactID,
		Name:           firstNonEmpty(d.Title, "Untitled deal"),
		Currency:       d.Currency,
		Status:         dealStatus(d.Status),
		CreatedAt:      parseTime(d.AddTime),
		CloseDate:      parsePtrTime(d.ExpectedCloseDate),
		Meta:           ownerMeta(ptrID(d.OwnerID)),
	}
	// numeric(12,2) holds under ten billion; a larger value is left blank
	// rather than failing the page it arrived on.
	if d.Value != nil && *d.Value > -1e10 && *d.Value < 1e10 {
		in.Value = d.Value
	}
	switch in.Status {
	case models.DealStatusWon:
		in.ClosedAt = firstTime(parsePtrTime(d.WonTime), parsePtrTime(d.CloseTime))
	case models.DealStatusLost:
		in.ClosedAt = firstTime(parsePtrTime(d.LostTime), parsePtrTime(d.CloseTime))
	}
	if in.AssignedTo, err = s.d.Repo.UserForOwner(ctx, o.ID, provider, ptrID(d.OwnerID)); err != nil {
		return false, false, err
	}
	_, created, err = s.d.Repo.MirrorDeal(ctx, in)
	return created, err == nil, err
}

// localContacts maps Pipedrive person ids to Warmbly contacts, linking any
// unlinked ones that match by email.
func (s *Service) localContacts(ctx context.Context, o *org, people []int64) (map[int64]uuid.UUID, error) {
	out := map[int64]uuid.UUID{}
	var unknown []int64
	seen := map[int64]bool{}
	for _, pid := range people {
		if pid == 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id(pid))
		if err != nil {
			return nil, err
		}
		if rec != nil {
			out[pid] = rec.ContactID
		} else {
			unknown = append(unknown, pid)
		}
	}
	if len(unknown) == 0 {
		return out, nil
	}
	persons, err := o.Client.PersonsByIDs(ctx, unknown)
	if err != nil {
		return nil, err
	}
	if err := s.linkPersons(ctx, o, persons, out); err != nil {
		return nil, err
	}
	return out, nil
}

// linkPersons stores the persons that match a Warmbly contact by email.
func (s *Service) linkPersons(ctx context.Context, o *org, persons []Person, into map[int64]uuid.UUID) error {
	emails := make([]string, 0, len(persons))
	for i := range persons {
		emails = append(emails, persons[i].Email())
	}
	byEmail, err := s.d.Repo.ContactIDsByEmail(ctx, o.ID, emails)
	if err != nil {
		return err
	}
	for i := range persons {
		cid, ok := byEmail[persons[i].Email()]
		if !ok {
			continue
		}
		if _, _, err := s.storeContactRecord(ctx, o, cid, &persons[i]); err != nil {
			return err
		}
		if into != nil {
			into[persons[i].ID] = cid
		}
	}
	return nil
}

// isEmailLog reports an activity that records an email rather than asks for
// one: Warmbly's own logged sends and replies, and any done email activity.
func isEmailLog(a *Activity) bool {
	return a.Done && a.Type == "email"
}

// pullActivities mirrors the activities owned by Pipedrive users who are
// workspace members: each member's Pipedrive activities appear on their
// Warmbly Tasks page.
func (s *Service) pullActivities(ctx context.Context, o *org) {
	owners, err := s.d.Repo.ListOwners(ctx, o.ID, provider)
	if err != nil {
		return
	}
	mapped := map[string]bool{}
	for _, ow := range owners {
		if ow.UserID != nil && !ow.Archived {
			mapped[ow.ExternalID] = true
		}
	}
	if len(mapped) == 0 {
		return
	}
	since, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorActivities)
	if err != nil {
		return
	}
	if since == nil {
		t := time.Now().Add(-initialActivityWindow)
		since = &t
	}
	fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
	if err != nil {
		return
	}
	touched := false
	incremental(ctx, s, o, cursorActivities, "/api/v2/activities", url.Values{}, since, func(a Activity) string { return a.UpdateTime },
		func(acts []Activity) error {
			var keep []Activity
			for i := range acts {
				a := &acts[i]
				if isEmailLog(a) {
					continue
				}
				if a.IsDeleted {
					if err := s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectTask, id(a.ID)); err != nil {
						return err
					}
					continue
				}
				if mapped[ptrID(a.OwnerID)] {
					keep = append(keep, *a)
				} else if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectTask, id(a.ID)); l != nil {
					keep = append(keep, *a)
				}
			}
			if len(keep) > 0 {
				touched = true
			}
			return s.mirrorActivities(ctx, o, keep, fallback)
		})
	if touched {
		s.notify(ctx, o.ID, "", "task")
	}
}

func (s *Service) mirrorActivities(ctx context.Context, o *org, acts []Activity, fallback uuid.UUID) error {
	if len(acts) == 0 {
		return nil
	}
	var people []int64
	for i := range acts {
		if p := acts[i].Person(); p != 0 {
			people = append(people, p)
		}
	}
	local, err := s.localContacts(ctx, o, people)
	if err != nil {
		return err
	}
	for i := range acts {
		a := &acts[i]
		var contactID *uuid.UUID
		if cid, ok := local[a.Person()]; ok {
			contactID = &cid
		}
		if err := s.mirrorActivity(ctx, o, a, contactID, fallback); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) mirrorActivity(ctx context.Context, o *org, a *Activity, contactID *uuid.UUID, fallback uuid.UUID) error {
	owner := ptrID(a.OwnerID)
	assigned, err := s.d.Repo.UserForOwner(ctx, o.ID, provider, owner)
	if err != nil {
		return err
	}
	createdBy := fallback
	if assigned != nil {
		createdBy = *assigned
	}
	var dealID *uuid.UUID
	if a.DealID != nil {
		if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectDeal, id(*a.DealID)); l != nil {
			did := l.LocalID
			dealID = &did
		}
	}
	in := &repository.MirrorTask{
		OrganizationID: o.ID,
		Provider:       provider,
		ExternalID:     id(a.ID),
		ContactID:      contactID,
		DealID:         dealID,
		AssignedTo:     assigned,
		CreatedBy:      createdBy,
		Title:          firstNonEmpty(a.Subject, "Untitled activity"),
		DueDate:        dueAt(a.DueDate, a.DueTime),
		Priority:       models.CRMTaskPriorityMedium,
		Type:           s.activityName(ctx, o, a.Type),
		Status:         models.CRMTaskStatusPending,
		CreatedAt:      parseTime(a.AddTime),
		Meta:           ownerMeta(owner),
	}
	if body := htmlToText(a.Note); body != "" {
		in.Description = &body
	}
	// Pipedrive knows done and not done; Warmbly's finer states and priority
	// survive a pull while they agree with it.
	if l, _ := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectTask, id(a.ID)); l != nil {
		if cur, err := s.d.CRM.GetCRMTask(ctx, o.ID, l.LocalID); err == nil && cur != nil {
			in.Priority = cur.Priority
			doneHere := cur.Status == models.CRMTaskStatusCompleted || cur.Status == models.CRMTaskStatusCancelled
			if doneHere == a.Done {
				in.Status = cur.Status
			}
		}
	}
	if a.Done && in.Status != models.CRMTaskStatusCancelled {
		in.Status = models.CRMTaskStatusCompleted
	}
	if in.Status == models.CRMTaskStatusCompleted || in.Status == models.CRMTaskStatusCancelled {
		in.CompletedAt = firstTime(parseTime(a.MarkedAsDoneTime), parseTime(a.UpdateTime))
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
	return ptrNow()
}

// pullPersons reads persons changed in Pipedrive and updates the ones Warmbly
// has: owner, labels, organization, opt-out and mapped fields.
func (s *Service) pullPersons(ctx context.Context, o *org) {
	since, err := s.d.Repo.GetCursor(ctx, o.ID, provider, cursorPersons)
	if err != nil {
		return
	}
	if since == nil {
		// Nothing to catch up on: contacts link as Warmbly meets them.
		s.recordRun(ctx, o, cursorPersons, ptrNow(), nil)
		return
	}
	touched := false
	incremental(ctx, s, o, cursorPersons, "/api/v2/persons", o.Client.withMarketing(nil), since, func(p Person) string { return p.UpdateTime },
		func(persons []Person) error {
			emails := make([]string, 0, len(persons))
			for i := range persons {
				emails = append(emails, persons[i].Email())
			}
			byEmail, err := s.d.Repo.ContactIDsByEmail(ctx, o.ID, emails)
			if err != nil {
				return err
			}
			for i := range persons {
				p := &persons[i]
				var contactID uuid.UUID
				if rec, _ := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id(p.ID)); rec != nil {
					contactID = rec.ContactID
					if p.IsDeleted {
						if err := s.d.Repo.DeleteContactRecord(ctx, o.ID, provider, id(p.ID)); err != nil {
							return err
						}
						touched = true
						continue
					}
				} else if cid, ok := byEmail[p.Email()]; ok && !p.IsDeleted {
					contactID = cid
				} else {
					continue
				}
				if err := s.applyContact(ctx, o, contactID, p); err != nil {
					return err
				}
				touched = true
			}
			return nil
		})
	if touched {
		s.notify(ctx, o.ID, "", "contact")
	}
}

// applyContact stores a pulled person, copies the fields Pipedrive owns onto
// the Warmbly contact, and runs the exit rules on what changed.
func (s *Service) applyContact(ctx context.Context, o *org, contactID uuid.UUID, p *Person) error {
	rec, prev, err := s.storeContactRecord(ctx, o, contactID, p)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	custom := map[string]string{}
	for field, key := range o.Config.FieldMap {
		// No direction is two-way: the dashboard stores "both" by leaving it out.
		if dir := o.Config.FieldDirection[field]; dir == models.CRMFieldPush {
			continue
		}
		v := personProp(p, key, rec.CompanyName)
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
	if prev != nil {
		for _, l := range newLabels(prev, rec) {
			if slices.Contains(rules.LifecycleStages, l) {
				s.holdContact(ctx, o, contactID, "Labeled "+s.labelName(ctx, o, l)+" in Pipedrive")
				break
			}
		}
	}
	if rec.OptedOut && (prev == nil || !prev.OptedOut) && rules.OptedOut {
		s.suppressOptOut(ctx, o, contactID, p.Email())
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
		log.Warn().Err(err).Str("contact_id", contactID.String()).Msg("pipedrive: exit rule could not hold the contact")
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
		Reason:         "Unsubscribed from email marketing in Pipedrive",
		Source:         models.DeliverabilityEventUnsubscribe,
		Metadata:       map[string]interface{}{"provider": string(provider), "contact_id": contactID.String()},
	}); err != nil {
		log.Warn().Err(err).Msg("pipedrive: could not suppress an opted-out contact")
	}
}

// refreshContact pulls one person; deep also reads their deals, activities
// and notes, which cost more of the company's daily API budget.
func (s *Service) refreshContact(ctx context.Context, o *org, contactID uuid.UUID, ext string, deep bool) error {
	pid := extID(ext)
	p, err := o.Client.GetPerson(ctx, pid)
	if gone(p != nil && p.IsDeleted, err) {
		// Deleted or merged away in Pipedrive: the contact is no longer linked.
		return s.d.Repo.DeleteContactRecord(ctx, o.ID, provider, ext)
	}
	if err != nil {
		return err
	}
	if err := s.applyContact(ctx, o, contactID, p); err != nil {
		return err
	}
	if !deep {
		s.notify(ctx, o.ID, contactID.String(), "contact")
		return nil
	}
	deals, err := o.Client.Deals(ctx, url.Values{"person_id": {id(pid)}}, 200)
	if err != nil {
		return err
	}
	for i := range deals {
		if _, _, err := s.mirrorDeal(ctx, o, &deals[i], &contactID); err != nil {
			return err
		}
	}
	fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
	if err != nil {
		return err
	}
	acts, err := o.Client.Activities(ctx, url.Values{"person_id": {id(pid)}}, 200)
	if err != nil {
		return err
	}
	for i := range acts {
		if isEmailLog(&acts[i]) || acts[i].IsDeleted {
			continue
		}
		if err := s.mirrorActivity(ctx, o, &acts[i], &contactID, fallback); err != nil {
			return err
		}
	}
	notes, err := o.Client.Notes(ctx, url.Values{"person_id": {id(pid)}, "sort": {"update_time DESC"}}, 100)
	if err != nil {
		return err
	}
	for i := range notes {
		if err := s.mirrorNote(ctx, o, &notes[i], contactID, fallback); err != nil {
			return err
		}
	}
	s.notify(ctx, o.ID, contactID.String(), "contact", "deal", "task", "note")
	return nil
}

func (s *Service) mirrorNote(ctx context.Context, o *org, n *Note, contactID, fallback uuid.UUID) error {
	if !n.ActiveFlag {
		return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectNote, id(n.ID))
	}
	body := htmlToText(n.Content)
	if body == "" {
		return nil
	}
	author := fallback
	if u, _ := s.d.Repo.UserForOwner(ctx, o.ID, provider, id(n.UserID)); u != nil {
		author = *u
	}
	_, _, err := s.d.Repo.MirrorNote(ctx, &repository.MirrorNote{
		OrganizationID: o.ID, Provider: provider, ExternalID: id(n.ID), ContactID: contactID, UserID: author,
		Content: body, CreatedAt: firstTime(parseTime(n.AddTime)),
	})
	return err
}

// gone reports a read Pipedrive answered with "not found", or a deleted record.
func gone(deleted bool, err error) bool {
	if err != nil {
		ae, ok := AsAPIError(err)
		return ok && ae.NotFound()
	}
	return deleted
}

// refreshObject pulls one record a webhook said changed. A mirror is deleted
// only when Pipedrive itself answers that the record is gone.
func (s *Service) refreshObject(ctx context.Context, o *org, objectType, ext string) error {
	n := extID(ext)
	switch objectType {
	case "pipelines":
		return s.pullPipelines(ctx, o)
	case "person":
		if rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, ext); err != nil {
			return err
		} else if rec != nil {
			return s.refreshContact(ctx, o, rec.ContactID, ext, false)
		}
		p, err := o.Client.GetPerson(ctx, n)
		if gone(p != nil && p.IsDeleted, err) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.linkPersons(ctx, o, []Person{*p}, nil)
	case "deal":
		d, err := o.Client.GetDeal(ctx, n)
		if gone(d != nil && d.IsDeleted, err) {
			return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectDeal, ext)
		}
		if err != nil {
			return err
		}
		var contactID *uuid.UUID
		if d.PersonID != nil {
			local, err := s.localContacts(ctx, o, []int64{*d.PersonID})
			if err != nil {
				return err
			}
			if cid, ok := local[*d.PersonID]; ok {
				contactID = &cid
			}
		}
		created, mirrored, err := s.mirrorDeal(ctx, o, d, contactID)
		if err == nil && !mirrored {
			if err = s.pullPipelines(ctx, o); err == nil {
				created, _, err = s.mirrorDeal(ctx, o, d, contactID)
			}
		}
		if err == nil && created && contactID != nil && o.Config.ExitRules.DealCreated && recentlyCreated(d) {
			s.holdContact(ctx, o, *contactID, "A deal was opened in Pipedrive: "+d.Title)
		}
		s.notify(ctx, o.ID, contactIDString(contactID), "deal")
		return err
	case "activity":
		a, err := o.Client.GetActivity(ctx, n)
		if gone(a != nil && a.IsDeleted, err) {
			return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectTask, ext)
		}
		if err != nil {
			return err
		}
		if isEmailLog(a) {
			return nil
		}
		known, err := s.d.Repo.GetLinkByExternal(ctx, o.ID, provider, models.CRMObjectTask, ext)
		if err != nil {
			return err
		}
		mapped, err := s.d.Repo.UserForOwner(ctx, o.ID, provider, ptrID(a.OwnerID))
		if err != nil {
			return err
		}
		if known == nil && mapped == nil {
			return nil
		}
		fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
		if err != nil {
			return err
		}
		if err := s.mirrorActivities(ctx, o, []Activity{*a}, fallback); err != nil {
			return err
		}
		s.notify(ctx, o.ID, "", "task")
		return nil
	case "note":
		note, err := o.Client.GetNote(ctx, n)
		if gone(note != nil && !note.ActiveFlag, err) {
			return s.d.Repo.DeleteMirrored(ctx, o.ID, provider, models.CRMObjectNote, ext)
		}
		if err != nil || note.PersonID == nil {
			return err
		}
		rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, id(*note.PersonID))
		if err != nil || rec == nil {
			return err
		}
		fallback, err := s.d.Repo.FallbackActor(ctx, o.ID)
		if err != nil {
			return err
		}
		if err := s.mirrorNote(ctx, o, note, rec.ContactID, fallback); err != nil {
			return err
		}
		s.notify(ctx, o.ID, rec.ContactID.String(), "note")
		return nil
	default:
		return fmt.Errorf("unknown object type %q", objectType)
	}
}
