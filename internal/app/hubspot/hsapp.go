package hubspot

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// Inside HubSpot: the Warmbly card on contact records and the "Add to Warmbly
// campaign" workflow action. HubSpot signs every request with the app's
// client secret; the portal it names picks the workspace. The card acts as the
// member matched to the signed-in HubSpot user; a workflow acts as the owner.

// User is the signed-in HubSpot user a card request was signed for.
type User struct {
	ID    string
	Email string
}

// Leads enrolls contacts in campaigns and reads their campaign state (the
// contact service). Nil disables the in-HubSpot actions.
type Leads interface {
	Add(ctx context.Context, userID string, orgID uuid.UUID, contacts []models.AddContact) ([]models.Contact, *errx.Error)
	BulkUpdate(ctx context.Context, userID string, orgID uuid.UUID, data *models.BulkEditContactsData) ([]models.Contact, *errx.Error)
	CampaignStates(ctx context.Context, orgID, contactID uuid.UUID) ([]models.ContactCampaignState, *errx.Error)
}

// CardCampaign is one campaign on the HubSpot card.
type CardCampaign struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LeadStatus string `json:"lead_status"`
	Step       string `json:"step"`
	Held       bool   `json:"held"`
	HoldReason string `json:"hold_reason,omitempty"`
}

// CardView is what the HubSpot record card shows.
type CardView struct {
	Connected  bool           `json:"connected"`
	InWarmbly  bool           `json:"in_warmbly"`
	ContactURL string         `json:"contact_url,omitempty"`
	Status     string         `json:"status,omitempty"`
	Intent     string         `json:"intent,omitempty"`
	Campaigns  []CardCampaign `json:"campaigns"`
	Options    []CardOption   `json:"options"`
}

// CardOption is a campaign the contact can be added to.
type CardOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// portalOrg resolves the workspace running HubSpot mode on a portal.
func (s *Service) portalOrg(ctx context.Context, portalID string) (*org, *errx.Error) {
	ids, err := s.d.Repo.OrgsForAccount(ctx, provider, strings.TrimSpace(portalID))
	if err != nil {
		return nil, errx.InternalError()
	}
	if len(ids) == 0 {
		return nil, errx.NewWithIdentifier(errx.NotFound, "crm_not_connected",
			"This HubSpot account is not connected to a Warmbly workspace in HubSpot mode.")
	}
	// A portal must name exactly one workspace, or nothing is chosen for it.
	if len(ids) > 1 {
		return nil, errx.New(errx.Conflict,
			"This HubSpot account is connected to more than one Warmbly workspace in HubSpot mode. Keep HubSpot mode on in one of them.")
	}
	o, err := s.resolve(ctx, ids[0])
	if err != nil || o == nil {
		return nil, errx.NewWithIdentifier(errx.NotFound, "crm_not_connected", "HubSpot mode is off for this workspace.")
	}
	return o, nil
}

// cardMember is the member a card request acts as: the one matched to the
// signed-in HubSpot user, holding every permission in perm.
func (s *Service) cardMember(ctx context.Context, o *org, u User, perm models.OrganizationPermission) (string, *errx.Error) {
	email := strings.TrimSpace(u.Email)
	if email == "" || strings.TrimSpace(u.ID) == "" {
		return "", errx.New(errx.Forbidden, "HubSpot did not say who you are. Reload the record and try again.")
	}
	id, perms, err := s.d.Repo.MemberForOwnerEmail(ctx, o.ID, provider, email)
	if err != nil {
		return "", errx.InternalError()
	}
	if id == nil {
		return "", errx.New(errx.Forbidden,
			"Your HubSpot user is not matched to a Warmbly member. Ask a workspace admin to match you under Owners on the HubSpot page in Warmbly.")
	}
	if !perms.HasPermission(perm) {
		return "", errx.New(errx.Forbidden, "Your Warmbly role does not allow this. Ask a workspace admin for access.")
	}
	return id.String(), nil
}

// contactFor finds the Warmbly contact behind a HubSpot record.
func (s *Service) contactFor(ctx context.Context, o *org, hsContactID, email string) (*models.Contact, error) {
	if hsContactID != "" {
		rec, err := s.d.Repo.GetContactRecordByExternal(ctx, o.ID, provider, hsContactID)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			cs, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{rec.ContactID})
			if xerr != nil {
				return nil, xerr
			}
			if len(cs) == 1 {
				return &cs[0], nil
			}
		}
	}
	if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
		c, xerr := s.d.Contacts.GetByEmailAndOrganization(ctx, o.ID, email)
		if xerr == nil && c != nil {
			return c, nil
		}
	}
	return nil, nil
}

// Card builds the record card for a HubSpot contact.
func (s *Service) Card(ctx context.Context, portalID string, u User, hsContactID, email string) (*CardView, *errx.Error) {
	o, xerr := s.portalOrg(ctx, portalID)
	if xerr != nil {
		if xerr.Code == errx.NotFound {
			return &CardView{Campaigns: []CardCampaign{}, Options: []CardOption{}}, nil
		}
		return nil, xerr
	}
	if _, xerr := s.cardMember(ctx, o, u, models.PermViewCampaigns|models.PermViewContacts); xerr != nil {
		return nil, xerr
	}
	out := &CardView{Connected: true, Campaigns: []CardCampaign{}, Options: []CardOption{}}
	opts, err := s.d.Repo.CampaignOptions(ctx, o.ID, "", 50)
	if err != nil {
		return nil, errx.InternalError()
	}
	for _, c := range opts {
		out.Options = append(out.Options, CardOption{Label: c.Name, Value: c.ID.String()})
	}
	c, err := s.contactFor(ctx, o, hsContactID, email)
	if err != nil {
		return nil, errx.InternalError()
	}
	if c == nil {
		return out, nil
	}
	out.InWarmbly = true
	if s.d.AppURL != "" {
		out.ContactURL = s.contactLink(c.ID)
	}
	if rec, _ := s.d.Repo.GetContactRecord(ctx, o.ID, c.ID, provider); rec != nil {
		out.Status = rec.Properties[propStatus]
	}
	if s.d.Leads != nil {
		states, xerr := s.d.Leads.CampaignStates(ctx, o.ID, c.ID)
		if xerr == nil {
			for _, st := range states {
				cc := CardCampaign{ID: st.CampaignID.String(), Name: st.CampaignName, LeadStatus: st.LeadStatus}
				if st.Hold != nil {
					cc.Held = true
					cc.HoldReason = st.Hold.Reason
				}
				if total := len(st.Steps); total > 0 {
					cc.Step = stepLabel(st.CompletedSteps, total)
				}
				out.Campaigns = append(out.Campaigns, cc)
			}
		}
	}
	return out, nil
}

func stepLabel(done, total int) string {
	if done >= total {
		return "All steps sent"
	}
	return "Step " + strconv.Itoa(done+1) + " of " + strconv.Itoa(total)
}

// Enroll adds a HubSpot contact to a Warmbly campaign, creating the Warmbly
// contact from the HubSpot record when it is new. A card request (u set) acts
// as the user's matched member; a workflow acts as the workspace owner.
func (s *Service) Enroll(ctx context.Context, portalID, hsContactID, email, campaignID string, u *User) *errx.Error {
	o, xerr := s.portalOrg(ctx, portalID)
	if xerr != nil {
		return xerr
	}
	var actor string
	if u != nil {
		if actor, xerr = s.cardMember(ctx, o, *u, models.PermManageCampaigns|models.PermManageContacts); xerr != nil {
			return xerr
		}
	} else {
		id, err := s.d.Repo.FallbackActor(ctx, o.ID)
		if err != nil {
			return errx.InternalError()
		}
		actor = id.String()
	}
	if s.d.Leads == nil {
		return errx.New(errx.NotImplemented, "enrolment from HubSpot is not available on this instance")
	}
	cid, err := uuid.Parse(strings.TrimSpace(campaignID))
	if err != nil {
		return errx.New(errx.BadRequest, "choose a campaign")
	}
	opts, err := s.d.Repo.CampaignOptions(ctx, o.ID, "", 500)
	if err != nil {
		return errx.InternalError()
	}
	found := false
	for _, c := range opts {
		if c.ID == cid {
			found = true
			break
		}
	}
	if !found {
		return errx.New(errx.NotFound, "That campaign is not in this workspace or has finished.")
	}
	c, err := s.contactFor(ctx, o, hsContactID, email)
	if err != nil {
		return errx.InternalError()
	}
	if c != nil {
		if _, xerr := s.d.Leads.BulkUpdate(ctx, actor, o.ID, &models.BulkEditContactsData{
			ContactSelection: models.ContactSelection{Contacts: []string{c.ID.String()}},
			AddCampaigns:     []string{cid.String()},
		}); xerr != nil {
			return xerr
		}
		return nil
	}
	if hsContactID == "" {
		return errx.New(errx.BadRequest, "the contact has no email address")
	}
	obj, err := o.Client.GetObject(ctx, "contacts", hsContactID, s.readProps(o), nil)
	if err != nil {
		return s.userError(ctx, o, err)
	}
	addr := strings.ToLower(strings.TrimSpace(firstNonEmpty(obj.Prop("email"), email)))
	if addr == "" {
		return errx.New(errx.BadRequest, "the contact has no email address")
	}
	if strings.EqualFold(obj.Prop("hs_email_optout"), "true") {
		return errx.NewWithIdentifier(errx.Conflict, "crm_opted_out", "This contact opted out of email in HubSpot.")
	}
	created, xerr := s.d.Leads.Add(ctx, actor, o.ID, []models.AddContact{{
		Email: addr, FirstName: obj.Prop("firstname"), LastName: obj.Prop("lastname"),
		Company: obj.Prop("company"), Phone: obj.Prop("phone"), Campaigns: []string{cid.String()},
	}})
	if xerr != nil {
		return xerr
	}
	if len(created) == 1 {
		if _, _, err := s.storeContactRecord(ctx, o, created[0].ID, obj); err != nil {
			return errx.InternalError()
		}
	}
	return nil
}

// SetPaused holds or resumes every campaign a HubSpot contact is in.
func (s *Service) SetPaused(ctx context.Context, portalID string, u User, hsContactID, email string, paused bool) *errx.Error {
	o, xerr := s.portalOrg(ctx, portalID)
	if xerr != nil {
		return xerr
	}
	if _, xerr := s.cardMember(ctx, o, u, models.PermManageCampaigns); xerr != nil {
		return xerr
	}
	c, err := s.contactFor(ctx, o, hsContactID, email)
	if err != nil {
		return errx.InternalError()
	}
	if c == nil {
		return errx.New(errx.NotFound, "This contact is not in Warmbly.")
	}
	if paused {
		if s.d.Holds != nil {
			if _, err := s.d.Holds.HoldLeadEverywhere(ctx, c.ID, nil, "Paused from HubSpot", models.LeadHoldSourceManual); err != nil {
				return errx.InternalError()
			}
		}
	} else if _, err := s.d.Repo.ResumeHeldEverywhere(ctx, o.ID, c.ID); err != nil {
		return errx.InternalError()
	}
	s.notify(ctx, o.ID, c.ID.String(), "contact")
	return nil
}

// CampaignChoices answers a workflow action's campaign dropdown.
func (s *Service) CampaignChoices(ctx context.Context, portalID, query string) ([]CardOption, *errx.Error) {
	o, xerr := s.portalOrg(ctx, portalID)
	if xerr != nil {
		return []CardOption{}, nil
	}
	opts, err := s.d.Repo.CampaignOptions(ctx, o.ID, query, 100)
	if err != nil {
		return nil, errx.InternalError()
	}
	out := make([]CardOption, 0, len(opts))
	for _, c := range opts {
		label := c.Name
		if c.Status != "active" {
			label += " (" + strings.ReplaceAll(c.Status, "_", " ") + ")"
		}
		out = append(out, CardOption{Label: label, Value: c.ID.String()})
	}
	return out, nil
}
