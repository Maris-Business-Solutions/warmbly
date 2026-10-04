package models

import (
	"time"

	"github.com/google/uuid"
)

// OAuth2 authorization server domain types. Apps register as OAuth clients;
// users grant them scoped access via the authorization-code flow (client secret
// required, PKCE optional); the issued access token carries an API-permission
// bitmask (Scopes) and authenticates API calls through the same gates as an API key.

type OAuthAppStatus string

const (
	OAuthAppActive   OAuthAppStatus = "active"
	OAuthAppDisabled OAuthAppStatus = "disabled"
)

// Credential prefixes mirror the api_keys `wmbly_` convention so a leaked token
// is greppable and self-describing.
const (
	OAuthClientIDPrefix     = "wmcid_"
	OAuthClientSecretPrefix = "wmcs_"
	OAuthAccessTokenPrefix  = "wmat_"
	OAuthRefreshTokenPrefix = "wmrt_"
	OAuthCodePrefix         = "wmac_"
)

// Lifetimes. The authorization code is single-use and short; access tokens are
// short-lived; refresh tokens are long-lived and rotate on every exchange.
const (
	OAuthAuthorizationCodeTTL = 10 * time.Minute
	OAuthAccessTokenTTL       = time.Hour
	OAuthRefreshTokenTTL      = 90 * 24 * time.Hour
)

// OAuthApplication is a registered third-party OAuth client.
type OAuthApplication struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	CreatedBy        uuid.UUID `json:"created_by"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	LogoURL          string    `json:"logo_url"`
	WebsiteURL       string    `json:"website_url"`
	ClientID         string    `json:"client_id"`
	ClientSecretHash string    `json:"-"`
	RedirectURIs     []string  `json:"redirect_uris"`
	// AllowedWebhookDomains constrains the host of any webhook endpoint this app
	// registers (subdomain-inclusive: ".acme.com" matches subdomains, "acme.com"
	// is exact). Empty = the app cannot register webhooks.
	AllowedWebhookDomains []string `json:"allowed_webhook_domains"`
	// App-level webhook subscription (the GitHub/Slack-app model). When WebhookURL
	// is set, every org that authorizes this app automatically receives the
	// subscribed events at WebhookURL, scoped to the permissions that org granted,
	// signed with WebhookSecret. WebhookEvents empty = all non-firehose events the
	// grant's scopes allow. The URL host must be within AllowedWebhookDomains.
	WebhookURL    string         `json:"webhook_url"`
	WebhookEvents []string       `json:"webhook_events"`
	WebhookSecret string         `json:"-"`
	Scopes        uint64         `json:"scopes"`
	Status        OAuthAppStatus `json:"status"`
	// IsPublic marks a client that authenticates with PKCE and no secret (native
	// apps, and every MCP client registered via Dynamic Client Registration). The
	// token endpoint accepts no secret for these and PKCE is mandatory.
	IsPublic bool `json:"is_public"`
	// DynamicallyRegistered is true for clients created at runtime via the RFC 7591
	// registration endpoint (no owning org/user until a human grants access).
	DynamicallyRegistered bool `json:"dynamically_registered"`
	// SuspendedAt is an operator's suspension; the owner cannot lift it.
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason string     `json:"suspended_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Usable reports whether the app may authorize, issue or accept tokens.
func (a *OAuthApplication) Usable() bool {
	return a.Status == OAuthAppActive && a.SuspendedAt == nil
}

// OAuthApplicationWithSecret is returned exactly once, on create or secret
// rotation; the plaintext secret is never stored or shown again.
type OAuthApplicationWithSecret struct {
	OAuthApplication
	ClientSecret string `json:"client_secret,omitempty"`
}

// OAuthApplicationWrite is the create/update payload from the developer UI.
// WebhookSecret is server-generated (returned once on set/rotate), never set here.
type OAuthApplicationWrite struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	LogoURL               string   `json:"logo_url"`
	WebsiteURL            string   `json:"website_url"`
	RedirectURIs          []string `json:"redirect_uris"`
	AllowedWebhookDomains []string `json:"allowed_webhook_domains"`
	WebhookURL            string   `json:"webhook_url"`
	WebhookEvents         []string `json:"webhook_events"`
	Scopes                uint64   `json:"scopes"`
}

// OAuthGrantOrg is one org that has an active grant for an app, with the union
// of scopes that org granted (across its users). Drives app-webhook materialization.
type OAuthGrantOrg struct {
	OrgID  uuid.UUID
	Scopes uint64
}

// OAuthAuthorizationCode is a single-use code bound to a PKCE challenge and the
// exact scopes/redirect the user consented to.
type OAuthAuthorizationCode struct {
	ID                  uuid.UUID
	CodeHash            string
	ApplicationID       uuid.UUID
	OrganizationID      uuid.UUID
	UserID              uuid.UUID
	RedirectURI         string
	Scopes              uint64
	CodeChallenge       string
	CodeChallengeMethod string
	UsedAt              *time.Time
	ExpiresAt           time.Time
	CreatedAt           time.Time
}

// OAuthAccessGrant is an issued access+refresh token pair (tokens stored hashed).
type OAuthAccessGrant struct {
	ID               uuid.UUID  `json:"id"`
	ApplicationID    uuid.UUID  `json:"application_id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	UserID           uuid.UUID  `json:"user_id"`
	Scopes           uint64     `json:"scopes"`
	AccessTokenHash  string     `json:"-"`
	RefreshTokenHash string     `json:"-"`
	AccessExpiresAt  time.Time  `json:"access_expires_at"`
	RefreshExpiresAt *time.Time `json:"refresh_expires_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// OAuthAuthorizedApp is one row in a user's "apps you've authorized" list (a
// grant joined to its application's display fields).
type OAuthAuthorizedApp struct {
	ApplicationID uuid.UUID  `json:"application_id"`
	Name          string     `json:"name"`
	LogoURL       string     `json:"logo_url"`
	WebsiteURL    string     `json:"website_url"`
	Scopes        uint64     `json:"scopes"`
	AuthorizedAt  time.Time  `json:"authorized_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
}

// OAuthDeveloperBlock stops a workspace or a person from registering or
// publishing OAuth apps. Exactly one of OrganizationID and UserID is set.
type OAuthDeveloperBlock struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   *uuid.UUID `json:"organization_id,omitempty"`
	OrganizationName string     `json:"organization_name,omitempty"`
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	UserEmail        string     `json:"user_email,omitempty"`
	Reason           string     `json:"reason"`
	BlockedByEmail   string     `json:"blocked_by_email,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// OAuthDeveloperAccess is what a workspace may do with OAuth apps right now.
type OAuthDeveloperAccess struct {
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason,omitempty"`
}

// AdminOAuthApp is an app as the operator's moderation list shows it.
type AdminOAuthApp struct {
	ID               uuid.UUID       `json:"id"`
	OrganizationID   uuid.UUID       `json:"organization_id"`
	OrganizationName string          `json:"organization_name"`
	CreatedBy        uuid.UUID       `json:"created_by"`
	CreatedByEmail   string          `json:"created_by_email"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	LogoURL          string          `json:"logo_url"`
	WebsiteURL       string          `json:"website_url"`
	ClientID         string          `json:"client_id"`
	RedirectURIs     []string        `json:"redirect_uris"`
	WebhookURL       string          `json:"webhook_url"`
	Permissions      []APIPermission `json:"permissions"`
	Status           OAuthAppStatus  `json:"status"`
	IsPublic         bool            `json:"is_public"`
	SuspendedAt      *time.Time      `json:"suspended_at,omitempty"`
	SuspendedReason  string          `json:"suspended_reason,omitempty"`
	Installs         int             `json:"installs"`
	ListingSlug      string          `json:"listing_slug,omitempty"`
	ListingStatus    string          `json:"listing_status,omitempty"`
	OrgBlocked       bool            `json:"org_blocked"`
	CreatorBlocked   bool            `json:"creator_blocked"`
	CreatedAt        time.Time       `json:"created_at"`
}

// AdminOAuthAppSearch filters the moderation list. Status is active,
// disabled or suspended.
type AdminOAuthAppSearch struct {
	Q      string `form:"q"`
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"-"`
}

type AdminOAuthAppsResult struct {
	Data       []AdminOAuthApp `json:"data"`
	Pagination Pagination      `json:"pagination"`
}

// CreateOAuthDeveloperBlock is the operator's block request: a workspace or a
// person, a reason, and whether to suspend the apps they already have.
type CreateOAuthDeveloperBlock struct {
	OrganizationID *uuid.UUID `json:"organization_id"`
	UserID         *uuid.UUID `json:"user_id"`
	Reason         string     `json:"reason"`
	SuspendApps    bool       `json:"suspend_apps"`
}
