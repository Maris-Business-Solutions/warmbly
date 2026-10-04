package pipedrive

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// Person keys a Warmbly field can map to besides custom field codes.
const (
	keyFirstName = "first_name"
	keyLastName  = "last_name"
	keyName      = "name"
	keyPhone     = "phone"
	keyOrgName   = "org_name"
)

var standardPersonKeys = []models.CRMProperty{
	{Name: keyFirstName, Label: "First name", Type: "varchar", GroupName: "Person"},
	{Name: keyLastName, Label: "Last name", Type: "varchar", GroupName: "Person"},
	{Name: keyName, Label: "Name", Type: "varchar", GroupName: "Person"},
	{Name: keyPhone, Label: "Phone", Type: "phone", GroupName: "Person"},
	{Name: keyOrgName, Label: "Organization", Type: "org", GroupName: "Person"},
}

// mappableTypes are the custom field types a Warmbly text value can be
// written to as is.
var mappableTypes = map[string]bool{"varchar": true, "varchar_auto": true, "text": true, "phone": true, "double": true, "date": true}

// personFields is the company's person schema, cached for ten minutes.
func (s *Service) personFields(ctx context.Context, o *org) ([]Field, error) {
	key := "pipedrive:pfields:" + o.Company
	if s.d.Cache != nil {
		if raw, err := s.d.Cache.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var out []Field
			if json.Unmarshal(raw, &out) == nil {
				return out, nil
			}
		}
	}
	fields, err := o.Client.PersonFields(ctx)
	if err != nil {
		return nil, err
	}
	if s.d.Cache != nil {
		_ = s.d.Cache.SetJSON(ctx, key, fields, 10*time.Minute)
	}
	return fields, nil
}

func (s *Service) fieldIndex(ctx context.Context, o *org) map[string]*Field {
	out := map[string]*Field{}
	fields, err := s.personFields(ctx, o)
	if err != nil {
		return out
	}
	for i := range fields {
		out[fields[i].FieldCode] = &fields[i]
	}
	return out
}

// labels are the company's person labels: Pipedrive's stand-in for a
// lifecycle stage ("Customer", "Hot lead").
func (s *Service) labels(ctx context.Context, o *org) []models.CRMOption {
	fields, err := s.personFields(ctx, o)
	if err != nil {
		return nil
	}
	for _, f := range fields {
		if f.FieldCode != "label_ids" && f.FieldCode != "label" {
			continue
		}
		out := make([]models.CRMOption, 0, len(f.Options))
		for _, op := range f.Options {
			out = append(out, models.CRMOption{Value: id(op.ID), Label: op.Label})
		}
		return out
	}
	return nil
}

func (s *Service) labelName(ctx context.Context, o *org, value string) string {
	for _, l := range s.labels(ctx, o) {
		if l.Value == value {
			return l.Label
		}
	}
	return value
}

// labelByName finds a label id by its name, for seeding defaults.
func labelByName(labels []models.CRMOption, names ...string) string {
	for _, n := range names {
		for _, l := range labels {
			if strings.EqualFold(strings.TrimSpace(l.Label), n) {
				return l.Value
			}
		}
	}
	return ""
}

// ensureFields makes sure the Warmbly person fields exist and returns their
// codes. Best-effort: a company where the connected user may not add fields
// still syncs, without the fields. Only a complete set is remembered.
func (s *Service) ensureFields(ctx context.Context, o *org) warmblyFieldSet {
	key := "pipedrive:wfields:" + o.Company
	if s.d.Cache != nil {
		if raw, err := s.d.Cache.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var set warmblyFieldSet
			// A partial set is kept too, for the shorter time a failure is
			// cached, so a missing permission is not retried on every event.
			if json.Unmarshal(raw, &set) == nil && set.Codes != nil {
				return set
			}
		}
	}
	set := warmblyFieldSet{Codes: map[string]string{}, Options: map[string]int64{}}
	fields, err := o.Client.PersonFields(ctx)
	if err != nil {
		return set
	}
	byName := map[string]Field{}
	for _, f := range fields {
		if f.IsCustomField {
			byName[strings.ToLower(f.FieldName)] = f
		}
	}
	var failure error
	for _, def := range warmblyFields {
		f, ok := byName[strings.ToLower(def.Name)]
		if !ok {
			created, cerr := o.Client.CreatePersonField(ctx, def.Name, def.Type, def.Options)
			if cerr != nil {
				failure = cerr
				continue
			}
			f = *created
		}
		set.Codes[def.Name] = f.FieldCode
		if def.Name == fieldStatus {
			for _, op := range f.Options {
				set.Options[op.Label] = op.ID
			}
		}
	}
	if failure != nil {
		log.Info().Err(failure).Str("org_id", o.ID.String()).Msg("pipedrive: could not add the Warmbly person fields")
		s.recordRun(ctx, o, "fields", nil, failure)
	} else {
		s.recordRun(ctx, o, "fields", ptrNow(), nil)
	}
	if s.d.Cache != nil {
		ttl := 6 * time.Hour
		if failure != nil {
			ttl = 30 * time.Minute
		}
		_ = s.d.Cache.SetJSON(ctx, key, set, ttl)
		s.d.Cache.Del(ctx, "pipedrive:pfields:"+o.Company)
	}
	return set
}

func ptrNow() *time.Time {
	now := time.Now()
	return &now
}

// warmblyValues turns Warmbly field values into custom_fields, skipping any
// field the company does not have.
func (s *Service) warmblyValues(ctx context.Context, o *org, values map[string]string) map[string]any {
	if !o.Config.WriteProperties || len(values) == 0 {
		return nil
	}
	set := s.ensureFields(ctx, o)
	out := map[string]any{}
	for name, v := range values {
		code := set.Codes[name]
		if code == "" {
			continue
		}
		switch name {
		case fieldStatus:
			if opt, ok := set.Options[v]; ok {
				out[code] = opt
			}
		default:
			out[code] = v
		}
	}
	return out
}

// personProp reads one mapped key off a person.
func personProp(p *Person, key, orgName string) string {
	switch key {
	case keyFirstName:
		return p.FirstName
	case keyLastName:
		return p.LastName
	case keyName:
		return p.Name
	case keyPhone:
		return p.Phone()
	case keyOrgName:
		return orgName
	}
	if v, ok := p.CustomFields[key]; ok {
		return fieldValue(v, nil)
	}
	return ""
}
