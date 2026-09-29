package models

import (
	"time"

	"github.com/google/uuid"
)

// CampaignLeadCC is a contact copied on every email one campaign sends one
// lead, so several people at one company share a single thread (issue #731).
type CampaignLeadCC struct {
	ContactID uuid.UUID `json:"contact_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Company   string    `json:"company,omitempty"`
	// Status says whether the next email copies them: one of the
	// LeadCCStatus constants. Anything but "active" is left off.
	Status    string     `json:"status"`
	BouncedAt *time.Time `json:"bounced_at,omitempty"`
}

// Copied reports whether the next email to the lead carries this address.
func (c CampaignLeadCC) Copied() bool { return c.Status == LeadCCStatusActive }

// Why a copy is or is not on the next email, in the order they are decided.
const (
	LeadCCStatusActive = "active"
	// LeadCCStatusUnsubscribed is an opted-out or suppressed address.
	LeadCCStatusUnsubscribed = "unsubscribed"
	// LeadCCStatusBounced is an address that bounced on this thread or on any
	// campaign email of its own.
	LeadCCStatusBounced = "bounced"
	// LeadCCStatusUndeliverable is an address verification refused, under the
	// same rule the campaign applies to its leads.
	LeadCCStatusUndeliverable = "undeliverable"
)

// LeadHoldSourceCC holds a contact's own lead while they are copied on another
// lead's thread in the same campaign, so they never get two sequences. Written
// only by the campaign_lead_cc triggers (migration 000230).
const LeadHoldSourceCC = "cc"

// SetCampaignLeadCC replaces the contacts copied on one lead. An empty list
// removes them all.
type SetCampaignLeadCC struct {
	ContactIDs []string `json:"contact_ids"`
}

// CampaignLeadCCSuggestion is a contact who looks like a colleague of the
// lead, offered first when picking who to copy. Reason is "company" when the
// company names match and "domain" when only the email domain does.
type CampaignLeadCCSuggestion struct {
	ContactID uuid.UUID `json:"contact_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Company   string    `json:"company,omitempty"`
	Reason    string    `json:"reason"`
}
