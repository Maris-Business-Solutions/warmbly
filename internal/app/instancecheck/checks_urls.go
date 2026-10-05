package instancecheck

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/warmbly/warmbly/internal/config"
)

const (
	docsAddresses = "/development/configuration/#addresses"
	docsSSO       = "/development/accounts-and-access/#single-sign-on"
	docsRealtime  = "/development/instance-health/#realtime"
	docsDelivery  = "/guides/deliverability/"
)

func urlChecks() []check {
	return []check{
		{id: "app_url_unset", run: checkAppURLUnset},
		{id: "app_url_insecure", run: checkAppURLInsecure},
		{id: "cors_missing_origin", run: checkCORSMissingOrigin},
		{id: "api_public_url_unset_oidc", run: checkAPIPublicURLUnsetOIDC},
		{id: "oidc_discovery_failed", run: checkOIDCDiscoveryFailed},
		{id: "websocket_unreachable", run: checkWebsocketUnreachable},
		{id: "tracking_domain_unset", run: checkTrackingDomainUnset},
		{id: "tracking_domain_unreachable", run: checkTrackingDomainUnreachable},
		{id: "tracking_domain_shares_brand", run: checkTrackingDomainSharesBrand},
		{id: "app_origin_wildcard", run: checkAppOriginWildcard},
	}
}

func checkAppURLUnset(ctx context.Context, d Deps, in Input) *Finding {
	if appURLConfigured() {
		return nil
	}
	guess := config.AppBaseURL()
	if guess == "" {
		return result(CategoryURLs, SeverityError, "APP_URL is not set",
			"APP_URL is not set and nothing else names this instance's dashboard, so password reset, invitation and setup links "+
				"are being mailed as paths with no host in front of them and nobody can open one. Set APP_URL to your dashboard origin.",
			docsAddresses)
	}
	return result(CategoryURLs, SeverityError, "APP_URL is not set",
		fmt.Sprintf("APP_URL is not set, so password reset, invitation and setup links are being built against %s, guessed from "+
			"CORS_ALLOW_ORIGINS or PUBLIC_HOST. If that is not where your dashboard is served, every one of those links is dead. "+
			"Set APP_URL to your dashboard origin.", guess),
		docsAddresses)
}

// Checks the URL links are actually built from, not only the one that was
// configured. An install that set no APP_URL still mails reset and invitation
// tokens, against a base inferred from CORS_ALLOW_ORIGINS or PUBLIC_HOST, and
// that base is as capable of being plain http as a configured one.
func checkAppURLInsecure(ctx context.Context, d Deps, in Input) *Finding {
	raw := appURL()
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || isLoopbackHost(u.Hostname()) {
		return nil
	}
	source := "APP_URL is"
	if !appURLConfigured() {
		source = "APP_URL is not set, so emailed links are being built against"
	}
	return result(CategoryURLs, SeverityWarning, "The dashboard is not behind HTTPS",
		fmt.Sprintf("%s %s. Browsers refuse WebAuthn outside a secure context, so passkeys are disabled, and session "+
			"cookies and the reset and invitation tokens in emailed links all travel in the clear. Put the dashboard "+
			"behind HTTPS.", source, raw),
		docsAddresses)
}

func checkCORSMissingOrigin(ctx context.Context, d Deps, in Input) *Finding {
	allowed := runtimeOf(d).CORSOrigins
	if len(allowed) == 0 {
		allowed = splitList(env("CORS_ALLOW_ORIGINS"))
	}
	// An empty list means the backend derived one at boot and did not hand it
	// over; there is nothing to compare against, so this check cannot run.
	if len(allowed) == 0 {
		return nil
	}

	wanted := []string{}
	if appURLConfigured() {
		wanted = append(wanted, strings.TrimRight(appURL(), "/"))
	}
	if in.Origin != "" {
		wanted = append(wanted, strings.TrimRight(in.Origin, "/"))
	}

	for _, origin := range wanted {
		if origin == "" || containsOrigin(allowed, origin) {
			continue
		}
		return result(CategoryURLs, SeverityWarning, "An origin is missing from the CORS allowlist",
			fmt.Sprintf("%s is not in the allowed CORS origins, so the browser will block its API calls. "+
				"Add it to CORS_ALLOW_ORIGINS.", origin),
			docsAddresses)
	}
	return nil
}

func checkAPIPublicURLUnsetOIDC(ctx context.Context, d Deps, in Input) *Finding {
	if env("OIDC_ISSUER_URL") == "" {
		return nil
	}
	if env("API_PUBLIC_URL") != "" || env("OIDC_REDIRECT_URL") != "" {
		return nil
	}
	return result(CategoryURLs, SeverityError, "OIDC has no redirect URL",
		"OIDC is configured but there is no redirect URL: API_PUBLIC_URL is empty and OIDC_REDIRECT_URL is not set, "+
			"so the OIDC login path is disabled. Set API_PUBLIC_URL to this backend's public base.",
		docsSSO)
}

func checkOIDCDiscoveryFailed(ctx context.Context, d Deps, in Input) *Finding {
	rt := runtimeOf(d)
	if rt.OIDCDiscoveryErr == "" {
		return nil
	}
	return result(CategoryURLs, SeverityError, "OIDC discovery failed",
		fmt.Sprintf("Discovery against %s failed at boot, so the single sign-on button is not shown: %s.",
			env("OIDC_ISSUER_URL"), rt.OIDCDiscoveryErr),
		docsSSO)
}

func checkWebsocketUnreachable(ctx context.Context, d Deps, in Input) *Finding {
	raw := runtimeOf(d).WebsocketURL
	if raw == "" {
		raw = env("WEBSOCKET_URL")
	}
	health := realtimeHealthURL(raw)
	if health == "" {
		return nil
	}
	if probeHTTP(ctx, health) {
		return nil
	}
	// Running out of budget is not evidence of a down service.
	if ctx.Err() != nil {
		return nil
	}
	return result(CategoryURLs, SeverityWarning, "The realtime service is not reachable",
		fmt.Sprintf("The realtime service is not reachable at %s, so the dashboard will not update live "+
			"and presence will be empty.", raw),
		docsRealtime)
}

// An install that never configured a tracking host is a working install, and
// this is the only place it is ever said out loud: nothing else fails, mail
// still sends, and the operator finds out months later that no campaign ever
// recorded an open. An installer run that skipped the wizard lands here.
func checkTrackingDomainUnset(ctx context.Context, d Deps, in Input) *Finding {
	if env("TRACKING_DOMAIN") != "" {
		return nil
	}
	return result(CategoryURLs, SeverityWarning, "No tracking domain is set",
		"TRACKING_DOMAIN is not set, so campaign mail goes out with no open pixel and unwrapped links, no workspace "+
			"can verify a tracking domain of its own, and every recipient's unsubscribe link is served from your API "+
			"address instead of the sender's domain. Sending itself is unaffected. Set TRACKING_DOMAIN to a host "+
			"routed to the tracking service, or leave it unset deliberately if you run no tracking service.",
		docsDelivery)
}

// checkTrackingDomainSharesBrand catches a tracking host on the same
// registered domain as the product itself. Every campaign's links and opt-out
// pages carry that host, so one customer's complaints can get the domain listed
// on a URL blocklist, and the listing lands on the domain that also serves the
// marketing site and the platform's own mail.
func checkTrackingDomainSharesBrand(_ context.Context, _ Deps, _ Input) *Finding {
	tracking := hostOnly(env("TRACKING_DOMAIN"))
	if tracking == "" || isLoopbackHost(tracking) {
		return nil
	}
	for _, key := range []string{"APP_URL", "API_PUBLIC_URL"} {
		other := hostOf(env(key))
		if other == "" {
			continue
		}
		if registrableDomain(tracking) != registrableDomain(other) {
			continue
		}
		return result(CategoryURLs, SeverityWarning, "The tracking domain is on your own brand's domain",
			fmt.Sprintf("TRACKING_DOMAIN is %s, which shares the registered domain %s with %s. Every campaign's "+
				"links and unsubscribe pages carry that host, so complaints against any one workspace can get it "+
				"listed on a URL blocklist, and the listing reaches the domain serving your site and your platform "+
				"mail as well. Use a separate registered domain for tracking, and move workspaces onto their own "+
				"verified tracking domains.", tracking, registrableDomain(tracking), key),
			docsDelivery)
	}
	return nil
}

func checkTrackingDomainUnreachable(ctx context.Context, d Deps, in Input) *Finding {
	domain := env("TRACKING_DOMAIN")
	if domain == "" {
		return nil
	}
	// Outbound mail always builds https:// links, so that is the probe that
	// matters; http is tried second so a local sink is not reported as down.
	for _, candidate := range trackingProbeURLs(domain) {
		if probeHTTP(ctx, candidate) {
			return nil
		}
	}
	// Running out of budget is not evidence of a down service.
	if ctx.Err() != nil {
		return nil
	}
	return result(CategoryURLs, SeverityWarning, "The tracking domain did not answer",
		fmt.Sprintf("%s did not answer, so open pixels and click links in campaign mail will not record. "+
			"Recipients still receive the mail.", domain),
		docsDelivery)
}

func checkAppOriginWildcard(ctx context.Context, d Deps, in Input) *Finding {
	if env("APP_ORIGIN") != "" {
		return nil
	}
	// APP_ORIGIN is only needed when APP_URL cannot supply the origin, since
	// the callback page derives its postMessage target from APP_URL otherwise.
	if u, err := url.Parse(config.AppBaseURL()); err == nil && u.Scheme != "" && u.Host != "" {
		return nil
	}
	return result(CategoryURLs, SeverityInfo, "No OAuth callback target origin",
		"Neither APP_ORIGIN nor a usable APP_URL is set, so the mailbox OAuth callback page posts the authorization "+
			"code back to the dashboard with a wildcard target origin. Set APP_URL to your dashboard origin, "+
			"or APP_ORIGIN if the dashboard is served somewhere else.",
		docsAddresses)
}

// realtimeHealthURL mirrors wsHealthURL in cmd/backend/main.go.
func realtimeHealthURL(wsURI string) string {
	if wsURI == "" {
		return ""
	}
	u, err := url.Parse(wsURI)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme == "wss" {
		u.Scheme = "https"
	} else {
		u.Scheme = "http"
	}
	u.Path = "/health"
	u.RawQuery = ""
	return u.String()
}

func trackingProbeURLs(domain string) []string {
	if strings.Contains(domain, "://") {
		return []string{strings.TrimRight(domain, "/") + "/health"}
	}
	domain = strings.TrimRight(domain, "/")
	return []string{"https://" + domain + "/health", "http://" + domain + "/health"}
}

func splitList(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func containsOrigin(list []string, origin string) bool {
	for _, item := range list {
		if item == "*" || strings.EqualFold(strings.TrimRight(item, "/"), origin) {
			return true
		}
	}
	return false
}
