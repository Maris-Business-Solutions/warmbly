package pipedrive

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// contactName is the person name for a Warmbly contact.
func contactName(c *models.Contact) string {
	return firstNonEmpty(strings.TrimSpace(c.FirstName+" "+c.LastName), c.Email)
}

// personBody projects a Warmbly contact onto a person through the field map,
// for the fields Warmbly is allowed to write. Pipedrive splits a name into
// first and last itself, so those two travel as one name.
func (s *Service) personBody(ctx context.Context, o *org, c *models.Contact, creating bool) map[string]any {
	body := map[string]any{}
	custom := map[string]any{}
	writesName := false
	for field, key := range o.Config.FieldMap {
		if dir := o.Config.FieldDirection[field]; dir == models.CRMFieldPull && !creating {
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
		if strings.TrimSpace(v) == "" {
			continue
		}
		switch key {
		case keyFirstName, keyLastName, keyName:
			writesName = true
		case keyPhone:
			body["phones"] = []ContactPoint{{Value: v, Primary: true, Label: "work"}}
		case keyOrgName:
			if orgID := s.ensureOrganization(ctx, o, v, c.Email); orgID != 0 {
				body["org_id"] = orgID
			}
		default:
			custom[key] = v
		}
	}
	if writesName || creating {
		body["name"] = contactName(c)
	}
	if creating {
		if _, ok := body["org_id"]; !ok && o.Config.CreateCompanies {
			if orgID := s.ensureOrganization(ctx, o, c.Company, c.Email); orgID != 0 {
				body["org_id"] = orgID
			}
		}
	}
	if len(custom) > 0 {
		body["custom_fields"] = custom
	}
	return body
}

// ensureOrganization finds the organization by name (or the email's domain),
// creating it when the workspace allows. 0 when there is none to use.
func (s *Service) ensureOrganization(ctx context.Context, o *org, name, email string) int64 {
	name = strings.TrimSpace(name)
	if name == "" {
		name = emailDomain(email)
	}
	if name == "" {
		return 0
	}
	found, err := o.Client.FindOrganization(ctx, name)
	if err == nil && found != nil {
		return found.ID
	}
	if err != nil || !o.Config.CreateCompanies {
		return 0
	}
	created, err := o.Client.CreateOrganization(ctx, map[string]any{"name": truncateRunes(name, 255)})
	if err != nil {
		return 0
	}
	return created.ID
}

// ensureContact returns the Pipedrive person id for a Warmbly contact, creating
// the person when the workspace allows it. "" with a nil error means the
// contact is not in Pipedrive and may not be created.
func (s *Service) ensureContact(ctx context.Context, o *org, contactID uuid.UUID) (string, *models.CRMContactRecord, error) {
	return s.ensureContactOwned(ctx, o, contactID, 0)
}

// ensureContactOwned is ensureContact where a new person gets owner (a
// Pipedrive user id; 0 leaves Pipedrive's default, the connected user).
func (s *Service) ensureContactOwned(ctx context.Context, o *org, contactID uuid.UUID, owner int64) (string, *models.CRMContactRecord, error) {
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
	return s.ensureContactFor(ctx, o, &contacts[0], owner)
}

func (s *Service) ensureContactFor(ctx context.Context, o *org, c *models.Contact, owner int64) (string, *models.CRMContactRecord, error) {
	email := strings.ToLower(strings.TrimSpace(c.Email))
	personID, err := o.Client.FindPersonByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}
	var p *Person
	if personID != 0 {
		if p, err = o.Client.GetPerson(ctx, personID); err != nil {
			return "", nil, err
		}
	} else {
		if !o.Config.CreateContacts {
			return "", nil, nil
		}
		body := s.personBody(ctx, o, c, true)
		body["emails"] = []ContactPoint{{Value: email, Primary: true, Label: "work"}}
		if owner != 0 {
			body["owner_id"] = owner
		}
		if vals := s.warmblyValues(ctx, o, map[string]string{fieldLink: s.contactLink(c.ID)}); len(vals) > 0 {
			custom, _ := body["custom_fields"].(map[string]any)
			if custom == nil {
				custom = map[string]any{}
			}
			for k, v := range vals {
				custom[k] = v
			}
			body["custom_fields"] = custom
		}
		if p, err = o.Client.CreatePerson(ctx, body); err != nil {
			return "", nil, err
		}
	}
	rec, _, err := s.storeContactRecord(ctx, o, c.ID, p)
	if err != nil {
		return "", nil, err
	}
	return rec.ExternalID, rec, nil
}

// storeContactRecord mirrors a Pipedrive person onto its Warmbly contact and
// returns the record before the write, so callers can see what changed.
func (s *Service) storeContactRecord(ctx context.Context, o *org, contactID uuid.UUID, p *Person) (*models.CRMContactRecord, *models.CRMContactRecord, error) {
	prev, err := s.d.Repo.GetContactRecord(ctx, o.ID, contactID, provider)
	if err != nil {
		return nil, nil, err
	}
	rec := &models.CRMContactRecord{
		OrganizationID:    o.ID,
		ContactID:         contactID,
		Provider:          provider,
		ExternalID:        id(p.ID),
		OwnerExternalID:   ptrID(p.OwnerID),
		CompanyExternalID: ptrID(p.OrgID),
		OptedOut:          strings.EqualFold(p.MarketingStatus, "unsubscribed"),
		Properties:        map[string]string{},
		ExternalUpdatedAt: parseTime(p.UpdateTime),
	}
	if prev != nil && p.MarketingStatus == "" {
		// A read without the marketing field says nothing about opt-out.
		rec.OptedOut = prev.OptedOut
	}
	labels := make([]string, 0, len(p.LabelIDs))
	for _, l := range p.LabelIDs {
		labels = append(labels, id(l))
	}
	if len(labels) > 0 {
		rec.LifecycleStage = labels[0]
		rec.Properties["label_ids"] = strings.Join(labels, ",")
	}
	if prev != nil && prev.Properties[statusKey] != "" {
		rec.Properties[statusKey] = prev.Properties[statusKey]
	}
	if len(o.Config.DisplayProperties) > 0 {
		idx := s.fieldIndex(ctx, o)
		for _, code := range o.Config.DisplayProperties {
			if v, ok := p.CustomFields[code]; ok {
				if text := fieldValue(v, idx[code]); text != "" {
					rec.Properties[code] = text
				}
			}
		}
	}
	if rec.CompanyExternalID != "" {
		if prev != nil && prev.CompanyExternalID == rec.CompanyExternalID && prev.CompanyName != "" {
			rec.CompanyName, rec.CompanyDomain = prev.CompanyName, prev.CompanyDomain
		} else if org, err := o.Client.GetOrganization(ctx, *p.OrgID); err == nil {
			rec.CompanyName = org.Name
			rec.CompanyDomain = websiteDomain(org.Website)
		}
	}
	if err := s.d.Repo.UpsertContactRecord(ctx, rec); err != nil {
		return nil, nil, err
	}
	return rec, prev, nil
}

func websiteDomain(w string) string {
	w = strings.ToLower(strings.TrimSpace(w))
	w = strings.TrimPrefix(strings.TrimPrefix(w, "https://"), "http://")
	w = strings.TrimPrefix(w, "www.")
	host, _, _ := strings.Cut(w, "/")
	return host
}

func (s *Service) contactLink(contactID uuid.UUID) string {
	if s.d.AppURL == "" {
		return ""
	}
	return strings.TrimRight(s.d.AppURL, "/") + "/app/contacts?contact=" + contactID.String()
}

// ContactView is the Pipedrive side of a contact for the drawer and inbox panel.
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
	out.URL = o.personURL(rec.ExternalID)
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
		out.LifecycleStage = &models.CRMOption{Value: rec.LifecycleStage, Label: s.labelName(ctx, o, rec.LifecycleStage)}
	}
	if rec.CompanyExternalID != "" {
		out.Company = &models.CRMCompanyRef{ExternalID: rec.CompanyExternalID, Name: rec.CompanyName, Domain: rec.CompanyDomain,
			URL: o.orgURL(rec.CompanyExternalID)}
	}
	if len(o.Config.DisplayProperties) > 0 {
		idx := s.fieldIndex(ctx, o)
		for _, code := range o.Config.DisplayProperties {
			label := code
			if f := idx[code]; f != nil {
				label = f.FieldName
			}
			out.Properties = append(out.Properties, models.CRMPropertyView{Name: code, Label: label, Value: rec.Properties[code]})
		}
	}
	return out, nil
}

// LinkContact finds (or, when allowed, creates) the person in Pipedrive and
// pulls their deals, activities and notes into Warmbly.
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
			"This contact is not in Pipedrive, and creating people is turned off in the Pipedrive settings.")
	}
	if err := s.refreshContact(ctx, o, contactID, ext, true); err != nil {
		return nil, s.userError(ctx, o, err)
	}
	return s.ContactView(ctx, orgID, contactID)
}

// RefreshContact pulls one contact's Pipedrive side now. Debounced, since
// Pipedrive meters API use per company per day: the person is re-read at most
// every two minutes, and its deals, activities and notes every six hours
// (webhooks keep them current in between).
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
		contacts, cerr := s.d.Contacts.GetByIDsAndOrganization(ctx, orgID, []uuid.UUID{contactID})
		if cerr != nil || len(contacts) == 0 {
			return nil, errx.New(errx.NotFound, "contact not found")
		}
		if !s.debounce(ctx, "lookup:"+contactID.String(), 15*time.Minute) {
			return s.ContactView(ctx, orgID, contactID)
		}
		personID, serr := o.Client.FindPersonByEmail(ctx, strings.ToLower(strings.TrimSpace(contacts[0].Email)))
		if serr != nil {
			return nil, s.userError(ctx, o, serr)
		}
		if personID == 0 {
			return s.ContactView(ctx, orgID, contactID)
		}
		if err := s.refreshContact(ctx, o, contactID, id(personID), true); err != nil {
			return nil, s.userError(ctx, o, err)
		}
		return s.ContactView(ctx, orgID, contactID)
	}
	if !s.debounce(ctx, "refresh:"+contactID.String(), 2*time.Minute) {
		return s.ContactView(ctx, orgID, contactID)
	}
	deep := s.debounce(ctx, "deep:"+contactID.String(), 6*time.Hour)
	if err := s.refreshContact(ctx, o, contactID, rec.ExternalID, deep); err != nil {
		return nil, s.userError(ctx, o, err)
	}
	return s.ContactView(ctx, orgID, contactID)
}

// UpdateContactRecord writes owner or label to Pipedrive and the mirror.
func (s *Service) UpdateContactRecord(ctx context.Context, orgID, contactID uuid.UUID, upd *models.UpdateCRMContact) (*models.CRMContactView, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	ext, rec, err := s.ensureContact(ctx, o, contactID)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	if ext == "" {
		return nil, errx.NewWithIdentifier(errx.NotFound, "crm_contact_missing", "This contact is not in Pipedrive yet.")
	}
	body := map[string]any{}
	if upd.OwnerExternalID != nil {
		owner := extID(*upd.OwnerExternalID)
		if owner == 0 {
			return nil, errx.New(errx.BadRequest, "choose a Pipedrive user as the owner")
		}
		body["owner_id"] = owner
	}
	if upd.LifecycleStage != nil {
		next := extID(*upd.LifecycleStage)
		if next == 0 && *upd.LifecycleStage != "" {
			return nil, errx.New(errx.BadRequest, "unknown label")
		}
		// The panel shows one label; changing it replaces that one and keeps
		// the person's others.
		var current []string
		if rec != nil {
			current = splitIDs(rec.Properties["label_ids"])
		}
		labels := []int64{}
		if next != 0 {
			labels = append(labels, next)
		}
		for i, l := range current {
			if i == 0 || extID(l) == next {
				continue
			}
			labels = append(labels, extID(l))
		}
		body["label_ids"] = labels
	}
	if len(body) == 0 {
		return s.ContactView(ctx, orgID, contactID)
	}
	if _, err := o.Client.UpdatePerson(ctx, extID(ext), body); err != nil {
		return nil, s.userError(ctx, o, err)
	}
	p, err := o.Client.GetPerson(ctx, extID(ext))
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	if _, _, err := s.storeContactRecord(ctx, o, contactID, p); err != nil {
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
	ok, err := s.d.Cache.SetNX(ctx, "pipedrive:db:"+key, 1, ttl).Result()
	return err != nil || ok
}

func (s *Service) marked(ctx context.Context, key string) bool {
	if s.d.Cache == nil {
		return false
	}
	n, err := s.d.Cache.Exists(ctx, key).Result()
	return err == nil && n > 0
}

func (s *Service) mark(ctx context.Context, key string, ttl time.Duration) {
	if s.d.Cache != nil {
		s.d.Cache.Set(ctx, key, 1, ttl)
	}
}

// newLabels are the labels in rec that prev did not have.
func newLabels(prev, rec *models.CRMContactRecord) []string {
	now := splitIDs(rec.Properties["label_ids"])
	if prev == nil {
		return now
	}
	before := splitIDs(prev.Properties["label_ids"])
	var out []string
	for _, l := range now {
		if !slices.Contains(before, l) {
			out = append(out, l)
		}
	}
	return out
}

func toIDs(vs []string) []int64 {
	out := make([]int64, 0, len(vs))
	for _, v := range vs {
		if n := extID(v); n != 0 {
			out = append(out, n)
		}
	}
	return out
}

func splitIDs(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	d := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if freeMailDomains[d] {
		return ""
	}
	return d
}

// freeMailDomains never become organizations.
var freeMailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true, "outlook.com": true, "hotmail.com": true,
	"live.com": true, "icloud.com": true, "me.com": true, "aol.com": true, "proton.me": true, "protonmail.com": true,
	"gmx.com": true, "gmx.de": true, "mail.com": true, "yandex.com": true, "zoho.com": true, "msn.com": true,
}
