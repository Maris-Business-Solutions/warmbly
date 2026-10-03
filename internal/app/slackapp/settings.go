package slackapp

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

const (
	channelCacheTTL  = time.Minute
	channelScanPages = 10
	channelListMax   = 200
)

var channelRef = regexp.MustCompile(`^([CG][A-Z0-9]{2,}|#[a-z0-9][a-z0-9._-]{0,79})$`)

// workspaceConnection is the org's Slack connection for the dashboard,
// preferring a usable one but showing one that needs reconnecting.
func (s *Service) workspaceConnection(ctx context.Context, orgID uuid.UUID) (*models.IntegrationConnection, error) {
	if c, err := s.integ.SlackConnection(ctx, orgID); err != nil || c != nil {
		return c, err
	}
	conns, err := s.integ.ListConnections(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].Provider == models.IntegrationSlack {
			return &conns[i], nil
		}
	}
	return nil, nil
}

// missingScopes lists required bot scopes the install was not granted.
func missingScopes(granted []string) []string {
	out := []string{}
	for _, sc := range integration.SlackBotScopes {
		if !slices.Contains(granted, sc) {
			out = append(out, sc)
		}
	}
	return out
}

// Status is the dashboard's Slack panel. Links are listed only for members
// who manage settings; everyone sees their own.
func (s *Service) Status(ctx context.Context, orgID, userID uuid.UUID, canManage bool) (*models.SlackStatus, *errx.Error) {
	st := &models.SlackStatus{
		AppConfigured:         s.integ.SlackOAuthConfigured(),
		InteractiveConfigured: s.integ.SlackOAuthConfigured() && s.Interactive(),
		MissingScopes:         []string{},
		Links:                 []models.SlackUserLink{},
	}
	conn, err := s.workspaceConnection(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if conn != nil {
		st.Connection = conn
		st.Settings = settingsFrom(conn)
		st.MissingScopes = missingScopes(conn.GrantedScopes)
	}
	link, err := s.repo.GetLinkForUser(ctx, orgID, userID)
	if err != nil {
		return nil, errx.InternalError()
	}
	st.MyLink = link
	if canManage {
		links, err := s.repo.ListLinks(ctx, orgID)
		if err != nil {
			return nil, errx.InternalError()
		}
		st.Links = links
	}
	return st, nil
}

// Channels lists channels the bot can see, filtered by q, capped at 200.
func (s *Service) Channels(ctx context.Context, orgID uuid.UUID, q string) ([]models.SlackChannel, bool, *errx.Error) {
	conn, err := s.integ.SlackConnection(ctx, orgID)
	if err != nil {
		return nil, false, errx.InternalError()
	}
	if conn == nil {
		return nil, false, ErrSlackNotConnected
	}
	all, xerr := s.allChannels(ctx, conn)
	if xerr != nil {
		return nil, false, xerr
	}
	q = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(q), "#"))
	out := make([]models.SlackChannel, 0, min(len(all), channelListMax))
	more := false
	for _, ch := range all {
		if q != "" && !strings.Contains(strings.ToLower(ch.Name), q) {
			continue
		}
		if len(out) == channelListMax {
			more = true
			break
		}
		out = append(out, ch)
	}
	return out, more, nil
}

// allChannels pages conversations.list once a minute per connection, since
// it is one of Slack's slower-limited methods.
func (s *Service) allChannels(ctx context.Context, conn *models.IntegrationConnection) ([]models.SlackChannel, *errx.Error) {
	s.chanMu.Lock()
	if c, ok := s.chanList[conn.ID]; ok && time.Since(c.at) < channelCacheTTL {
		s.chanMu.Unlock()
		return c.list, nil
	}
	s.chanMu.Unlock()

	token, err := s.integ.SlackBotToken(ctx, conn.OrganizationID, conn.ID)
	if err != nil {
		return nil, ErrSlackNotConnected
	}
	var list []models.SlackChannel
	cursor := ""
	for page := 0; page < channelScanPages; page++ {
		chans, next, err := s.client.ListChannels(ctx, token, cursor)
		if err != nil {
			if len(list) > 0 {
				break
			}
			if IsAPIError(err, "invalid_auth", "token_revoked", "account_inactive", "not_authed") {
				return nil, ErrSlackNotConnected
			}
			return nil, errx.NewPublic(errx.ServiceUnavailable, "Slack did not return the channel list. Try again in a minute.")
		}
		for _, c := range chans {
			if c.IsArchived {
				continue
			}
			list = append(list, models.SlackChannel{ID: c.ID, Name: c.Name, IsPrivate: c.IsPrivate, IsMember: c.IsMember})
		}
		if next == "" {
			break
		}
		cursor = next
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	s.chanMu.Lock()
	s.chanList[conn.ID] = cachedChannels{at: time.Now(), list: list}
	s.chanMu.Unlock()
	return list, nil
}

// knownCategory reports a notification category the preferences know.
func knownCategory(c models.NotificationCategory) bool {
	return models.DefaultNotificationPreferences().CategoryPref(c) != models.CategoryPref{}
}

func validChannel(ch string) bool {
	return ch == "" || channelRef.MatchString(ch)
}

// validateSettings normalizes a settings write and refuses unknown values.
func validateSettings(in models.SlackSettings) (models.SlackSettings, *errx.Error) {
	out := models.SlackSettings{
		Channel:           strings.TrimSpace(in.Channel),
		AssistantDisabled: in.AssistantDisabled,
		AssistantDMOnly:   in.AssistantDMOnly,
		InboxChannel:      strings.TrimSpace(in.InboxChannel),
		InboxScope:        strings.TrimSpace(in.InboxScope),
	}
	if !validChannel(out.Channel) {
		return out, errx.New(errx.BadRequest, "channel must be a Slack channel id or #name")
	}
	if !validChannel(out.InboxChannel) {
		return out, errx.New(errx.BadRequest, "inbox_channel must be a Slack channel id or #name")
	}
	switch out.InboxScope {
	case "", models.SlackInboxScopeReplies, models.SlackInboxScopeAll:
	default:
		return out, errx.New(errx.BadRequest, "inbox_scope must be replies or all")
	}
	for cat, ch := range in.Routes {
		if !knownCategory(cat) {
			return out, errx.New(errx.BadRequest, "unknown notification category: "+string(cat))
		}
		ch = strings.TrimSpace(ch)
		if ch == "" {
			continue
		}
		if !validChannel(ch) {
			return out, errx.New(errx.BadRequest, "routes must map to Slack channel ids or #names")
		}
		if out.Routes == nil {
			out.Routes = map[models.NotificationCategory]string{}
		}
		out.Routes[cat] = ch
	}
	return out, nil
}

// UpdateSettings validates and stores the workspace's Slack settings.
func (s *Service) UpdateSettings(ctx context.Context, orgID uuid.UUID, in models.SlackSettings) (*models.SlackSettings, *errx.Error) {
	st, xerr := validateSettings(in)
	if xerr != nil {
		return nil, xerr
	}
	conn, err := s.workspaceConnection(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if conn == nil {
		return nil, ErrSlackNotConnected
	}
	if xerr := s.checkInboxChannel(ctx, conn, st.InboxChannel); xerr != nil {
		return nil, xerr
	}
	updated, err := s.integ.UpdateSlackSettings(ctx, orgID, conn.ID, st)
	if err != nil || updated == nil {
		return nil, errx.InternalError()
	}
	out := settingsFrom(updated)
	return &out, nil
}

// checkInboxChannel keeps prospects' mail out of channels another company can
// read: the inbox channel must be a known channel that is not Slack Connect.
func (s *Service) checkInboxChannel(ctx context.Context, conn *models.IntegrationConnection, channel string) *errx.Error {
	if channel == "" {
		return nil
	}
	if strings.HasPrefix(channel, "#") {
		return errx.New(errx.BadRequest, "Pick the inbox channel from the list.")
	}
	token, err := s.integ.SlackBotToken(ctx, conn.OrganizationID, conn.ID)
	if err != nil {
		return errx.New(errx.ServiceUnavailable, "Slack could not be reached. Try again in a moment.")
	}
	info, err := s.client.ConversationInfo(ctx, token, channel)
	if err != nil {
		return errx.New(errx.BadRequest, "Warmbly can't see that channel. Invite the Warmbly app to it, then pick it again.")
	}
	if info.IsExtShared {
		return errx.New(errx.BadRequest, "The inbox channel can't be shared with another company through Slack Connect.")
	}
	return nil
}
