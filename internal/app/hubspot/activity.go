package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// OnEvent is the platform event sink: it turns sends, replies, bounces,
// unsubscribes, opens, clicks and bookings into outbox jobs. It runs before
// the webhook throttle, so HubSpot sees every send even when a burst is too
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
		// One property write per contact per hour is plenty for "last opened".
		job.DedupeKey = fmt.Sprintf("%s:%s:%d", job.Payload["event"], email, time.Now().Unix()/3600)
	case models.WebhookEventContactUpdated:
		// A Warmbly edit is the most recent change: two-way and push fields go
		// to HubSpot. HubSpot's own edits arrive through the pull, which writes
		// the contact without emitting this event, so nothing loops.
		if str(m, "entity_type") != string(models.AuditEntityContact) {
			return
		}
		if md, ok := m["metadata"].(map[string]string); ok && md["crm"] != "" {
			return
		}
		id := str(m, "entity_id")
		if _, err := uuid.Parse(id); err != nil {
			return
		}
		job.Kind = models.CRMJobPushContact
		job.Payload = map[string]any{"contact_id": id}
		job.Subject = "Contact " + id[:8]
		job.DedupeKey = "contact:" + id
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
		log.Warn().Err(err).Str("org_id", orgID.String()).Str("kind", job.Kind).Msg("hubspot: could not queue activity")
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

// jobContact resolves the Warmbly contact a job is about. create allows a
// HubSpot contact to be created for them (sends and replies, never opens).
func (s *Service) jobContact(ctx context.Context, o *org, p map[string]any, create bool) (uuid.UUID, string, *models.CRMContactRecord, error) {
	var contactID uuid.UUID
	if id, err := uuid.Parse(str(p, "contact_id")); err == nil {
		contactID = id
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
	ext, rec, err := s.ensureContact(ctx, o, contactID)
	return contactID, ext, rec, err
}

// logEmail writes a send or a reply as a HubSpot email activity, attached to
// the contact, its company and its open deals, and updates the Warmbly
// properties.
//
// A retry never logs the same email twice: the logged email is linked to the
// send's task (or, for a reply, to this job) before anything else can fail.
func (s *Service) logEmail(ctx context.Context, o *org, job *models.CRMSyncJob) error {
	p := job.Payload
	if err := s.propertiesReady(ctx, o); err != nil {
		return err
	}
	contactID, ext, rec, err := s.jobContact(ctx, o, p, true)
	if err != nil || ext == "" {
		return err
	}
	logKey := job.ID
	if taskID, perr := uuid.Parse(str(p, "task_id")); perr == nil && str(p, "direction") != "in" {
		logKey = taskID
	}
	inbound := str(p, "direction") == "in"
	at := parseHSTime(str(p, "at"))
	if at == nil {
		now := time.Now()
		at = &now
	}
	owner := ""
	if acct, err := uuid.Parse(str(p, "email_account_id")); err == nil {
		if uid, _ := s.d.Repo.MailboxUser(ctx, o.ID, acct); uid != nil {
			owner = s.ownerFor(ctx, o, uid)
		}
	}
	if owner == "" && rec != nil {
		owner = rec.OwnerExternalID
	}

	logIt := (inbound && o.Config.Activity.Replies) || (!inbound && o.Config.Activity.Sent)
	if logIt {
		if done, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectEmail, logKey); err != nil {
			return err
		} else if done != nil {
			logIt = false
		}
	}
	if logIt {
		contactEmail := str(p, "contact_email")
		from, to := str(p, "from_email"), contactEmail
		direction := "EMAIL"
		if inbound {
			direction = "INCOMING_EMAIL"
			from, to = contactEmail, str(p, "to_email")
		}
		headers, _ := json.Marshal(map[string]any{
			"from": map[string]string{"email": from},
			"to":   []map[string]string{{"email": to}},
			"cc":   []any{},
			"bcc":  []any{},
		})
		props := map[string]string{
			"hs_timestamp":       hsTime(*at),
			"hs_email_direction": direction,
			"hs_email_status":    "SENT",
			"hs_email_subject":   firstNonEmpty(str(p, "subject"), "(no subject)"),
			"hs_email_text":      truncateRunes(str(p, "body"), 60000),
			"hs_email_headers":   string(headers),
		}
		if owner != "" {
			props["hubspot_owner_id"] = owner
		}
		assocs := []Assoc{{ToID: ext, TypeID: assocEmailToContact}}
		if rec != nil && rec.CompanyExternalID != "" {
			assocs = append(assocs, Assoc{ToID: rec.CompanyExternalID, TypeID: assocEmailToCompany})
		}
		assocs = append(assocs, s.openDealAssocs(ctx, o, contactID, assocEmailToDeal)...)
		obj, err := o.Client.Create(ctx, "emails", props, assocs)
		if err != nil {
			return err
		}
		if err := s.d.Repo.UpsertLink(ctx, &models.CRMExternalLink{OrganizationID: o.ID, Provider: provider,
			ObjectType: models.CRMObjectEmail, LocalID: logKey, ExternalID: obj.ID}); err != nil {
			return err
		}
	}

	if o.Config.WriteProperties {
		props := map[string]string{}
		if s.d.AppURL != "" {
			props[propLink] = s.contactLink(contactID)
		}
		if inbound {
			props[propLastReplied] = hsTime(*at)
			intent := str(p, "intent")
			props[propReplyIntent] = intent
			props[propStatus] = statusReplied
			switch models.ReplyIntentType(intent) {
			case models.ReplyIntentPositive:
				props[propStatus] = statusInterested
			case models.ReplyIntentNegative:
				props[propStatus] = statusNotInterest
			}
		} else {
			props[propLastContacted] = hsTime(*at)
			if cid, err := uuid.Parse(str(p, "campaign_id")); err == nil {
				if name, _ := s.d.Repo.CampaignName(ctx, o.ID, cid); name != "" {
					props[propLastCampaign] = name
				}
			}
			if cur := currentStatus(rec); cur == "" || cur == statusContacted {
				props[propStatus] = statusContacted
			}
		}
		if !inbound {
			if contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID}); xerr == nil && len(contacts) == 1 {
				for k, v := range pushOnlyProperties(o, &contacts[0]) {
					props[k] = v
				}
			}
		}
		if _, err := o.Client.Update(ctx, "contacts", ext, props); err != nil {
			return err
		}
		s.rememberStatus(ctx, o, contactID, props[propStatus])
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
	return rec.Properties[propStatus]
}

// rememberStatus keeps the last written warmbly_status on the record, so a
// follow-up send never turns "interested" back into "contacted".
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
	rec.Properties[propStatus] = status
	_ = s.d.Repo.UpsertContactRecord(ctx, rec)
}

// pushOnlyProperties are the mapped fields Warmbly owns outright.
func pushOnlyProperties(o *org, c *models.Contact) map[string]string {
	all := pushProperties(o, c, false)
	for field, prop := range o.Config.FieldMap {
		if o.Config.FieldDirection[field] != models.CRMFieldPush {
			delete(all, prop)
		}
	}
	return all
}

func (s *Service) openDealAssocs(ctx context.Context, o *org, contactID uuid.UUID, typeID int) []Assoc {
	deals, err := s.d.CRM.GetDealsByContact(ctx, o.ID, contactID)
	if err != nil || len(deals) == 0 {
		return nil
	}
	var ids []uuid.UUID
	for _, d := range deals {
		if d.Status == models.DealStatusOpen {
			ids = append(ids, d.ID)
		}
	}
	links, err := s.d.Repo.LinksForLocal(ctx, o.ID, provider, models.CRMObjectDeal, ids)
	if err != nil {
		return nil
	}
	out := make([]Assoc, 0, len(links))
	for _, l := range links {
		out = append(out, Assoc{ToID: l.ExternalID, TypeID: typeID})
	}
	return out
}

// replyOutcomeJob runs the interested-reply outcome for a contact in HubSpot.
func (s *Service) replyOutcomeJob(ctx context.Context, o *org, p map[string]any) error {
	contactID, ext, _, err := s.jobContact(ctx, o, p, false)
	if err != nil || ext == "" {
		return err
	}
	return s.replyOutcome(ctx, o, contactID, ext, p)
}

// replyOutcome is what an interested reply does in HubSpot: lead status,
// lifecycle stage, and optionally a deal.
func (s *Service) replyOutcome(ctx context.Context, o *org, contactID uuid.UUID, ext string, p map[string]any) error {
	cfg := o.Config.PositiveReply
	props := map[string]string{}
	if cfg.LeadStatus != "" {
		props["hs_lead_status"] = cfg.LeadStatus
	}
	if cfg.LifecycleStage != "" {
		props["lifecyclestage"] = cfg.LifecycleStage
	}
	if len(props) > 0 {
		if _, err := o.Client.Update(ctx, "contacts", ext, props); err != nil {
			// HubSpot refuses moving a lifecycle stage backwards; the lead
			// status still matters, so retry without it.
			if ae, ok := AsAPIError(err); ok && !ae.Retryable() && cfg.LifecycleStage != "" && cfg.LeadStatus != "" {
				delete(props, "lifecyclestage")
				if _, err := o.Client.Update(ctx, "contacts", ext, props); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	if cfg.CreateDeal && cfg.DealPipelineID != nil && cfg.DealStageID != nil {
		deals, err := s.d.CRM.GetDealsByContact(ctx, o.ID, contactID)
		if err != nil {
			return err
		}
		for _, d := range deals {
			if d.Status == models.DealStatusOpen {
				return s.afterOutcome(ctx, o, contactID)
			}
		}
		contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID})
		if xerr != nil || len(contacts) == 0 {
			return nil
		}
		c := contacts[0]
		name := strings.TrimSpace(firstNonEmpty(c.Company, strings.TrimSpace(c.FirstName+" "+c.LastName), c.Email))
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
		_ = s.refreshContact(ctx, o, contactID, rec.ExternalID)
	}
	return nil
}

// logEvent records a bounce, unsubscribe, open or click on a contact HubSpot
// already has. Nobody is created in HubSpot for an open.
func (s *Service) logEvent(ctx context.Context, o *org, p map[string]any) error {
	if err := s.propertiesReady(ctx, o); err != nil {
		return err
	}
	contactID, ext, _, err := s.jobContact(ctx, o, p, false)
	if err != nil || ext == "" {
		return err
	}
	props := map[string]string{}
	at := hsTime(time.Now())
	if t := parseHSTime(str(p, "at")); t != nil {
		at = hsTime(*t)
	}
	switch str(p, "event") {
	case "bounce":
		props[propStatus] = statusBounced
		if taskID, perr := uuid.Parse(str(p, "task_id")); perr == nil {
			if l, _ := s.d.Repo.GetLinkByLocal(ctx, o.ID, provider, models.CRMObjectEmail, taskID); l != nil && o.Config.Activity.Bounces {
				_, _ = o.Client.Update(ctx, "emails", l.ExternalID, map[string]string{"hs_email_status": "BOUNCED"})
			}
		}
	case "unsubscribe":
		props[propStatus] = statusUnsubscribed
		props[propUnsubscribed] = "true"
		if o.Config.Activity.Unsubscribes {
			note := map[string]string{"hs_note_body": "Unsubscribed from Warmbly outreach.", "hs_timestamp": at}
			if _, err := o.Client.Create(ctx, "notes", note, []Assoc{{ToID: ext, TypeID: assocNoteToContact}}); err != nil {
				return err
			}
		}
	case "open":
		props[propLastOpened] = at
	case "click":
		props[propLastClicked] = at
	}
	if !o.Config.WriteProperties {
		delete(props, propStatus)
		delete(props, propUnsubscribed)
		delete(props, propLastOpened)
		delete(props, propLastClicked)
	}
	if len(props) == 0 {
		return nil
	}
	if _, err := o.Client.Update(ctx, "contacts", ext, props); err != nil {
		return err
	}
	s.rememberStatus(ctx, o, contactID, props[propStatus])
	return nil
}

// logMeeting records a Calendly or Cal.com booking as a HubSpot meeting.
func (s *Service) logMeeting(ctx context.Context, o *org, p map[string]any) error {
	if err := s.propertiesReady(ctx, o); err != nil {
		return err
	}
	contactID, ext, rec, err := s.jobContact(ctx, o, p, true)
	if err != nil || ext == "" {
		return err
	}
	start := parseHSTime(str(p, "scheduled_for"))
	if start == nil {
		now := time.Now()
		start = &now
	}
	end := start.Add(30 * time.Minute)
	props := map[string]string{
		"hs_timestamp":          hsTime(*start),
		"hs_meeting_title":      firstNonEmpty(str(p, "event_name"), "Meeting"),
		"hs_meeting_start_time": hsTime(*start),
		"hs_meeting_end_time":   hsTime(end),
		"hs_meeting_outcome":    "SCHEDULED",
		"hs_meeting_body":       "Booked through " + firstNonEmpty(str(p, "source"), "a scheduling link") + ".",
	}
	if rec != nil && rec.OwnerExternalID != "" {
		props["hubspot_owner_id"] = rec.OwnerExternalID
	}
	assocs := append([]Assoc{{ToID: ext, TypeID: assocMeetingToContact}}, s.openDealAssocs(ctx, o, contactID, assocMeetingToDeal)...)
	if _, err := o.Client.Create(ctx, "meetings", props, assocs); err != nil {
		return err
	}
	if o.Config.WriteProperties {
		if _, err := o.Client.Update(ctx, "contacts", ext, map[string]string{propStatus: statusMeeting}); err != nil {
			return err
		}
		s.rememberStatus(ctx, o, contactID, statusMeeting)
	}
	return nil
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// pushContactFields writes a Warmbly contact's two-way and push fields to its
// linked HubSpot record. Contacts not in HubSpot are left alone.
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
	all := pushProperties(o, &contacts[0], true)
	props := map[string]string{}
	for field, prop := range o.Config.FieldMap {
		if v, ok := all[prop]; ok && o.Config.FieldDirection[field] != models.CRMFieldPull {
			props[prop] = v
		}
	}
	if len(props) == 0 {
		return nil
	}
	_, err = o.Client.Update(ctx, "contacts", rec.ExternalID, props)
	return err
}
