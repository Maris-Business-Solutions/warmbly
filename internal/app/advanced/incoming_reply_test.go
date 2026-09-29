package advanced

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type incomingReplyAdvancedRepo struct {
	repository.AdvancedOutreachRepository
	marked    int
	intents   int
	intentOff bool
}

func (r *incomingReplyAdvancedRepo) GetOutreachSettings(context.Context, uuid.UUID) (*models.AdvancedOutreachSettings, error) {
	settings := models.DefaultAdvancedOutreachSettings()
	settings.ReplyIntent.Enabled = !r.intentOff
	return &settings, nil
}

func (r *incomingReplyAdvancedRepo) MarkVariantEvent(context.Context, uuid.UUID, uuid.UUID, string) error {
	r.marked++
	return nil
}

func (r *incomingReplyAdvancedRepo) CreateReplyIntent(context.Context, *models.ReplyIntentRecord) error {
	r.intents++
	return nil
}

type incomingReplyEmailRepo struct {
	repository.EmailRepository
	account *models.Email
}

func (r incomingReplyEmailRepo) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	return r.account, nil
}

type incomingReplyTaskRepo struct {
	repository.TaskRepository
	task     *repository.Task
	campaign *repository.CampaignTask
}

func (r incomingReplyTaskRepo) GetTaskByMessageID(context.Context, string) (*repository.Task, error) {
	return r.task, nil
}

func (r incomingReplyTaskRepo) GetCampaignTask(context.Context, uuid.UUID) (*repository.CampaignTask, error) {
	return r.campaign, nil
}

type incomingReplyContactRepo struct {
	repository.ContactRepository
	senderContact *models.Contact
	taskContact   *models.Contact
}

func (r incomingReplyContactRepo) GetByEmailAndOrganization(context.Context, uuid.UUID, string) (*models.Contact, *errx.Error) {
	return r.senderContact, nil
}

func (r incomingReplyContactRepo) GetByID(context.Context, uuid.UUID) (*models.Contact, *errx.Error) {
	return r.taskContact, nil
}

type incomingReplyProgressRepo struct {
	repository.CampaignProgressRepository
	replied       int
	classified    int
	claims        int
	completed     int
	latest        *repository.CampaignSequencePair
	sourceInbound bool
	sourceClaimed bool
	replyAccepted bool
	receivingSent bool
	completeErr   error
	advanced      *incomingReplyAdvancedRepo
	copies        []models.CampaignLeadCC
	copiedLead    *repository.CopiedLeadRef
	heldEvery     int
}

func (r *incomingReplyProgressRepo) ListLeadCC(context.Context, uuid.UUID, uuid.UUID) ([]models.CampaignLeadCC, error) {
	return r.copies, nil
}

func (r *incomingReplyProgressRepo) LeadForCopiedReply(context.Context, uuid.UUID, uuid.UUID) (*repository.CopiedLeadRef, error) {
	return r.copiedLead, nil
}

func (r *incomingReplyProgressRepo) IsInboundReplySource(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return r.sourceInbound, nil
}

func (r *incomingReplyProgressRepo) CampaignContactSentFromAccount(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	return r.receivingSent, nil
}

func (r *incomingReplyProgressRepo) GetLatestCampaignSequenceForContact(context.Context, uuid.UUID) (*repository.CampaignSequencePair, error) {
	return r.latest, nil
}

func (r *incomingReplyProgressRepo) GetLatestReplyClass(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}

func (r *incomingReplyProgressRepo) RecordReplyClassification(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, float64) error {
	r.classified++
	return nil
}

func (r *incomingReplyProgressRepo) ClaimIncomingReply(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	r.claims++
	if !r.sourceClaimed {
		return uuid.Nil, nil
	}
	return uuid.New(), nil
}

func (r *incomingReplyProgressRepo) CompleteIncomingReply(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	r.completed++
	return r.completeErr
}

func (r *incomingReplyProgressRepo) RecordEmailReplied(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
	r.replied++
	return r.replyAccepted, nil
}

type incomingReplyCampaignRepo struct {
	repository.CampaignRepository
	campaign *models.Campaign
}

func (r incomingReplyCampaignRepo) GetByID(context.Context, uuid.UUID) (*models.Campaign, error) {
	return r.campaign, nil
}

func (incomingReplyCampaignRepo) GetSequencesRoutingByCampaignID(context.Context, uuid.UUID) ([]models.Sequence, error) {
	return nil, nil
}

func newIncomingReplyService(account *models.Email, senderContact *models.Contact, taskContact uuid.UUID) (*service, *incomingReplyProgressRepo) {
	taskID, campaignID, sequenceID := uuid.New(), uuid.New(), uuid.New()
	progress := &incomingReplyProgressRepo{
		sourceInbound: true,
		sourceClaimed: true,
		replyAccepted: true,
		receivingSent: true,
	}
	advancedRepo := &incomingReplyAdvancedRepo{}
	progress.advanced = advancedRepo
	taskContactRecord := &models.Contact{ID: taskContact, Email: "task-contact@example.test"}
	if senderContact != nil && senderContact.ID == taskContact {
		taskContactRecord = senderContact
	}
	return &service{
		repo: advancedRepo,
		campaignRepo: incomingReplyCampaignRepo{campaign: &models.Campaign{
			ID:             campaignID,
			OrganizationID: account.OrganizationID,
		}},
		emailRepo: incomingReplyEmailRepo{account: account},
		taskRepo: incomingReplyTaskRepo{
			task:     &repository.Task{ID: taskID, TaskType: "campaign", EmailAccountID: account.ID},
			campaign: &repository.CampaignTask{TaskID: taskID, CampaignID: &campaignID, ContactID: &taskContact, SequenceID: &sequenceID},
		},
		contactRepo: incomingReplyContactRepo{
			senderContact: senderContact,
			taskContact:   taskContactRecord,
		},
		campaignProgressRepo: progress,
	}, progress
}

func TestMessageAddressesMailbox(t *testing.T) {
	account := &models.Email{
		Email:       "mailbox@example.test",
		SendAsEmail: "alias@example.test",
		ReplyTo:     "replies@example.test",
	}
	for _, tc := range []struct {
		name    string
		message *models.EmailMessageStoreData
		want    bool
	}{
		{name: "mailbox in to", message: &models.EmailMessageStoreData{ToAddr: []string{"Mailbox <mailbox@example.test>"}}, want: true},
		{name: "send alias in cc", message: &models.EmailMessageStoreData{CC: []string{"alias@example.test"}}, want: true},
		{name: "reply address in bcc", message: &models.EmailMessageStoreData{BCC: []string{"replies@example.test"}}, want: true},
		{name: "different recipient", message: &models.EmailMessageStoreData{ToAddr: []string{"other@example.test"}}},
		{name: "IMAP sync form in to", message: &models.EmailMessageStoreData{ToAddr: []string{"M Kannan (mailbox@example.test)"}}, want: true},
		{name: "IMAP sync form, other recipient", message: &models.EmailMessageStoreData{ToAddr: []string{"Other (other@example.test)"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := messageAddressesMailbox(tc.message, account); got != tc.want {
				t.Fatalf("messageAddressesMailbox() = %t, want %t", got, tc.want)
			}
		})
	}
}

// A person answers from whatever address their client picks: a Gmail "send
// as" alias, a forward, a colleague's desk. The thread names the send, and
// that is the evidence; the From address is not a second condition. This is
// the self-hoster's report: every reply arrived, none was ever counted.
func TestProcessIncomingReplyCountsAThreadReplyFromAnotherAddress(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	// The alias is nobody in the contact list; only the thread can attribute it.
	service, progress := newIncomingReplyService(account, nil, contactID)

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"M K <alias@gmail.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
		Snippet:   "Received it, thank you.",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 for a reply in the lead's own thread", progress.replied)
	}
	if progress.advanced.marked != 1 {
		t.Fatalf("MarkVariantEvent calls = %d, want the reply counted for the variant", progress.advanced.marked)
	}
}

// Turning reply-intent automation off must not turn reply detection off:
// replied_at, stop-on-reply and the analytics all hang on it.
func TestProcessIncomingReplyStampsRepliedWithIntentAutomationOff(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	progress.advanced.intentOff = true

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
		Snippet:   "Sounds good, let's talk.",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 with intent automation off", progress.replied)
	}
	if progress.advanced.intents != 0 {
		t.Fatalf("CreateReplyIntent calls = %d, want 0 with intent automation off", progress.advanced.intents)
	}
}

func TestParseSenderEmailReadsEveryStoredForm(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{[]string{"M K <MKMSC74@gmail.com>"}, "mkmsc74@gmail.com"},
		{[]string{"mkmsc74@gmail.com"}, "mkmsc74@gmail.com"},
		{[]string{"M K (mkmsc74@gmail.com)"}, "mkmsc74@gmail.com"},
		{[]string{"", "x@y.test"}, ""},
		{nil, ""},
	} {
		if got := parseSenderEmail(tc.in); got != tc.want {
			t.Errorf("parseSenderEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The self-hoster's exact case: an IONOS mailbox over IMAP, whose sync stored
// every address as "Name (addr)". The From matched the contact and the To
// named the mailbox, and both checks still failed because neither could read
// that form, so no reply into an IMAP mailbox counted for a week.
func TestProcessIncomingReplyCountsARepliedStoredInTheIMAPForm(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "kannan@eml.example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "mkmsc74@gmail.com",
	}, contactID)

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"M K (mkmsc74@gmail.com)"},
		ToAddr:    []string{"M Kannan (kannan@eml.example.test)"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
		Snippet:   "Recd it thank you..",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 for a reply the IMAP sync stored", progress.replied)
	}
}

func TestProcessIncomingReplyRejectsMailboxOwnSentCopy(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, nil, contactID)

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Sender <sender@example.test>"},
		ToAddr:    []string{"recipient@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 for the mailbox's own outbound copy", progress.replied)
	}
}

func TestProcessIncomingReplyRejectsOutboundFolder(t *testing.T) {
	for _, tc := range []struct {
		name, folder, providerFolder string
	}{
		{name: "sent", folder: models.FolderSent},
		{name: "draft", folder: models.FolderDrafts},
		{name: "provider sent", folder: models.FolderInbox, providerFolder: models.FolderSent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
			account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
			service, progress := newIncomingReplyService(account, &models.Contact{
				ID: contactID, Email: "recipient@example.test",
			}, contactID)

			xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
				EmailID:        accountID,
				Folder:         tc.folder,
				ProviderFolder: tc.providerFolder,
				FromAddr:       []string{"Recipient <recipient@example.test>"},
				ToAddr:         []string{"sender@example.test"},
				InReplyTo:      []string{"<opener@example.test>"},
				Subject:        "Re: Hello",
			})
			if xerr != nil {
				t.Fatal(xerr)
			}
			if progress.replied != 0 {
				t.Fatalf("RecordEmailReplied calls = %d, want 0 for outbound folder", progress.replied)
			}
		})
	}
}

func TestProcessIncomingReplyTrustsPersistedDirectionOverEventPayload(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	progress.sourceInbound = false

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 when the stored source is outbound", progress.replied)
	}
}

func TestProcessIncomingReplyStopsWhenWriteBoundaryRejectsSource(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	progress.replyAccepted = false

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 write-boundary claim", progress.replied)
	}
	if progress.advanced.marked != 0 {
		t.Fatalf("MarkVariantEvent calls = %d, want 0 after rejected claim", progress.advanced.marked)
	}
	if progress.advanced.intents != 0 {
		t.Fatalf("CreateReplyIntent calls = %d, want 0 after rejected claim", progress.advanced.intents)
	}
	if progress.completed != 1 {
		t.Fatalf("CompleteIncomingReply calls = %d, want 1 after rejected write", progress.completed)
	}
}

func TestProcessIncomingReplyClaimsBeforePersistingAutomatedState(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	progress.sourceClaimed = false

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Automatic reply: out of office",
		Snippet:   "I am out of the office.",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.claims != 1 {
		t.Fatalf("ClaimIncomingReply calls = %d, want 1", progress.claims)
	}
	if progress.classified != 0 {
		t.Fatalf("RecordReplyClassification calls = %d, want 0 after rejected claim", progress.classified)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 for rejected automated reply", progress.replied)
	}
	if progress.advanced.intents != 0 {
		t.Fatalf("CreateReplyIntent calls = %d, want 0 after rejected claim", progress.advanced.intents)
	}
	if progress.completed != 0 {
		t.Fatalf("CompleteIncomingReply calls = %d, want 0 without a claim", progress.completed)
	}
}

// Someone else answering in the lead's thread (a colleague, an assistant,
// another contact on the same list) is still an answer to the email we sent
// that lead; it is attributed to the thread, not to the sender's own latest
// campaign. #549 was the mailbox's OWN outbound copy, which the folder and
// sender checks above still refuse.
func TestProcessIncomingReplyCountsAThreadReplyFromAnotherContact(t *testing.T) {
	orgID, accountID := uuid.New(), uuid.New()
	taskContactID, senderContactID := uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: senderContactID, Email: "other@example.test",
	}, taskContactID)
	latestCampaignID, latestSequenceID := uuid.New(), uuid.New()
	progress.latest = &repository.CampaignSequencePair{CampaignID: latestCampaignID, SequenceID: latestSequenceID}

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Other person <other@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want the thread's lead marked replied", progress.replied)
	}
}

func TestProcessIncomingReplyRequiresRecipientToMatchMailbox(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"someone-else@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 when the recipient is not the sending mailbox", progress.replied)
	}
}

func TestProcessIncomingReplyAcceptsCrossMailboxThreadWhenStoredSourceIsInbound(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	service.taskRepo.(incomingReplyTaskRepo).task.EmailAccountID = uuid.New()
	latestCampaignID, latestSequenceID := uuid.New(), uuid.New()
	progress.latest = &repository.CampaignSequencePair{CampaignID: latestCampaignID, SequenceID: latestSequenceID}

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 for an inbound cross-mailbox thread", progress.replied)
	}
	if progress.completed != 1 {
		t.Fatalf("CompleteIncomingReply calls = %d, want 1 for an inbound cross-mailbox thread", progress.completed)
	}
}

func TestProcessIncomingReplyRejectsCrossOrganizationThread(t *testing.T) {
	orgID, otherOrgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	service.taskRepo.(incomingReplyTaskRepo).task.EmailAccountID = uuid.New()
	service.campaignRepo = incomingReplyCampaignRepo{campaign: &models.Campaign{
		ID:             uuid.New(),
		OrganizationID: &otherOrgID,
	}}

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 for a cross-organization thread", progress.replied)
	}
}

func TestProcessIncomingReplyRejectsCrossMailboxThreadAtUnrelatedWorkspaceMailbox(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "unrelated@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	service.taskRepo.(incomingReplyTaskRepo).task.EmailAccountID = uuid.New()
	progress.receivingSent = false

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"unrelated@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want 0 for an unrelated workspace mailbox", progress.replied)
	}
}

func TestProcessIncomingReplyAcceptsMatchingThreadSender(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want 1 for the matching contact", progress.replied)
	}
	if progress.completed != 1 {
		t.Fatalf("CompleteIncomingReply calls = %d, want 1 for the matching contact", progress.completed)
	}
}

func TestProcessIncomingReplyFencesExpiredClaimBeforeSideEffects(t *testing.T) {
	orgID, accountID, contactID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	service, progress := newIncomingReplyService(account, &models.Contact{
		ID: contactID, Email: "recipient@example.test",
	}, contactID)
	progress.completeErr = errors.New("claim was replaced")

	xerr := service.ProcessIncomingReply(context.Background(), accountID, &models.EmailMessageStoreData{
		ID:        uuid.New(),
		EmailID:   accountID,
		Folder:    models.FolderInbox,
		FromAddr:  []string{"Recipient <recipient@example.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: []string{"<opener@example.test>"},
		Subject:   "Re: Hello",
	})
	if xerr == nil {
		t.Fatal("expected the replaced claim to stop processing")
	}
	if progress.completed != 1 {
		t.Fatalf("CompleteIncomingReply calls = %d, want 1", progress.completed)
	}
	if progress.advanced.marked != 0 {
		t.Fatalf("MarkVariantEvent calls = %d, want 0 after ownership was lost", progress.advanced.marked)
	}
	if progress.advanced.intents != 0 {
		t.Fatalf("CreateReplyIntent calls = %d, want 0 after ownership was lost", progress.advanced.intents)
	}
}
