package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
)

// OAuthManager owns the OAuth 2.0 authorization-code machinery for every
// provider that supports it. Client credentials are read from the environment
// at construction (one app per provider, registered in that provider's
// developer console). A provider with no credentials configured is reported
// as not-Configured so the dashboard renders it as "coming soon" rather than a
// dead Connect button — the framework lights up the moment real credentials
// are supplied, no code change required.
//
// This mirrors the mailbox OAuth flow in internal/app/email/oauth.go: start →
// provider popup → callback page postMessages code+state → finish exchanges and
// persists encrypted tokens.
type OAuthManager struct {
	redirectURL string
	providers   map[models.IntegrationProvider]*oauthProvider
	http        *http.Client
}

// identifyFunc resolves the connected external account (id + display name) and
// the scopes actually granted, given a fresh token.
type identifyFunc func(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (extID, extName string, scopes []string, err error)

type oauthProvider struct {
	provider models.IntegrationProvider
	config   *oauth2.Config
	scopes   []string
	// optional scopes are requested but not required, so an account whose plan
	// lacks one can still connect.
	optional []string
	usePKCE  bool
	identify identifyFunc
	// scopeSep overrides the space x/oauth2 joins scopes with (Slack wants commas).
	scopeSep string
}

// NewOAuthManager builds the provider registry from environment variables. For
// each provider it reads <PREFIX>_OAUTH_CLIENT_ID / <PREFIX>_OAUTH_CLIENT_SECRET
// (e.g. HUBSPOT_OAUTH_CLIENT_ID). The shared redirect/callback URL comes from
// INTEGRATIONS_OAUTH_REDIRECT_URL, else the backend's public URL
// (config.BackendPublicURL) + the callback path.
func NewOAuthManager() *OAuthManager {
	redirect := strings.TrimSpace(os.Getenv("INTEGRATIONS_OAUTH_REDIRECT_URL"))
	if redirect == "" {
		redirect = config.BackendPublicURL() + "/integrations/oauth/callback"
	}

	m := &OAuthManager{
		redirectURL: redirect,
		providers:   map[models.IntegrationProvider]*oauthProvider{},
		http:        &http.Client{Timeout: 15 * time.Second},
	}

	register := func(p models.IntegrationProvider, envPrefix string, ep oauth2.Endpoint, scopes []string, usePKCE bool, id identifyFunc) {
		clientID := strings.TrimSpace(os.Getenv(envPrefix + "_OAUTH_CLIENT_ID"))
		clientSecret := strings.TrimSpace(os.Getenv(envPrefix + "_OAUTH_CLIENT_SECRET"))
		op := &oauthProvider{provider: p, scopes: scopes, usePKCE: usePKCE, identify: id}
		if clientID != "" && clientSecret != "" {
			op.config = &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Endpoint:     ep,
				RedirectURL:  redirect,
				Scopes:       scopes,
			}
		}
		m.providers[p] = op
	}

	register(models.IntegrationHubSpot, "HUBSPOT", oauth2.Endpoint{
		AuthURL:  "https://app.hubspot.com/oauth/authorize",
		TokenURL: "https://api.hubapi.com/oauth/v1/token",
	}, HubSpotRequiredScopes, false, identifyHubSpot)
	m.providers[models.IntegrationHubSpot].optional = HubSpotOptionalScopes

	register(models.IntegrationSlack, "SLACK", oauth2.Endpoint{
		AuthURL:  "https://slack.com/oauth/v2/authorize",
		TokenURL: "https://slack.com/api/oauth.v2.access",
	}, SlackBotScopes, false, identifySlack)
	m.providers[models.IntegrationSlack].scopeSep = ","

	register(models.IntegrationGoogleSheets, "GOOGLE_SHEETS", oauth2.Endpoint{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
	}, []string{
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/userinfo.email",
	}, true, identifyGoogle)

	register(models.IntegrationPipedrive, "PIPEDRIVE", oauth2.Endpoint{
		AuthURL:  "https://oauth.pipedrive.com/oauth/authorize",
		TokenURL: "https://oauth.pipedrive.com/oauth/token",
	}, []string{"contacts:full", "deals:full"}, false, identifyPipedrive)

	register(models.IntegrationSalesforce, "SALESFORCE", oauth2.Endpoint{
		AuthURL:  "https://login.salesforce.com/services/oauth2/authorize",
		TokenURL: "https://login.salesforce.com/services/oauth2/token",
	}, []string{"api", "refresh_token", "id"}, true, identifySalesforce)

	return m
}

// HubSpotRequiredScopes is what CRM mode needs: contacts, companies, deals,
// owners, and the contact schema for the Warmbly property group.
var HubSpotRequiredScopes = []string{
	"oauth",
	"crm.objects.contacts.read", "crm.objects.contacts.write",
	"crm.objects.companies.read", "crm.objects.companies.write",
	"crm.objects.deals.read", "crm.objects.deals.write",
	"crm.objects.owners.read",
	"crm.schemas.contacts.read", "crm.schemas.contacts.write",
}

// HubSpotOptionalScopes unlock list import and reading logged email bodies;
// a portal without them still connects.
var HubSpotOptionalScopes = []string{"crm.lists.read", "sales-email-read"}

// SupportsOAuth reports whether the provider has an OAuth flow at all.
func (m *OAuthManager) SupportsOAuth(p models.IntegrationProvider) bool {
	_, ok := m.providers[p]
	return ok
}

// Configured reports whether the provider has client credentials wired.
func (m *OAuthManager) Configured(p models.IntegrationProvider) bool {
	op, ok := m.providers[p]
	return ok && op.config != nil
}

// RedirectURL is the shared OAuth callback every provider redirects to.
func (m *OAuthManager) RedirectURL() string { return m.redirectURL }

// Scopes returns the requested scopes for a provider (empty if none/unknown).
func (m *OAuthManager) Scopes(p models.IntegrationProvider) []string {
	if op, ok := m.providers[p]; ok {
		return op.scopes
	}
	return nil
}

// configFor returns the provider's OAuth config, pointed at loginHost for a
// provider whose authorization server varies per org (Salesforce sandboxes and
// My Domains). An empty host keeps the registered endpoint.
func (op *oauthProvider) configFor(loginHost string) *oauth2.Config {
	if op.config == nil || loginHost == "" || op.provider != models.IntegrationSalesforce {
		return op.config
	}
	cfg := *op.config
	cfg.Endpoint = oauth2.Endpoint{
		AuthURL:  "https://" + loginHost + "/services/oauth2/authorize",
		TokenURL: "https://" + loginHost + "/services/oauth2/token",
	}
	return &cfg
}

// SalesforceLoginHost resolves "production", "sandbox" or a My Domain to the
// host a Salesforce handshake runs against. Only Salesforce's own domains are
// accepted: the token endpoint receives the client secret.
func SalesforceLoginHost(in string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(in))
	switch v {
	case "", "production", "login.salesforce.com":
		return "login.salesforce.com", nil
	case "sandbox", "test.salesforce.com":
		return "test.salesforce.com", nil
	}
	v = strings.TrimPrefix(strings.TrimPrefix(v, "https://"), "http://")
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	if !strings.HasSuffix(v, ".my.salesforce.com") || len(v) > 200 {
		return "", errors.New("enter your My Domain, for example acme.my.salesforce.com")
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return "", errors.New("enter your My Domain, for example acme.my.salesforce.com")
		}
	}
	return v, nil
}

// salesforceInstanceSuffixes are the domains Salesforce serves an org's REST API
// from: My Domain and instance hosts, Government Cloud and the China region.
var salesforceInstanceSuffixes = []string{".salesforce.com", ".force.com", ".cloudforce.com", ".salesforce.mil", ".sfcrmproducts.cn"}

// SalesforceInstanceURL canonicalises an org's API host to https://host, and
// accepts only a Salesforce domain on the default port: the bearer token goes there.
func SalesforceInstanceURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" ||
		(u.Port() != "" && u.Port() != "443") || u.RawQuery != "" || u.Fragment != "" ||
		strings.Trim(u.Path, "/") != "" {
		return "", errors.New("salesforce instance url is not a salesforce host")
	}
	host := strings.ToLower(u.Hostname())
	if len(host) > 253 {
		return "", errors.New("salesforce instance url is not a salesforce host")
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return "", errors.New("salesforce instance url is not a salesforce host")
		}
	}
	for _, suffix := range salesforceInstanceSuffixes {
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) && !strings.Contains(host, "..") {
			return "https://" + host, nil
		}
	}
	return "", errors.New("salesforce instance url is not a salesforce host")
}

// AuthCodeURL builds the provider authorization URL. It returns the URL plus
// the PKCE verifier to persist (empty when the provider doesn't use PKCE).
func (m *OAuthManager) AuthCodeURL(p models.IntegrationProvider, state, loginHost string) (authURL, verifier string, err error) {
	op, ok := m.providers[p]
	if !ok || op.config == nil {
		return "", "", fmt.Errorf("oauth not configured for provider %s", p)
	}
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.ApprovalForce}
	if p == models.IntegrationSalesforce {
		// Salesforce reuses a live browser session; asking for a login lets the
		// member choose which org they authorize.
		opts = []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("prompt", "login consent")}
	}
	if op.scopeSep != "" && len(op.scopes) > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("scope", strings.Join(op.scopes, op.scopeSep)))
	}
	if len(op.optional) > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("optional_scope", strings.Join(op.optional, " ")))
	}
	if op.usePKCE {
		verifier = randomURLToken(32)
		sum := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])
		opts = append(opts,
			oauth2.SetAuthURLParam("code_challenge", challenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)
	}
	return op.configFor(loginHost).AuthCodeURL(state, opts...), verifier, nil
}

// Exchange swaps an authorization code for tokens and resolves the connected
// account identity.
func (m *OAuthManager) Exchange(ctx context.Context, p models.IntegrationProvider, code, verifier, loginHost string) (*models.IntegrationTokens, extAccount, error) {
	op, ok := m.providers[p]
	if !ok || op.config == nil {
		return nil, extAccount{}, fmt.Errorf("oauth not configured for provider %s", p)
	}
	var opts []oauth2.AuthCodeOption
	if op.usePKCE && verifier != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", verifier))
	}
	tok, err := op.configFor(loginHost).Exchange(ctx, code, opts...)
	if err != nil {
		return nil, extAccount{}, fmt.Errorf("token exchange failed: %w", err)
	}

	extID, extName, grantedScopes, idErr := "", "", []string(nil), error(nil)
	if op.identify != nil {
		extID, extName, grantedScopes, idErr = op.identify(ctx, m, tok)
		if idErr != nil {
			// Identity is best-effort: a connected token is still usable even
			// if the profile lookup hiccups. We just won't show the account name.
			grantedScopes = nil
		}
	}
	if len(grantedScopes) == 0 {
		grantedScopes = scopesFromToken(tok, op.scopes)
	}

	tokens := &models.IntegrationTokens{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Scopes:       grantedScopes,
	}
	if !tok.Expiry.IsZero() {
		exp := tok.Expiry.UTC()
		tokens.ExpiresAt = &exp
	}

	acct := extAccount{ID: extID, Name: extName}
	// Salesforce returns the org's API host as an "instance_url" extra; only a
	// Salesforce domain is kept, since the access token is sent there.
	if iu, ok := tok.Extra("instance_url").(string); ok {
		if v, verr := SalesforceInstanceURL(iu); verr == nil {
			acct.InstanceURL = v
		}
	}
	if p == models.IntegrationHubSpot {
		acct.UIDomain = hubspotUIDomain(ctx, m, tok.AccessToken)
	}
	if id, ok := tok.Extra("id").(string); ok {
		acct.IdentityURL = strings.TrimSpace(id)
	}
	if p == models.IntegrationSlack {
		if au, ok := tok.Extra("authed_user").(map[string]any); ok {
			acct.InstallerID, _ = au["id"].(string)
		}
	}
	return tokens, acct, nil
}

// RefreshIfNeeded returns a valid access token for the connection, refreshing
// via the stored refresh token when the access token is within 60s of expiry,
// or whenever force is set (Salesforce issues no expiry, so a refused session
// is the only signal). It reports whether the token was refreshed.
func (m *OAuthManager) RefreshIfNeeded(ctx context.Context, p models.IntegrationProvider, current models.IntegrationTokens, force bool, loginHost string) (models.IntegrationTokens, bool, error) {
	op, ok := m.providers[p]
	if !ok || op.config == nil {
		return current, false, fmt.Errorf("oauth not configured for provider %s", p)
	}
	stillValid := current.ExpiresAt == nil || time.Until(*current.ExpiresAt) > 60*time.Second
	if (stillValid && !force) || current.RefreshToken == "" {
		return current, false, nil
	}

	src := op.configFor(loginHost).TokenSource(ctx, &oauth2.Token{
		AccessToken:  current.AccessToken,
		RefreshToken: current.RefreshToken,
		Expiry:       time.Now().Add(-time.Minute),
	})
	tok, err := src.Token()
	if err != nil {
		return current, false, fmt.Errorf("token refresh failed: %w", err)
	}
	refreshed := models.IntegrationTokens{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Scopes:       current.Scopes,
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = current.RefreshToken // some providers omit it on refresh
	}
	if !tok.Expiry.IsZero() {
		exp := tok.Expiry.UTC()
		refreshed.ExpiresAt = &exp
	}
	if iu, ok := tok.Extra("instance_url").(string); ok {
		if v, verr := SalesforceInstanceURL(iu); verr == nil {
			refreshed.InstanceURL = v
		}
	}
	return refreshed, true, nil
}

// extAccount is the resolved external identity for a connection.
type extAccount struct {
	ID   string
	Name string
	// InstanceURL is the provider-specific API host returned at token-exchange
	// time (Salesforce's per-org domain). Empty for providers with a fixed host.
	InstanceURL string
	// UIDomain is the provider web app host for record links (HubSpot).
	UIDomain string
	// IdentityURL is Salesforce's /id/<org>/<user> URL for the connected user.
	IdentityURL string
	// InstallerID is the Slack member who approved the install (authed_user).
	InstallerID string
}

// --- identity resolvers -----------------------------------------------------

func identifyHubSpot(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (string, string, []string, error) {
	var out struct {
		HubID     int64    `json:"hub_id"`
		HubDomain string   `json:"hub_domain"`
		User      string   `json:"user"`
		Scopes    []string `json:"scopes"`
	}
	url := "https://api.hubapi.com/oauth/v1/access-tokens/" + tok.AccessToken
	if err := m.getJSON(ctx, url, "", &out); err != nil {
		return "", "", nil, err
	}
	name := out.HubDomain
	if name == "" {
		name = out.User
	}
	return fmt.Sprintf("%d", out.HubID), name, out.Scopes, nil
}

// hubspotUIDomain resolves the web app host for the portal (app-eu1 for EU
// data hosting), so record links open in the right region.
func hubspotUIDomain(ctx context.Context, m *OAuthManager, token string) string {
	var out struct {
		UIDomain string `json:"uiDomain"`
	}
	if err := m.getJSON(ctx, "https://api.hubapi.com/account-info/v3/details", token, &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out.UIDomain)
}

func identifySlack(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (string, string, []string, error) {
	var out struct {
		OK     bool   `json:"ok"`
		Team   string `json:"team"`
		TeamID string `json:"team_id"`
		URL    string `json:"url"`
		Error  string `json:"error"`
	}
	if err := m.getJSON(ctx, "https://slack.com/api/auth.test", tok.AccessToken, &out); err != nil {
		return "", "", nil, err
	}
	if !out.OK {
		return "", "", nil, fmt.Errorf("slack auth.test: %s", out.Error)
	}
	return out.TeamID, out.Team, nil, nil
}

func identifyGoogle(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (string, string, []string, error) {
	var out struct {
		Email string `json:"email"`
		ID    string `json:"id"`
	}
	if err := m.getJSON(ctx, "https://www.googleapis.com/oauth2/v2/userinfo", tok.AccessToken, &out); err != nil {
		return "", "", nil, err
	}
	return out.ID, out.Email, nil, nil
}

func identifyPipedrive(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (string, string, []string, error) {
	var out struct {
		Data struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			CompanyName string `json:"company_name"`
			Email       string `json:"email"`
		} `json:"data"`
	}
	if err := m.getJSON(ctx, "https://api.pipedrive.com/v1/users/me", tok.AccessToken, &out); err != nil {
		return "", "", nil, err
	}
	name := out.Data.CompanyName
	if name == "" {
		name = out.Data.Email
	}
	return fmt.Sprintf("%d", out.Data.ID), name, nil, nil
}

// identifySalesforce resolves the connected Salesforce org + username by GETting
// the identity URL Salesforce returns as the token's "id" extra. Best-effort:
// the connection is usable even if this lookup hiccups (the caller treats an
// error as "no profile" and still persists the token). The org's API host is
// captured separately from the token's "instance_url" extra in Exchange.
func identifySalesforce(ctx context.Context, m *OAuthManager, tok *oauth2.Token) (string, string, []string, error) {
	idURL, _ := tok.Extra("id").(string)
	idURL = strings.TrimSpace(idURL)
	if idURL == "" {
		return "", "", nil, nil
	}
	// The bearer token is sent to this URL, so it must be a Salesforce host.
	u, err := url.Parse(idURL)
	if err != nil || u.User != nil {
		return "", "", nil, errors.New("salesforce identity url is not a salesforce host")
	}
	if _, err := SalesforceInstanceURL(u.Scheme + "://" + u.Host); err != nil {
		return "", "", nil, errors.New("salesforce identity url is not a salesforce host")
	}
	var out struct {
		OrganizationID string `json:"organization_id"`
		Username       string `json:"username"`
		DisplayName    string `json:"display_name"`
	}
	if err := m.getJSON(ctx, idURL, tok.AccessToken, &out); err != nil {
		return "", "", nil, err
	}
	name := out.Username
	if name == "" {
		name = out.DisplayName
	}
	return out.OrganizationID, name, nil, nil
}

// --- helpers ----------------------------------------------------------------

// Revoke asks a provider to invalidate a token at its revocation endpoint.
func (m *OAuthManager) Revoke(ctx context.Context, endpoint, token string) error {
	form := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// salesforceIdentityIDs pulls the org and user ids out of the identity URL
// (https://login.salesforce.com/id/<org>/<user>).
func salesforceIdentityIDs(identityURL string) (orgID, userID string) {
	u, err := url.Parse(identityURL)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "id" {
		return parts[1], parts[2]
	}
	return "", ""
}

func (m *OAuthManager) getJSON(ctx context.Context, url, bearer string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return json.Unmarshal(body, dst)
}

// scopesFromToken pulls the granted scopes out of the token's "scope" extra
// field (space- or comma-delimited), falling back to the requested scopes.
func scopesFromToken(tok *oauth2.Token, requested []string) []string {
	raw, _ := tok.Extra("scope").(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return requested
	}
	sep := " "
	if strings.Contains(raw, ",") && !strings.Contains(raw, " ") {
		sep = ","
	}
	parts := strings.Split(raw, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return requested
	}
	return out
}

func randomURLToken(n int) string {
	buf := make([]byte, n)
	// crypto/rand crashes the program rather than return an error, so there is no weak fallback.
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
