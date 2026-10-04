package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

const (
	drainBatch     = 500
	drainLease     = 10 * time.Minute
	maxAttempts    = 6
	descriptionCap = 32000
)

// Drain logs due outbox activity in Salesforce. Run on a short interval; each
// pass leases what it takes, so replicas never log the same event twice.
func (s *Service) Drain(ctx context.Context) error {
	items, err := s.Repo.ClaimDueActivities(ctx, drainBatch, drainLease)
	if err != nil || len(items) == 0 {
		return err
	}
	byConn := map[uuid.UUID][]models.SalesforceActivity{}
	orgOf := map[uuid.UUID]uuid.UUID{}
	for _, a := range items {
		byConn[a.ConnectionID] = append(byConn[a.ConnectionID], a)
		orgOf[a.ConnectionID] = a.OrganizationID
	}
	for connID, list := range byConn {
		s.drainConnection(ctx, orgOf[connID], connID, list)
	}
	return nil
}

// DrainConnection logs one connection's due activity now ("Sync now").
func (s *Service) drainConnection(ctx context.Context, orgID, connID uuid.UUID, items []models.SalesforceActivity) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		s.deferAll(ctx, items, 15*time.Minute, "Salesforce connection not available right now")
		return
	}
	defer s.settle(ctx, c)
	if !c.settings.Enabled {
		s.finishAll(ctx, items, models.SalesforceActivitySkipped, "Salesforce sync is turned off")
		return
	}
	if c.Status == models.IntegrationStatusReauthRequired {
		s.deferAll(ctx, items, time.Hour, "Waiting for Salesforce to be reconnected")
		return
	}
	if s.overBudget(ctx, c) {
		s.deferAll(ctx, items, untilTomorrow(), "Daily Salesforce API budget reached; resumes tomorrow")
		return
	}
	if err := s.logActivities(ctx, c, items); err != nil {
		log.Warn().Err(err).Str("connection", connID.String()).Msg("salesforce: activity batch failed")
		s.retryAll(ctx, items, err)
	}
}

// pending is one activity on its way to Salesforce.
type pending struct {
	a       models.SalesforceActivity
	sent    *repository.SalesforceSentContent
	contact repository.SalesforceContact
	link    *models.SalesforceRecordLink
	task    map[string]any
	done    bool
}

func (s *Service) logActivities(ctx context.Context, c *conn, items []models.SalesforceActivity) error {
	// Resolve every activity to a Warmbly contact in the organization.
	var ids []uuid.UUID
	var emails []string
	for _, a := range items {
		if a.ContactID != nil {
			ids = append(ids, *a.ContactID)
		} else {
			emails = append(emails, a.ContactEmail)
		}
	}
	byID, err := s.Repo.ContactsByIDs(ctx, c.OrganizationID, ids)
	if err != nil {
		return err
	}
	byEmail, err := s.Repo.ContactsByEmails(ctx, c.OrganizationID, emails)
	if err != nil {
		return err
	}
	work := make([]*pending, 0, len(items))
	for _, a := range items {
		p := &pending{a: a}
		if a.ContactID != nil {
			p.contact = byID[*a.ContactID]
		}
		if p.contact.ID == uuid.Nil {
			p.contact = byEmail[strings.ToLower(a.ContactEmail)]
		}
		if p.contact.ID == uuid.Nil {
			s.finish(ctx, p, models.SalesforceActivitySkipped, "", "Not a contact in this workspace")
			continue
		}
		if a.Kind == KindSent && !s.sendWentOut(ctx, p) {
			continue
		}
		work = append(work, p)
	}
	if len(work) == 0 {
		return nil
	}

	// Link: existing links, then a match, then creation when the rule allows.
	var contacts []repository.SalesforceContact
	seen := map[uuid.UUID]bool{}
	createFor := map[uuid.UUID]bool{}
	sender := map[uuid.UUID]string{}
	for _, p := range work {
		if !seen[p.contact.ID] {
			seen[p.contact.ID] = true
			contacts = append(contacts, p.contact)
		}
		if mayCreate(c.settings, p.a.Kind) {
			createFor[p.contact.ID] = true
		}
		if e := s.senderEmail(ctx, c.OrganizationID, p.a); e != "" {
			sender[p.contact.ID] = e
		}
	}
	links, failed := s.ensureLinks(ctx, c, contacts, "", "", nil)
	// People still missing are created in groups that share an owner rule input.
	bySender := map[string][]repository.SalesforceContact{}
	for _, ct := range contacts {
		if _, ok := links[ct.ID]; !ok && failed[ct.ID] == nil && createFor[ct.ID] {
			bySender[sender[ct.ID]] = append(bySender[sender[ct.ID]], ct)
		}
	}
	for from, group := range bySender {
		created, cerr := s.ensureLinks(ctx, c, group, c.settings.Matching.CreateAs, from, nil)
		for id, e := range cerr {
			failed[id] = e
		}
		for id, l := range created {
			links[id] = l
		}
	}
	for _, p := range work {
		if e := failed[p.contact.ID]; e != nil {
			s.fail(ctx, p, e)
			p.done = true
			continue
		}
		l, ok := links[p.contact.ID]
		if !ok {
			why := "No Lead or Contact in Salesforce for this address"
			if c.settings.Matching.CreateWhen == "reply" && !mayCreate(c.settings, p.a.Kind) {
				why += "; one is created when they reply"
			}
			s.finish(ctx, p, models.SalesforceActivitySkipped, "", why)
			p.done = true
			continue
		}
		lc := l
		p.link = &lc
	}

	// Tasks. Rows are already settled above, so a failure here settles the rest
	// rather than handing the whole batch back.
	if err := s.buildTasks(ctx, c, work); err != nil {
		for _, p := range work {
			if !p.done {
				s.fail(ctx, p, err)
				p.done = true
			}
		}
		return nil
	}
	var withTask []*pending
	for _, p := range work {
		if !p.done && p.task != nil {
			withTask = append(withTask, p)
		}
	}
	if len(withTask) > 0 {
		s.createTasks(ctx, c, withTask, true)
	}

	// Writeback: status, opt-out and pushed fields, one update per record.
	s.writeback(ctx, c, work, links)

	for _, p := range work {
		if p.done {
			continue
		}
		detail := ""
		if p.task == nil {
			detail = "Fields updated; no Task for this kind of activity"
		}
		s.finish(ctx, p, models.SalesforceActivitySynced, "", detail)
	}
	return nil
}

// sendWentOut settles a send event whose email never left: the dispatch is
// recorded before the worker answers, and a failed send retries under a new
// task. A send still in flight waits a little.
func (s *Service) sendWentOut(ctx context.Context, p *pending) bool {
	id, err := uuid.Parse(str(p.a.Payload, "task_id"))
	if err != nil {
		return true
	}
	sent, err := s.Repo.SentContent(ctx, p.a.OrganizationID, id)
	if err != nil || sent == nil {
		return true
	}
	p.sent = sent
	switch sent.Status {
	case "completed":
		return true
	case "pending", "active":
		if time.Since(p.a.OccurredAt) > 24*time.Hour {
			s.finish(ctx, p, models.SalesforceActivitySkipped, "", "The send never completed")
			return false
		}
		p.a.Status = models.SalesforceActivityPending
		p.a.NextAttemptAt = time.Now().UTC().Add(3 * time.Minute)
		p.a.Detail = "Waiting for the send to complete"
		_ = s.Repo.FinishActivity(ctx, &p.a)
		return false
	default:
		s.finish(ctx, p, models.SalesforceActivitySkipped, "", "The email was not sent")
		return false
	}
}

// mayCreate reports whether an activity of this kind may create its person.
// Bounces, opt-outs and opens never create anyone.
func mayCreate(st Settings, kind string) bool {
	switch st.Matching.CreateWhen {
	case "send":
		return kind == KindSent || kind == KindReplied || kind == KindMeetingBooked
	case "reply":
		return kind == KindReplied || kind == KindMeetingBooked
	}
	return false
}

func (s *Service) senderEmail(ctx context.Context, orgID uuid.UUID, a models.SalesforceActivity) string {
	if e := str(a.Payload, "from_email"); e != "" {
		return e
	}
	for _, k := range []string{"email_account_id", "sender_email_account_id"} {
		if id, err := uuid.Parse(str(a.Payload, k)); err == nil {
			if e, err := s.Repo.MailboxEmail(ctx, orgID, id); err == nil && e != "" {
				return e
			}
		}
	}
	return ""
}

// buildTasks writes the Task for every activity whose kind is logged.
func (s *Service) buildTasks(ctx context.Context, c *conn, work []*pending) error {
	meta, err := s.describe(ctx, c)
	if err != nil {
		return err
	}
	opps := s.openOpportunities(ctx, c, work)
	for _, p := range work {
		if p.done || p.link == nil || !c.settings.Activity.Logs(p.a.Kind) {
			continue
		}
		subject, body := s.taskText(ctx, c, p.a, p.sent)
		t := map[string]any{
			"Subject":      truncate(subject, 255),
			"Status":       meta.closedTask,
			"ActivityDate": p.a.OccurredAt.UTC().Format("2006-01-02"),
			"WhoId":        p.link.RecordID,
		}
		if isEmailKind(p.a.Kind) {
			t["TaskSubtype"] = "Email"
		}
		if body != "" {
			t["Description"] = truncate(body, descriptionCap)
		}
		if p.link.SObject == ObjectContact && c.settings.Activity.RelateToOpportunity {
			if opp := opps[p.link.AccountID]; opp != "" {
				t["WhatId"] = opp
			}
		}
		if owner := s.taskOwner(ctx, c, p); owner != "" {
			t["OwnerId"] = owner
		}
		p.task = t
	}
	return nil
}

func isEmailKind(kind string) bool {
	switch kind {
	case KindSent, KindReplied, KindOpened, KindClicked, KindBounced:
		return true
	}
	return false
}

// taskOwner picks who owns the logged Task. A queue cannot own a Task, so a
// Lead sitting in a queue falls back to the connected user.
func (s *Service) taskOwner(ctx context.Context, c *conn, p *pending) string {
	switch c.settings.Activity.AssignTo {
	case "record_owner":
		if strings.HasPrefix(p.link.OwnerID, "005") {
			return p.link.OwnerID
		}
	case "sender":
		if id := s.userByEmail(ctx, c, s.senderEmail(ctx, c.OrganizationID, p.a)); id != "" {
			return id
		}
	}
	return c.sfUserID()
}

// openOpportunities maps each Contact's account to its most recently touched
// open opportunity.
func (s *Service) openOpportunities(ctx context.Context, c *conn, work []*pending) map[string]string {
	out := map[string]string{}
	if !c.settings.Activity.RelateToOpportunity {
		return out
	}
	var accounts []string
	seen := map[string]bool{}
	for _, p := range work {
		if p.link != nil && p.link.SObject == ObjectContact && p.link.AccountID != "" && !seen[p.link.AccountID] {
			seen[p.link.AccountID] = true
			accounts = append(accounts, p.link.AccountID)
		}
	}
	for start := 0; start < len(accounts); start += 200 {
		chunk := accounts[start:min(start+200, len(accounts))]
		rows, err := c.client.QueryAll(ctx, "SELECT Id, AccountId FROM Opportunity WHERE IsClosed = false AND AccountId IN "+QuoteList(chunk)+" ORDER BY LastModifiedDate DESC", 0)
		if err != nil {
			return out
		}
		for _, r := range rows {
			acc := NormalizeID(r.String("AccountId"))
			if _, ok := out[acc]; !ok {
				out[acc] = NormalizeID(r.String("Id"))
			}
		}
	}
	return out
}

// taskText renders a Task's subject and description the way a rep would have
// written them.
func (s *Service) taskText(ctx context.Context, c *conn, a models.SalesforceActivity, sent *repository.SalesforceSentContent) (string, string) {
	content := s.openContent(ctx, a)
	var lines []string
	meta := func(label, v string) {
		if v != "" {
			lines = append(lines, label+": "+v)
		}
	}
	subject := content.Subject
	body := ""
	switch a.Kind {
	case KindSent:
		body = htmlToText(content.Body)
		if sent != nil {
			if subject == "" {
				subject = sent.Subject
			}
			meta("Campaign", sent.Campaign)
			meta("Step", sent.Step)
			meta("From", sent.FromEmail)
		}
		meta("To", a.ContactEmail)
		subject = "Email sent: " + orDefault(subject, "(no subject)")
	case KindReplied:
		meta("From", a.ContactEmail)
		meta("Intent", humanIntent(str(a.Payload, "intent")))
		body = content.Body
		subject = "Reply received: " + orDefault(subject, "(no subject)")
	case KindOpened:
		subject = "Email opened"
		meta("By", a.ContactEmail)
	case KindClicked:
		link := orDefault(str(a.Payload, "link_label"), str(a.Payload, "url"))
		subject = "Link clicked: " + orDefault(link, "a tracked link")
		meta("URL", str(a.Payload, "url"))
	case KindBounced:
		subject = "Email bounced"
		meta("Reason", str(a.Payload, "reason"))
	case KindUnsubscribed:
		subject = "Unsubscribed from Warmbly outreach"
		if str(a.Payload, "source") == "complaint" {
			subject = "Marked Warmbly outreach as spam"
		}
	case KindMeetingBooked:
		subject = "Meeting booked: " + orDefault(str(a.Payload, "event_name"), "meeting")
		meta("When", str(a.Payload, "scheduled_for"))
		meta("Join", str(a.Payload, "join_url"))
	}
	desc := strings.Join(lines, "\n")
	if c.settings.Activity.IncludeBody && strings.TrimSpace(body) != "" {
		if desc != "" {
			desc += "\n\n"
		}
		desc += strings.TrimSpace(body)
	}
	if desc != "" {
		desc += "\n\n"
	}
	desc += "Logged by Warmbly"
	return subject, desc
}

func (s *Service) openContent(ctx context.Context, a models.SalesforceActivity) activityContent {
	var out activityContent
	if a.ContentEncrypted == "" || s.Cipher == nil {
		return out
	}
	ci, err := s.Cipher.Cipher(ctx, a.OrganizationID)
	if err != nil {
		return out
	}
	plain, err := ci.Decrypt(ctx, a.ContentEncrypted)
	if err != nil {
		return out
	}
	_ = json.Unmarshal([]byte(plain), &out)
	return out
}

var (
	reBlockTags = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/tr|/h[1-6])\s*/?>`)
	reTags      = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlank     = regexp.MustCompile(`\n{3,}`)
	reStyle     = regexp.MustCompile(`(?is)<(style|script)[^>]*>.*?</(style|script)>`)
)

// htmlToText is a plain rendering of an email body for a Task description.
func htmlToText(s string) string {
	if !strings.Contains(s, "<") {
		return strings.TrimSpace(s)
	}
	s = reStyle.ReplaceAllString(s, "")
	s = reBlockTags.ReplaceAllString(s, "\n")
	s = reTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = reBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func humanIntent(v string) string {
	switch v {
	case "positive":
		return "Interested"
	case "negative":
		return "Not interested"
	case "out_of_office":
		return "Out of office"
	case "question":
		return "Question"
	case "neutral":
		return "Neutral"
	case "automated":
		return "Automated reply"
	}
	return v
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

// createTasks inserts the Tasks. A Task refused over its owner (an inactive
// user, a queue) is retried once owned by the connected user.
func (s *Service) createTasks(ctx context.Context, c *conn, work []*pending, retryOwner bool) {
	records := make([]map[string]any, len(work))
	for i, p := range work {
		records[i] = p.task
	}
	res, err := c.client.Create(ctx, "Task", records)
	if err != nil {
		for _, p := range work {
			s.fail(ctx, p, err)
			p.done = true
		}
		return
	}
	var again []*pending
	for i, p := range work {
		if i >= len(res) {
			s.fail(ctx, p, errors.New("salesforce returned no result for this Task"))
			p.done = true
			continue
		}
		if e := res[i].Err(); e != nil {
			var ae *APIError
			if retryOwner && errors.As(e, &ae) && ownerProblem(ae) {
				delete(p.task, "OwnerId")
				again = append(again, p)
				continue
			}
			if errors.As(e, &ae) && taskSubtypeProblem(ae) {
				delete(p.task, "TaskSubtype")
				again = append(again, p)
				continue
			}
			s.fail(ctx, p, e)
			p.done = true
			continue
		}
		s.finish(ctx, p, models.SalesforceActivitySynced, res[i].ID, "")
		p.done = true
	}
	if len(again) > 0 {
		s.createTasks(ctx, c, again, false)
	}
}

func ownerProblem(e *APIError) bool {
	for _, f := range e.Fields {
		if strings.EqualFold(f, "OwnerId") {
			return true
		}
	}
	return e.Code == "INACTIVE_OWNER_OR_USER" || e.Code == "INVALID_CROSS_REFERENCE_KEY" && strings.Contains(e.Message, "owner")
}

func taskSubtypeProblem(e *APIError) bool {
	for _, f := range e.Fields {
		if strings.EqualFold(f, "TaskSubtype") {
			return true
		}
	}
	return false
}

// writeback applies status, opt-out and pushed-field changes for the batch.
func (s *Service) writeback(ctx context.Context, c *conn, work []*pending, links map[uuid.UUID]models.SalesforceRecordLink) {
	meta, _ := s.describe(ctx, c)
	var leadDescribe *Describe
	if meta != nil {
		leadDescribe = meta.lead
	}
	changes := map[uuid.UUID]map[string]any{}
	var ids []uuid.UUID
	contactByID := map[uuid.UUID]repository.SalesforceContact{}
	for _, p := range work {
		if p.link == nil {
			continue
		}
		cid := p.contact.ID
		if _, ok := changes[cid]; !ok {
			changes[cid] = map[string]any{}
			ids = append(ids, cid)
			contactByID[cid] = p.contact
		}
		ch := changes[cid]
		if p.link.SObject == ObjectLead && !p.link.IsConverted {
			target := ""
			w := c.settings.Writeback
			switch p.a.Kind {
			case KindSent:
				target = w.LeadStatusOnSent
			case KindReplied:
				target = w.statusForReply(str(p.a.Payload, "intent"))
			case KindMeetingBooked:
				target = w.LeadStatusOnMeeting
			}
			if target != "" && !strings.EqualFold(target, p.link.LeadStatus) {
				cur := p.link.LeadStatus
				if v, ok := ch["Status"].(string); ok {
					cur = v
				}
				if !w.NeverMoveBackwards || statusRank(leadDescribe, target) >= statusRank(leadDescribe, cur) {
					ch["Status"] = target
				}
			}
		}
		if p.a.Kind == KindUnsubscribed && !p.link.OptedOut &&
			(c.settings.Inbound.OptOut == "both" || c.settings.Inbound.OptOut == "to_salesforce") {
			ch["HasOptedOutOfEmail"] = true
		}
	}
	eng, _ := s.Repo.Engagement(ctx, c.OrganizationID, ids)
	for _, cid := range ids {
		var e *repository.SalesforceEngagement
		if v, ok := eng[cid]; ok {
			e = &v
		}
		for k, v := range pushChanges(c.settings, links[cid], contactByID[cid], e) {
			if _, set := changes[cid][k]; !set {
				changes[cid][k] = v
			}
		}
	}
	errs := s.applyUpdates(ctx, c, changes, links)
	var pushed []uuid.UUID
	for _, cid := range ids {
		if errs[cid] == nil {
			pushed = append(pushed, links[cid].ID)
		}
	}
	_ = s.Repo.MarkLinksPushed(ctx, pushed)
	for _, p := range work {
		if p.link == nil {
			continue
		}
		if e := errs[p.contact.ID]; e != nil {
			if p.done {
				// The Task landed; the record update did not. Say so on the row.
				if p.a.TaskID != "" {
					s.annotate(ctx, p, "Task logged, but updating the record failed: "+describeErr(e))
				}
				continue
			}
			s.fail(ctx, p, e)
			p.done = true
		}
	}
}

// --- outcomes ---------------------------------------------------------------

func (s *Service) finish(ctx context.Context, p *pending, status, taskID, detail string) {
	p.a.Status = status
	p.a.Detail = detail
	if taskID != "" {
		p.a.TaskID = taskID
	}
	if p.link != nil {
		p.a.RecordID = p.link.RecordID
	}
	p.a.Attempts++
	_ = s.Repo.FinishActivity(ctx, &p.a)
	// FinishActivity released the lease; a later note on this row targets it unleased.
	p.a.LeaseID = nil
}

func (s *Service) annotate(ctx context.Context, p *pending, detail string) {
	p.a.Detail = truncate(detail, 1000)
	_ = s.Repo.FinishActivity(ctx, &p.a)
}

// fail retries a transient failure with backoff and gives up on anything
// Salesforce will refuse again unchanged.
func (s *Service) fail(ctx context.Context, p *pending, err error) {
	if errors.Is(err, integration.ErrPushReauth) || errors.Is(err, ErrSessionExpired) {
		// Not the activity's fault: hold it until the connection is fixed.
		p.a.Status = models.SalesforceActivityPending
		p.a.NextAttemptAt = time.Now().UTC().Add(time.Hour)
		p.a.Detail = "Waiting for Salesforce to be reconnected"
		_ = s.Repo.FinishActivity(ctx, &p.a)
		return
	}
	if errors.Is(err, ErrRateLimited) {
		p.a.Status = models.SalesforceActivityPending
		p.a.NextAttemptAt = time.Now().UTC().Add(untilTomorrow())
		p.a.Detail = "Salesforce API limit reached for today; resumes tomorrow"
		_ = s.Repo.FinishActivity(ctx, &p.a)
		return
	}
	p.a.Attempts++
	p.a.Detail = truncate(describeErr(err), 1000)
	if p.link != nil {
		p.a.RecordID = p.link.RecordID
		_ = s.Repo.SetLinkError(ctx, p.link.ID, describeErr(err))
	}
	if Retryable(err) && p.a.Attempts < maxAttempts {
		p.a.Status = models.SalesforceActivityPending
		p.a.NextAttemptAt = time.Now().UTC().Add(backoff(p.a.Attempts))
	} else {
		p.a.Status = models.SalesforceActivityFailed
	}
	_ = s.Repo.FinishActivity(ctx, &p.a)
	p.a.LeaseID = nil
}

func backoff(attempt int) time.Duration {
	d := time.Minute << min(attempt, 8)
	return min(d, 6*time.Hour)
}

func (s *Service) finishAll(ctx context.Context, items []models.SalesforceActivity, status, detail string) {
	for i := range items {
		p := &pending{a: items[i]}
		s.finish(ctx, p, status, "", detail)
	}
}

func (s *Service) deferAll(ctx context.Context, items []models.SalesforceActivity, after time.Duration, detail string) {
	for i := range items {
		a := items[i]
		a.Status = models.SalesforceActivityPending
		a.NextAttemptAt = time.Now().UTC().Add(after)
		a.Detail = detail
		_ = s.Repo.FinishActivity(ctx, &a)
	}
}

func (s *Service) retryAll(ctx context.Context, items []models.SalesforceActivity, err error) {
	for i := range items {
		p := &pending{a: items[i]}
		s.fail(ctx, p, err)
	}
}

func untilTomorrow() time.Duration {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, time.UTC)
	return next.Sub(now)
}

// SyncNow logs a connection's pending activity and pulls its changes at once.
// One run per connection at a time; a second click while it runs is a no-op.
func (s *Service) SyncNow(ctx context.Context, orgID, connID uuid.UUID) error {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return err
	}
	if !c.settings.Enabled {
		return errx.NewWithIdentifier(errx.BadRequest, "salesforce_sync_off", "Turn Salesforce sync on first.")
	}
	if _, running := s.syncing.LoadOrStore(connID, true); running {
		return nil
	}
	_ = s.Repo.EnsureSyncState(ctx, connID, orgID, time.Now().UTC())
	_ = s.Repo.MakeDue(ctx, orgID, connID)
	go func() {
		defer s.syncing.Delete(connID)
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if items, err := s.Repo.ClaimDueForConnection(bg, connID, drainBatch, drainLease); err == nil && len(items) > 0 {
			s.drainConnection(bg, orgID, connID, items)
		}
		if c2, err := s.open(bg, orgID, connID); err == nil && !s.overBudget(bg, c2) {
			s.pullConnection(bg, c2)
			s.settle(bg, c2)
		}
	}()
	return nil
}
