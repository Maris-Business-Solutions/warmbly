package slackapp

import (
	"github.com/warmbly/warmbly/internal/models"
)

// resolveLink picks the connection a Slack member acts through. A link to one
// of the team's usable connections decides the org; without one the first
// connection hosts the link prompt and linked is false.
func resolveLink(conns []models.IntegrationConnection, link *models.SlackUserLink) (*models.IntegrationConnection, bool) {
	if len(conns) == 0 {
		return nil, false
	}
	if link != nil {
		for i := range conns {
			if conns[i].ID == link.ConnectionID && conns[i].OrganizationID == link.OrganizationID {
				return &conns[i], true
			}
		}
	}
	return &conns[0], false
}

type msgRoute int

const (
	routeIgnore msgRoute = iota
	routeAgent
	routeRefuseDisabled
	routeRefuseDMOnly
	routeRefuseExternal
	routeRefuseNotOwner
)

// routeFacts is what is known about a message before the assistant runs.
type routeFacts struct {
	DM bool
	// Mention is an explicit request outside a DM: an @mention, a shortcut or
	// a button.
	Mention bool
	// MentionsBot marks a plain message event that also arrives as app_mention.
	MentionsBot bool
	InThread    bool
	// ThreadMapped: the thread already belongs to an assistant session.
	ThreadMapped bool
	// OwnerIsAuthor: the session belongs to the linked author of the message.
	OwnerIsAuthor bool
	// SameOrg: the thread's session belongs to the author's Warmbly workspace.
	SameOrg   bool
	ExtShared bool
	Settings  models.SlackSettings
}

// decideRoute says whether a message reaches the assistant. Channel messages
// that are not mentions only continue a thread the author already owns, and
// a mention in a teammate's thread takes it over; explicit requests that are
// refused get told why, passive ones are ignored.
func decideRoute(f routeFacts) msgRoute {
	explicit := f.DM || f.Mention
	if !explicit {
		if f.MentionsBot || !f.InThread || !f.ThreadMapped || !f.OwnerIsAuthor {
			return routeIgnore
		}
	}
	refuse := func(r msgRoute) msgRoute {
		if explicit {
			return r
		}
		return routeIgnore
	}
	switch {
	case f.ExtShared && !f.DM:
		return refuse(routeRefuseExternal)
	case f.Settings.AssistantDisabled:
		return refuse(routeRefuseDisabled)
	case f.Settings.AssistantDMOnly && !f.DM:
		return refuse(routeRefuseDMOnly)
	case f.ThreadMapped && !f.OwnerIsAuthor && !(f.Mention && f.SameOrg):
		return refuse(routeRefuseNotOwner)
	}
	return routeAgent
}

// refusalText is what an explicit request is told when it is refused.
func refusalText(r msgRoute) string {
	switch r {
	case routeRefuseExternal:
		return "Warmbly does not answer in channels shared with other organizations. Message me directly instead."
	case routeRefuseDisabled:
		return "The Warmbly assistant is turned off for this Slack workspace. A Warmbly admin can turn it on in Settings > Integrations > Slack."
	case routeRefuseDMOnly:
		return "In this workspace Warmbly answers in direct messages only. Message me directly or open the Warmbly app."
	case routeRefuseNotOwner:
		return "This thread is someone else's conversation with Warmbly. Start a new thread to ask your own question."
	}
	return ""
}
