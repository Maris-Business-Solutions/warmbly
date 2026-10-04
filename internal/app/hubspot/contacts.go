package hubspot

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// contactProps are the contact properties every pull reads.
var contactProps = []string{"email", "firstname", "lastname", "company", "phone", "jobtitle", "lifecyclestage",
	"hs_lead_status", "hubspot_owner_id", "associatedcompanyid", "hs_email_optout", "lastmodifieddate", propStatus}

func (s *Service) readProps(o *org) []string {
	props := append([]string{}, contactProps...)
	seen := map[string]bool{}
	for _, p := range props {
		seen[p] = true
	}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			props = append(props, p)
		}
	}
	for _, p := range o.Config.DisplayProperties {
		add(p)
	}
	for _, p := range o.Config.FieldMap {
		add(p)
	}
	return props
}

// pushProperties projects a Warmbly contact onto HubSpot properties through
// the field map, for the fields Warmbly is allowed to write.
func pushProperties(o *org, c *models.Contact, creating bool) map[string]string {
	out := map[string]string{}
	for field, prop := range o.Config.FieldMap {
		dir := o.Config.FieldDirection[field]
		if dir == models.CRMFieldPull && !creating {
			continue
		}
		var v string
		switch {
		case field == "first_name":
			v = c.FirstName
		case field == "last_name":
			v = c.LastName
		case field == "company":
			v = c.Company
		case field == "phone":
			v = c.Phone
		case field == "email":
			continue
		case strings.HasPrefix(field, "custom:"):
			v = c.CustomFields[strings.TrimPrefix(field, "custom:")]
		}
		if strings.TrimSpace(v) != "" {
			out[prop] = v
		}
	}
	return out
}

// ensureContact returns the HubSpot contact id for a Warmbly contact, creating
// the contact (and its company) when the workspace allows it. "" with a nil
// error means the contact is not in HubSpot and may not be created.
func (s *Service) ensureContact(ctx context.Context, o *org, contactID uuid.UUID) (string, *models.CRMContactRecord, error) {
	rec, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
	if err != nil {
		return "", nil, err
	}
	if rec != nil {
		return rec.ExternalID, rec, nil
	}
	contacts, xerr := s.d.Contacts.GetByIDsAndOrganization(ctx, o.ID, []uuid.UUID{contactID})
	if xerr != nil {
		return "", nil, xerr
	}
	if len(contacts) == 0 || strings.TrimSpace(contacts[0].Email) == "" {
		return "", nil, nil
	}
	c := &contacts[0]
	return s.ensureContactFor(ctx, o, c)
}

func (s *Service) ensureContactFor(ctx context.Context, o *org, c *models.Contact) (string, *models.CRMContactRecord, error) {
	email := strings.ToLower(strings.TrimSpace(c.Email))
	page, err := o.Client.Search(ctx, "contacts", [][]Filter{{{PropertyName: "email", Operator: "EQ", Value: email}}},
		"", s.readProps(o), "", 1)
	if err != nil {
		return "", nil, err
	}
	var obj *Object
	if len(page.Results) > 0 {
		obj = &page.Results[0]
	} else {
		if !o.Config.CreateContacts {
			return "", nil, nil
		}
		props := pushProperties(o, c, true)
		if o.Config.WriteProperties && s.d.AppURL != "" {
			props[propLink] = s.contactLink(c.ID)
		}
		ids, uerr := o.Client.UpsertContacts(ctx, []ContactUpsert{{Email: email, Properties: props}})
		if uerr != nil {
			return "", nil, uerr
		}
		id := ids[email]
		if id == "" {
			return "", nil, nil
		}
		if obj, err = o.Client.GetObject(ctx, "contacts", id, s.readProps(o), nil); err != nil {
			return "", nil, err
		}
		if o.Config.CreateCompanies && obj.Prop("associatedcompanyid") == "" {
			s.attachCompany(ctx, o, obj.ID, c)
			if refreshed, gerr := o.Client.GetObject(ctx, "contacts", id, s.readProps(o), nil); gerr == nil {
				obj = refreshed
			}
		}
	}
	rec, _, err := s.storeContactRecord(ctx, o, c.ID, obj)
	if err != nil {
		return "", nil, err
	}
	return obj.ID, rec, nil
}

// attachCompany associates a new contact with the company on its email domain,
// creating the company when HubSpot has none.
func (s *Service) attachCompany(ctx context.Context, o *org, hsContactID string, c *models.Contact) {
	domain := emailDomain(c.Email)
	if domain == "" {
		return
	}
	company, err := o.Client.FindCompanyByDomain(ctx, domain)
	if err != nil {
		return
	}
	if company == nil {
		name := firstNonEmpty(c.Company, domain)
		if company, err = o.Client.Create(ctx, "companies", map[string]string{"domain": domain, "name": name}, nil); err != nil {
			return
		}
	}
	_ = o.Client.AssociateDefault(ctx, "contacts", hsContactID, "companies", company.ID)
}

// storeContactRecord mirrors a HubSpot contact onto its Warmbly contact and
// returns the record before the write, so callers can see what changed.
func (s *Service) storeContactRecord(ctx context.Context, o *org, contactID uuid.UUID, obj *Object) (*models.CRMContactRecord, *models.CRMContactRecord, error) {
	prev, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
	if err != nil {
		return nil, nil, err
	}
	rec := &models.CRMContactRecord{
		OrganizationID:    o.ID,
		ContactID:         contactID,
		Provider:          provider,
		ExternalID:        obj.ID,
		OwnerExternalID:   obj.Prop("hubspot_owner_id"),
		LifecycleStage:    obj.Prop("lifecyclestage"),
		LeadStatus:        obj.Prop("hs_lead_status"),
		CompanyExternalID: obj.Prop("associatedcompanyid"),
		OptedOut:          strings.EqualFold(obj.Prop("hs_email_optout"), "true"),
		Properties:        map[string]string{},
	}
	if t := parseHSTime(obj.Prop("lastmodifieddate")); t != nil {
		rec.ExternalUpdatedAt = t
	} else if !obj.UpdatedAt.IsZero() {
		u := obj.UpdatedAt
		rec.ExternalUpdatedAt = &u
	}
	for _, p := range append([]string{propStatus}, o.Config.DisplayProperties...) {
		if v := obj.Prop(p); v != "" {
			rec.Properties[p] = v
		}
	}
	if rec.CompanyExternalID != "" {
		if prev != nil && prev.CompanyExternalID == rec.CompanyExternalID && prev.CompanyName != "" {
			rec.CompanyName, rec.CompanyDomain = prev.CompanyName, prev.CompanyDomain
		} else if co, err := o.Client.GetObject(ctx, "companies", rec.CompanyExternalID, []string{"name", "domain"}, nil); err == nil {
			rec.CompanyName, rec.CompanyDomain = co.Prop("name"), co.Prop("domain")
		}
	}
	if err := s.d.Repo.UpsertContactRecord(ctx, rec); err != nil {
		return nil, nil, err
	}
	return rec, prev, nil
}

func (s *Service) contactLink(contactID uuid.UUID) string {
	return strings.TrimRight(s.d.AppURL, "/") + "/app/contacts?contact=" + contactID.String()
}

// ContactView is the HubSpot side of a contact for the drawer and inbox panel.
func (s *Service) ContactView(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	out := &models.CRMContactView{Provider: provider, Properties: []models.CRMPropertyView{}}
	rec, err := s.d.Repo.GetContactRecord(ctx, orgID, contactID, provider)
	if err != nil {
		return nil, errx.InternalError()
	}
	if rec == nil {
		return out, nil
	}
	out.Linked = true
	out.ExternalID = rec.ExternalID
	out.URL = o.recordURL("0-1", rec.ExternalID)
	out.OptedOut = rec.OptedOut
	synced := rec.SyncedAt
	out.SyncedAt = &synced
	if rec.OwnerExternalID != "" {
		owners, _ := s.d.Repo.ListOwners(ctx, orgID, provider)
		for i := range owners {
			if owners[i].ExternalID == rec.OwnerExternalID {
				out.Owner = &owners[i]
				break
			}
		}
		if out.Owner == nil {
			out.Owner = &models.CRMOwner{ExternalID: rec.OwnerExternalID}
		}
	}
	if rec.LifecycleStage != "" {
		out.LifecycleStage = &models.CRMOption{Value: rec.LifecycleStage, Label: s.optionLabel(ctx, o, "lifecyclestage", rec.LifecycleStage)}
	}
	if rec.LeadStatus != "" {
		out.LeadStatus = &models.CRMOption{Value: rec.LeadStatus, Label: s.optionLabel(ctx, o, "hs_lead_status", rec.LeadStatus)}
	}
	if rec.CompanyExternalID != "" {
		out.Company = &models.CRMCompanyRef{ExternalID: rec.CompanyExternalID, Name: rec.CompanyName, Domain: rec.CompanyDomain,
			URL: o.recordURL("0-2", rec.CompanyExternalID)}
	}
	if len(o.Config.DisplayProperties) > 0 {
		labels := map[string]string{}
		if props, perr := s.contactProperties(ctx, o); perr == nil {
			for _, p := range props {
				labels[p.Name] = p.Label
			}
		}
		for _, name := range o.Config.DisplayProperties {
			label := labels[name]
			if label == "" {
				label = name
			}
			out.Properties = append(out.Properties, models.CRMPropertyView{Name: name, Label: label, Value: rec.Properties[name]})
		}
	}
	return out, nil
}

// LinkContact finds (or, when allowed, creates) the contact in HubSpot and
// pulls its deals, tasks and notes into Warmbly.
func (s *Service) LinkContact(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	ext, _, err := s.ensureContact(ctx, o, contactID)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	if ext == "" {
		return nil, errx.NewWithIdentifier(errx.NotFound, "crm_contact_missing",
			"This contact is not in HubSpot, and creating contacts is turned off in the HubSpot settings.")
	}
	if err := s.refreshContact(ctx, o, contactID, ext); err != nil {
		return nil, s.userError(ctx, o, err)
	}
	return s.ContactView(ctx, orgID, contactID)
}

// RefreshContact pulls one contact's HubSpot side now. Debounced, so opening
// the same contact twice in a minute costs one pull.
func (s *Service) RefreshContact(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	rec, err := s.d.Repo.GetContactRecord(ctx, orgID, contactID, provider)
	if err != nil {
		return nil, errx.InternalError()
	}
	if rec == nil {
		// Not linked yet: look it up by email without creating anything.
		contacts, cerr := s.d.Contacts.GetByIDsAndOrganization(ctx, orgID, []uuid.UUID{contactID})
		if cerr != nil || len(contacts) == 0 {
			return nil, errx.New(errx.NotFound, "contact not found")
		}
		if !s.debounce(ctx, "lookup:"+contactID.String(), 5*time.Minute) {
			return s.ContactView(ctx, orgID, contactID)
		}
		page, serr := o.Client.Search(ctx, "contacts", [][]Filter{{{PropertyName: "email", Operator: "EQ",
			Value: strings.ToLower(strings.TrimSpace(contacts[0].Email))}}}, "", s.readProps(o), "", 1)
		if serr != nil {
			return nil, s.userError(ctx, o, serr)
		}
		if len(page.Results) == 0 {
			return s.ContactView(ctx, orgID, contactID)
		}
		if _, _, err := s.storeContactRecord(ctx, o, contactID, &page.Results[0]); err != nil {
			return nil, errx.InternalError()
		}
		rec = &models.CRMContactRecord{ExternalID: page.Results[0].ID}
	} else if !s.debounce(ctx, "refresh:"+contactID.String(), time.Minute) {
		return s.ContactView(ctx, orgID, contactID)
	}
	if err := s.refreshContact(ctx, o, contactID, rec.ExternalID); err != nil {
		return nil, s.userError(ctx, o, err)
	}
	return s.ContactView(ctx, orgID, contactID)
}

// UpdateContactRecord writes owner, lifecycle stage or lead status to HubSpot
// and the mirror.
func (s *Service) UpdateContactRecord(ctx context.Context, orgID, contactID uuid.UUID, upd *models.UpdateCRMContact) (*models.CRMContactView, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	ext, _, err := s.ensureContact(ctx, o, contactID)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	if ext == "" {
		return nil, errx.NewWithIdentifier(errx.NotFound, "crm_contact_missing", "This contact is not in HubSpot yet.")
	}
	props := map[string]string{}
	if upd.OwnerExternalID != nil {
		props["hubspot_owner_id"] = *upd.OwnerExternalID
	}
	if upd.LifecycleStage != nil {
		props["lifecyclestage"] = *upd.LifecycleStage
	}
	if upd.LeadStatus != nil {
		props["hs_lead_status"] = *upd.LeadStatus
	}
	if len(props) == 0 {
		return s.ContactView(ctx, orgID, contactID)
	}
	obj, err := o.Client.Update(ctx, "contacts", ext, props)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	full, err := o.Client.GetObject(ctx, "contacts", obj.ID, s.readProps(o), nil)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	if _, _, err := s.storeContactRecord(ctx, o, contactID, full); err != nil {
		return nil, errx.InternalError()
	}
	s.notify(ctx, orgID, contactID.String(), "contact")
	return s.ContactView(ctx, orgID, contactID)
}

// debounce is true when the key was not seen within ttl (and claims it).
func (s *Service) debounce(ctx context.Context, key string, ttl time.Duration) bool {
	if s.d.Cache == nil {
		return true
	}
	ok, err := s.d.Cache.SetNX(ctx, "hubspot:db:"+key, 1, ttl).Result()
	return err != nil || ok
}
