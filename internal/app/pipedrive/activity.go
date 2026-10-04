package pipedrive

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// OnEvent is the platform event sink: it turns sends, replies, bounces,
// unsubscribes, opens, clicks and bookings into outbox jobs. It runs before
// the webhook throttle, so Pipedrive sees every send even when a burst is too
// large for webhook fan-out.
func (s *Service) OnEvent(ctx context.Context, orgID uuid.UUID, eventType models.WebhookEventType, data any) {
	m, ok := data.(map[string]any)
	if !ok || orgID == uuid.Nil || !s.Active(ctx, orgID) {
		return
	}
	row, err := s.settingsRow(ctx, orgID)
	if err != nil || row == nil {
		return
	}
	act := row.Config.Activity
	job := &models.CRMSyncJob{OrganizationID: orgID, Provider: provider, Payload: map[string]any{}}
	copyKeys(job.Payload, m, "contact_id", "contact_email", "campaign_id", "intent", "reason", "subject")
	email := str(m, "contact_email")
	job.Subject = email
	switch eventType {
	case models.WebhookEventCampaignEmailSent:
		job.Kind = models.CRMJobLogEmail
		job.Payload["direction"] = "out"
		copyKeys(job.Payload, m, "from_email")
		job.Payload["subject"] = str(m, "_subject")
		job.Payload["body"] = str(m, "_body_text")
		job.Payload["task_id"] = str(m, "_task_id")
		job.Payload["email_account_id"] = str(m, "_email_account_id")
		job.Payload["at"] = time.Now().UTC().Format(time.RFC3339)
		if t := str(m, "_task_id"); t != "" {
			job.DedupeKey = "email:" + t
		}
		if !act.Sent && !row.Config.WriteProperties {
			return
		}
	case models.WebhookEventCampaignReplyReceived:
		job.Kind = models.CRMJobLogEmail
		job.Payload["direction"] = "in"
		job.Payload["body"] = firstNonEmpty(str(m, "_body_text"), str(m, "snippet"))
		job.Payload["email_account_id"] = str(m, "email_account_id")
		job.Payload["from_email"] = email
		job.Payload["to_email"] = str(m, "_mailbox_email")
		job.Payload["at"] = time.Now().UTC().Format(time.RFC3339)
		if mid := str(m, "_message_id"); mid != "" {
			job.DedupeKey = "reply:" + mid
		}
	case models.WebhookEventCampaignEmailBounced:
		job.Kind = models.CRMJobLogEvent
		job.Payload["event"] = "bounce"
		job.Payload["task_id"] = str(m, "_task_id")
		job.DedupeKey = "bounce:" + email + ":" + str(m, "campaign_id")
	case models.WebhookEventCampaignUnsubscribed:
		job.Kind = models.CRMJobLogEvent
		job.Payload["event"] = "unsubscribe"
		job.DedupeKey = "unsub:" + email
	case models.WebhookEventCampaignEmailOpened, models.WebhookEventCampaignEmailClicked:
		if !row.Config.WriteProperties || (eventType == models.WebhookEventCampaignEmailOpened && !act.Opens) ||
			(eventType == models.WebhookEventCampaignEmailClicked && !act.Clicks) {
			return
		}
		job.Kind = models.CRMJobLogEvent
		job.Payload["event"] = "open"
		if eventType == models.WebhookEventCampaignEmailClicked {
			job.Payload["event"] = "click"
		}
		job.Payload["at"] = time.Now().UTC().Format(time.RFC3339)
		// "Last opened" is a date in Pipedrive: one write per contact per day.
		job.DedupeKey = fmt.Sprintf("%s:%s:%s", job.Payload["event"], email, time.Now().UTC().Format("2006-01-02"))
	case models.WebhookEventContactUpdated:
		// A Warmbly edit is the most recent change: two-way and push fields go
		// to Pipedrive. Pipedrive's own edits arrive through the pull, which
		// writes the contact without emitting this event, so nothing loops.
		if str(m, "entity_type") != string(models.AuditEntityContact) {
			return
		}
		if md, ok := m["metadata"].(map[string]string); ok && md["crm"] != "" {
			return
		}
		cid := str(m, "entity_id")
		if _, err := uuid.Parse(cid); err != nil {
			return
		}
		job.Kind = models.CRMJobPushContact
		job.Payload = map[string]any{"contact_id": cid}
		job.Subject = "Contact " + cid[:8]
		job.DedupeKey = "contact:" + cid
		job.NextAttemptAt = time.Now().Add(10 * time.Second)
	case models.WebhookEventMeetingBooked:
		if !act.Meetings {
			return
		}
		job.Kind = models.CRMJobLogMeeting
		copyKeys(job.Payload, m, "event_name", "scheduled_for", "invitee_email", "invitee_name", "source", "external_event_id")
		if email == "" {
			job.Subject = str(m, "invitee_email")
			job.Payload["contact_email"] = str(m, "invitee_email")
		}
		job.DedupeKey = "meeting:" + str(m, "source") + ":" + str(m, "external_event_id")
	default:
		return
	}
	if job.Subject == "" {
		job.Subject = job.Kind
	}
	if err := s.d.Repo.EnqueueJob(ctx, job); err != nil {
		log.Warn().Err(err).Str("org_id", orgID.String()).Str("kind", job.Kind).Msg("pipedrive: could not queue activity")
	}
}

func copyKeys(dst, src map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := src[k]; ok && v != nil && fmt.Sprint(v) != "" {
			dst[k] = v
		}
	}
}

func str(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// mailboxOwner is the Pipedrive user who owns the sending mailbox.
func (s *Service) mailboxOwner(ctx context.Context, o *org, p map[string]any) int64 {
	acct, err := uuid.Parse(str(p, "email_account_id"))
	if err != nil {
		return 0
	}
	uid, _ := s.d.Repo.MailboxUser(ctx, o.ID, acct)
	return extID(s.ownerFor(ctx, o, uid))
}

// jobContact resolves the Warmbly contact a job is about. create allows a
// Pipedrive person to be created for them (sends and replies, never opens).
func (s *Service) jobContact(ctx context.Context, o *org, p map[string]any, create bool) (uuid.UUID, string, *models.CRMContactRecord, error) {
	var contactID uuid.UUID
	if cid, err := uuid.Parse(str(p, "contact_id")); err == nil {
		contactID = cid
	} else if email := str(p, "contact_email"); email != "" {
		c, xerr := s.d.Contacts.GetByEmailAndOrganization(ctx, o.ID, email)
		if xerr != nil || c == nil {
			return uuid.Nil, "", nil, nil
		}
		contactID = c.ID
	} else {
		return uuid.Nil, "", nil, nil
	}
	if !create {
		rec, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
		if err != nil || rec == nil {
			return contactID, "", nil, err
		}
		return contactID, rec.ExternalID, rec, nil
	}
	ext, rec, err := s.ensureContactOwned(ctx, o, contactID, s.mailboxOwner(ctx, o, p))
	return contactID, ext, rec, err
}

// openDeal is the contact's first open deal in Pipedrive, or 0.
func (s *Service) openDeal(ctx context.Context, o *org, contactID uuid.UUID) int64 {
	deals, err := s.d.CRM.GetDealsByContact(ctx, o.ID, contactID)
	if err != nil {
		return 0
	}
	var ids []uuid.UUID
	for _, d := range deals {
		if d.Status == models.DealStatusOpen {
			ids = append(ids, d.ID)
		}
	}
	links, err := s.d.Repo.LinksForLocal(ctx, o.ID, provider, models.CRMObjectDeal, ids)
	if err != nil {
		return 0
	}
	for _, did := range ids {
		if l, ok := links[did]; ok {
			return extID(l.ExternalID)
		}
	}
	return 0
}

// logEmail writes a send or a reply as a done Email activity on the person,
// their organization and their open deal, and updates the Warmbly fields.
//
// A retry never logs the same email twice: the logged activity is linked to
// the send's task (or, for a reply, to this job) before anything else can fail.
func (s *Service) logEmail(ctx context.Context, o *org, job *models.CRMSyncJob) error {
	p := job.Payload
	contactID, ext, rec, err := s.jobContact(ctx, o, p, true)
	if err != nil || ext == "" {
		return err
	}
	logKey := job.ID
	inbound := str(p, "direction") == "in"
	if taskID, perr := uuid.Parse(str(p, "task_id")); perr == nil && !inbound {
		logKey = taskID
	}
	at := parseTime(str(p, "at"))
	if at == nil {
		at = ptrNow()
	}
	owner := s.mailboxOwner(ctx, o, p)
	if owner == 0 && rec != nil {
		owner = extID(rec.OwnerExternalID)
	}

	logIt := (inbound && o.Config.Activity.Replies) || (!inbound && o.Config.Activity.Sent)
	if logIt {
		if done, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectEmail, logKey); err != nil {
			return err
		} else if done != nil {
			logIt = false
		}
	}
	campaign := ""
	if cid, err := uuid.Parse(str(p, "campaign_id")); err == nil {
		campaign, _ = s.d.Repo.CampaignName(ctx, o.ID, cid)
	}
	if logIt {
		subject := firstNonEmpty(str(p, "subject"), "(no subject)")
		contactEmail := str(p, "contact_email")
		from, to := str(p, "from_email"), contactEmail
		title := subject
		if inbound {
			from, to = contactEmail, str(p, "to_email")
			title = "Reply: " + subject
		}
		var head strings.Builder
		head.WriteString("<p><b>From:</b> " + html.EscapeString(from) + "<br><b>To:</b> " + html.EscapeString(to))
		if campaign != "" {
			head.WriteString("<br><b>Campaign:</b> " + html.EscapeString(campaign))
		}
		head.WriteString("<br><i>Logged by Warmbly</i></p>")
		body := map[string]any{
			"subject":      truncateRunes(title, 250),
			"type":         "email",
			"done":         true,
			"due_date":     pdDate(*at),
			"due_time":     pdClock(*at),
			"note":         head.String() + "<p>" + textToHTML(truncateRunes(str(p, "body"), 60000)) + "</p>",
			"participants": []Participant{{PersonID: extID(ext), Primary: true}},
		}
		if owner != 0 {
			body["owner_id"] = owner
		}
		if rec != nil && rec.CompanyExternalID != "" {
			body["org_id"] = extID(rec.CompanyExternalID)
		}
		if deal := s.openDeal(ctx, o, contactID); deal != 0 {
			body["deal_id"] = deal
		}
		created, err := o.Client.CreateActivity(ctx, body)
		if err != nil {
			return err
		}
		if err := s.d.Repo.UpsertLink(ctx, &models.CRMExternalLink{OrganizationID: o.ID, Provider: provider,
			ObjectType: models.CRMObjectEmail, LocalID: logKey, ExternalID: id(created.ID),
			Meta: map[string]any{"subject": truncateRunes(title, 250)}}); err != nil {
			return err
		}
	}

	if o.Config.WriteProperties {
		values := map[string]string{}
		if link := s.contactLink(contactID); link != "" {
			values[fieldLink] = link
		}
		if inbound {
			values[fieldLastReplied] = pdDate(*at)
			intent := str(p, "intent")
			values[fieldReplyIntent] = intent
			values[fieldStatus] = statusReplied
			switch models.ReplyIntentType(intent) {
			case models.ReplyIntentPositive:
				values[fieldStatus] = statusInterested
			case models.ReplyIntentNegative:
				values[fieldStatus] = statusNotInterest
			}
		} else {
			values[fieldLastContacted] = pdDate(*at)
			if campaign != "" {
				values[fieldLastCampaign] = campaign
			}
			if cur := currentStatus(rec); cur == "" || cur == statusContacted {
				values[fieldStatus] = statusContacted
			}
		}
		body := map[string]any{}
		if custom := s.warmblyValues(ctx, o, values); len(custom) > 0 {
			body["custom_fields"] = custom
		}
		if !inbound {
			if contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID}); xerr == nil && len(contacts) == 1 {
				for k, v := range s.pushOnlyBody(ctx, o, &contacts[0]) {
					if k == "custom_fields" {
						custom, _ := body["custom_fields"].(map[string]any)
						if custom == nil {
							custom = map[string]any{}
						}
						for ck, cv := range v.(map[string]any) {
							custom[ck] = cv
						}
						body["custom_fields"] = custom
						continue
					}
					body[k] = v
				}
			}
		}
		if len(body) > 0 {
			if _, err := o.Client.UpdatePerson(ctx, extID(ext), body); err != nil {
				return err
			}
		}
		s.rememberStatus(ctx, o, contactID, values[fieldStatus])
	}

	if inbound && models.ReplyIntentType(str(p, "intent")) == models.ReplyIntentPositive {
		// Its own job, so a failure there retries the outcome and not the email.
		return s.d.Repo.EnqueueJob(ctx, &models.CRMSyncJob{
			OrganizationID: o.ID, Provider: provider, Kind: models.CRMJobReplyOutcome,
			DedupeKey: "outcome:" + job.ID.String(), Subject: job.Subject,
			Payload: map[string]any{"contact_id": contactID.String(), "campaign_id": str(p, "campaign_id"),
				"email_account_id": str(p, "email_account_id")},
		})
	}
	s.notify(ctx, o.ID, contactID.String(), "contact")
	return nil
}

func currentStatus(rec *models.CRMContactRecord) string {
	if rec == nil {
		return ""
	}
	return rec.Properties[statusKey]
}

// rememberStatus keeps the last written Warmbly status on the record, so a
// follow-up send never turns "Interested" back into "Contacted".
func (s *Service) rememberStatus(ctx context.Context, o *org, contactID uuid.UUID, status string) {
	if status == "" {
		return
	}
	rec, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
	if err != nil || rec == nil {
		return
	}
	if rec.Properties == nil {
		rec.Properties = map[string]string{}
	}
	rec.Properties[statusKey] = status
	_ = s.d.Repo.UpsertContactRecord(ctx, rec)
}

// pushOnlyBody is the mapped fields Warmbly owns outright.
func (s *Service) pushOnlyBody(ctx context.Context, o *org, c *models.Contact) map[string]any {
	narrowed := *o
	narrowed.Config.FieldMap = map[string]string{}
	for field, key := range o.Config.FieldMap {
		if o.Config.FieldDirection[field] == models.CRMFieldPush {
			narrowed.Config.FieldMap[field] = key
		}
	}
	if len(narrowed.Config.FieldMap) == 0 {
		return nil
	}
	return s.personBody(ctx, &narrowed, c, false)
}

// replyOutcomeJob runs the interested-reply outcome for a contact in Pipedrive.
func (s *Service) replyOutcomeJob(ctx context.Context, o *org, p map[string]any) error {
	contactID, ext, rec, err := s.jobContact(ctx, o, p, false)
	if err != nil || ext == "" {
		return err
	}
	return s.replyOutcome(ctx, o, contactID, ext, rec, p)
}

// replyOutcome is what an interested reply does in Pipedrive: a label on the
// person, a lead in the Leads Inbox, and optionally a deal.
func (s *Service) replyOutcome(ctx context.Context, o *org, contactID uuid.UUID, ext string, rec *models.CRMContactRecord, p map[string]any) error {
	cfg := o.Config.PositiveReply
	if label := extID(cfg.LifecycleStage); label != 0 {
		// Labels are a set: the person keeps the ones they already have.
		current := []string{}
		if rec != nil {
			current = splitIDs(rec.Properties["label_ids"])
		}
		if !slices.Contains(current, id(label)) {
			labels := append(toIDs(current), label)
			if _, err := o.Client.UpdatePerson(ctx, extID(ext), map[string]any{"label_ids": labels}); err != nil {
				return err
			}
		}
	}
	owner := s.mailboxOwner(ctx, o, p)
	hasOpenDeal := s.openDeal(ctx, o, contactID) != 0
	contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID})
	if xerr != nil || len(contacts) == 0 {
		return s.afterOutcome(ctx, o, contactID)
	}
	c := contacts[0]
	name := strings.TrimSpace(firstNonEmpty(c.Company, strings.TrimSpace(c.FirstName+" "+c.LastName), c.Email))
	if cfg.CreateLead && !hasOpenDeal {
		lead := map[string]any{"title": truncateRunes(name, 250), "person_id": extID(ext)}
		if rec != nil && rec.CompanyExternalID != "" {
			lead["organization_id"] = extID(rec.CompanyExternalID)
		}
		if owner != 0 {
			lead["owner_id"] = owner
		}
		// Pipedrive has no idempotency key for leads: a retried outcome skips a
		// lead it already made.
		key := "pipedrive:lead:" + contactID.String()
		if !s.marked(ctx, key) {
			err := o.Client.CreateLead(ctx, lead)
			if ae, ok := AsAPIError(err); ok && ae.AuthProblem() {
				// The app or the connected user may not add leads; the rest of
				// the outcome still runs.
				log.Info().Str("org_id", o.ID.String()).Msg("pipedrive: no permission to add a lead")
				err = nil
			}
			if err != nil {
				return err
			}
			s.mark(ctx, key, 24*time.Hour)
		}
	}
	if cfg.CreateDeal && cfg.DealPipelineID != nil && cfg.DealStageID != nil && !hasOpenDeal {
		create := &models.CreateDeal{PipelineID: *cfg.DealPipelineID, StageID: *cfg.DealStageID, ContactID: &contactID, Name: name}
		if cid, err := uuid.Parse(str(p, "campaign_id")); err == nil {
			create.CampaignID = &cid
		}
		if acct, err := uuid.Parse(str(p, "email_account_id")); err == nil {
			create.SourceMailboxID = &acct
			if uid, _ := s.d.Repo.MailboxUser(ctx, o.ID, acct); uid != nil {
				create.AssignedTo = uid
			}
		}
		deal, err := s.d.CRM.CreateDeal(ctx, o.ID, create)
		if err != nil {
			return err
		}
		if xerr := s.PushDealCreate(ctx, o.ID, deal); xerr != nil {
			_ = s.d.CRM.DeleteDeal(ctx, o.ID, deal.ID)
			return xerr
		}
	}
	return s.afterOutcome(ctx, o, contactID)
}

func (s *Service) afterOutcome(ctx context.Context, o *org, contactID uuid.UUID) error {
	if rec, _ := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider); rec != nil {
		_ = s.refreshContact(ctx, o, contactID, rec.ExternalID, false)
	}
	return nil
}

// logEvent records a bounce, unsubscribe, open or click on a person Pipedrive
// already has. Nobody is created in Pipedrive for an open.
func (s *Service) logEvent(ctx context.Context, o *org, job *models.CRMSyncJob) error {
	p := job.Payload
	contactID, ext, rec, err := s.jobContact(ctx, o, p, false)
	if err != nil || ext == "" {
		return err
	}
	at := time.Now()
	if t := parseTime(str(p, "at")); t != nil {
		at = *t
	}
	values := map[string]string{}
	switch str(p, "event") {
	case "bounce":
		values[fieldStatus] = statusBounced
		if taskID, perr := uuid.Parse(str(p, "task_id")); perr == nil && o.Config.Activity.Bounces {
			if l, _ := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectEmail, taskID); l != nil {
				subject, _ := l.Meta["subject"].(string)
				_, _ = o.Client.UpdateActivity(ctx, extID(l.ExternalID), map[string]any{
					"subject": truncateRunes("Bounced: "+firstNonEmpty(subject, "email"), 250)})
			}
		}
	case "unsubscribe":
		values[fieldStatus] = statusUnsubscribed
		if o.Config.Activity.Unsubscribes {
			note := map[string]any{"content": "Unsubscribed from Warmbly outreach.", "person_id": extID(ext)}
			if rec != nil && rec.CompanyExternalID != "" {
				note["org_id"] = extID(rec.CompanyExternalID)
			}
			if err := s.createOnce(ctx, o, job, models.CRMObjectEmail, func() (int64, error) {
				n, err := o.Client.CreateNote(ctx, note)
				return n.ID, err
			}); err != nil {
				return err
			}
		}
	case "open":
		values[fieldLastOpened] = pdDate(at)
	case "click":
		values[fieldLastClicked] = pdDate(at)
	}
	custom := s.warmblyValues(ctx, o, values)
	if len(custom) == 0 {
		return nil
	}
	if _, err := o.Client.UpdatePerson(ctx, extID(ext), map[string]any{"custom_fields": custom}); err != nil {
		return err
	}
	s.rememberStatus(ctx, o, contactID, values[fieldStatus])
	return nil
}

// logMeeting records a Calendly or Cal.com booking as a Meeting activity.
func (s *Service) logMeeting(ctx context.Context, o *org, job *models.CRMSyncJob) error {
	p := job.Payload
	contactID, ext, rec, err := s.jobContact(ctx, o, p, true)
	if err != nil || ext == "" {
		return err
	}
	start := parseTime(str(p, "scheduled_for"))
	if start == nil {
		start = ptrNow()
	}
	body := map[string]any{
		"subject":      truncateRunes(firstNonEmpty(str(p, "event_name"), "Meeting"), 250),
		"type":         "meeting",
		"done":         false,
		"due_date":     pdDate(*start),
		"due_time":     pdClock(*start),
		"duration":     "00:30",
		"note":         html.EscapeString("Booked through " + firstNonEmpty(str(p, "source"), "a scheduling link") + "."),
		"participants": []Participant{{PersonID: extID(ext), Primary: true}},
	}
	if rec != nil {
		if rec.OwnerExternalID != "" {
			body["owner_id"] = extID(rec.OwnerExternalID)
		}
		if rec.CompanyExternalID != "" {
			body["org_id"] = extID(rec.CompanyExternalID)
		}
	}
	if deal := s.openDeal(ctx, o, contactID); deal != 0 {
		body["deal_id"] = deal
	}
	if err := s.createOnce(ctx, o, job, models.CRMObjectMeeting, func() (int64, error) {
		a, err := o.Client.CreateActivity(ctx, body)
		return a.ID, err
	}); err != nil {
		return err
	}
	if custom := s.warmblyValues(ctx, o, map[string]string{fieldStatus: statusMeeting}); len(custom) > 0 {
		if _, err := o.Client.UpdatePerson(ctx, extID(ext), map[string]any{"custom_fields": custom}); err != nil {
			return err
		}
		s.rememberStatus(ctx, o, contactID, statusMeeting)
	}
	return nil
}

// createOnce creates a Pipedrive record for a job at most once: the record is
// linked to the job before any later step can fail and retry it.
func (s *Service) createOnce(ctx context.Context, o *org, job *models.CRMSyncJob, objectType string, create func() (int64, error)) error {
	if done, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, objectType, job.ID); err != nil || done != nil {
		return err
	}
	created, err := create()
	if err != nil {
		return err
	}
	return s.d.Repo.UpsertLink(ctx, &models.CRMExternalLink{OrganizationID: o.ID, Provider: provider,
		ObjectType: objectType, LocalID: job.ID, ExternalID: id(created)})
}

// pushContactFields writes a Warmbly contact's two-way and push fields to its
// linked person. Contacts not in Pipedrive are left alone.
func (s *Service) pushContactFields(ctx context.Context, o *org, p map[string]any) error {
	contactID, err := uuid.Parse(str(p, "contact_id"))
	if err != nil {
		return nil
	}
	rec, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
	if err != nil || rec == nil {
		return err
	}
	contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID})
	if xerr != nil {
		return xerr
	}
	if len(contacts) != 1 {
		return nil
	}
	body := s.personBody(ctx, o, &contacts[0], false)
	if len(body) == 0 {
		return nil
	}
	_, err = o.Client.UpdatePerson(ctx, extID(rec.ExternalID), body)
	return err
}
