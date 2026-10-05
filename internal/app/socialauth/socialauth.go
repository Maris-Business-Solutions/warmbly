// Package socialauth is the browser authorization-code flow for Sign in with
// Google and Sign in with Apple. The native apps hand the backend a
// provider-signed ID token directly (internal/pkg/idtoken); a browser has to be
// sent to the provider and come back with a code, which is what this owns.
//
// Both providers end where generic OIDC ends, at a verified idtoken.Claims, so
// the auth service runs one federated login for all three.
package socialauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/warmbly/warmbly/internal/pkg/appleauth"
	"github.com/warmbly/warmbly/internal/pkg/idtoken"
	"golang.org/x/oauth2"
	googleendpoint "golang.org/x/oauth2/google"
)

// Google is the browser authorization-code flow for Sign in with Google.
type Google struct {
	oauth    *oauth2.Config
	verifier *idtoken.Verifier
}

// NewGoogle builds the flow. Both halves of the client credential and an
// absolute redirect URL are required, because either one missing surfaces only
// as a failed sign-in at the provider.
func NewGoogle(clientID, clientSecret, redirectURL string) (*Google, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("socialauth: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are both required")
	}
	if redirectURL == "" {
		return nil, errors.New("socialauth: GOOGLE_REDIRECT_URI is required (or set API_PUBLIC_URL and let it derive)")
	}
	if !strings.HasPrefix(redirectURL, "http://") && !strings.HasPrefix(redirectURL, "https://") {
		return nil, fmt.Errorf("socialauth: GOOGLE_REDIRECT_URI %q is not an absolute URL", redirectURL)
	}

	return &Google{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			// The ID token carries everything a login needs, so no userinfo
			// call and no stored token.
			Scopes:   []string{"openid", "email", "profile"},
			Endpoint: googleendpoint.Endpoint,
		},
		verifier: idtoken.GoogleVerifier(clientID),
	}, nil
}

func (g *Google) ProviderName() string { return "Google" }

// RedirectURL is the URI to register at the provider, surfaced so boot can log
// it for an operator whose sign-in is failing.
func (g *Google) RedirectURL() string { return g.oauth.RedirectURL }

// AuthCodeURL builds the authorization request. RFC 9700 requires PKCE for
// confidential clients too, and the nonce binds the ID token to this attempt.
// prompt=select_account stops Google silently reusing whichever account the
// browser is already signed into.
func (g *Google) AuthCodeURL(state, nonce, verifier string) string {
	return g.oauth.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// Exchange trades the authorization code for a verified identity.
func (g *Google) Exchange(ctx context.Context, code, verifier, expectedNonce string) (*idtoken.Claims, error) {
	tok, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("socialauth: google code exchange: %w", err)
	}

	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("socialauth: google token response carried no id_token")
	}

	claims, err := g.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("socialauth: verifying google id_token: %w", err)
	}
	if err := checkIdentity(claims, expectedNonce); err != nil {
		return nil, err
	}
	// The verifier already collapses Google's two issuer spellings onto the
	// https form, which is what the identity is keyed on.
	return claims, nil
}

// Apple is the browser authorization-code flow for Sign in with Apple. Apple
// only sends the email claim when the email scope is requested, and any scope
// forces response_mode=form_post, so its callback arrives as a cross-site POST.
type Apple struct {
	client      AppleCodeExchanger
	servicesID  string
	redirectURL string
	verifier    *idtoken.Verifier
}

// AppleCodeExchanger trades an authorization code at Apple's token endpoint;
// *appleauth.Client is the production implementation.
type AppleCodeExchanger interface {
	ExchangeCode(ctx context.Context, code, redirectURI string) (*appleauth.TokenResponse, error)
}

// NewApple builds the flow from the same credentials the native path uses. The
// client id is the Services ID (the web identifier), not the app's bundle ID.
func NewApple(client AppleCodeExchanger, servicesID, redirectURL string) (*Apple, error) {
	if client == nil {
		return nil, errors.New("socialauth: apple client is not configured")
	}
	if servicesID == "" {
		return nil, errors.New("socialauth: APPLE_APP_ID is required")
	}
	if redirectURL == "" {
		return nil, errors.New("socialauth: APPLE_REDIRECT_URI is required (or set API_PUBLIC_URL and let it derive)")
	}
	// Apple rejects a plain-http return URL, so refuse at boot rather than at
	// the first sign-in.
	if !strings.HasPrefix(redirectURL, "https://") {
		return nil, fmt.Errorf("socialauth: Apple requires an https redirect URI, got %q", redirectURL)
	}

	return &Apple{
		client:      client,
		servicesID:  servicesID,
		redirectURL: redirectURL,
		verifier:    idtoken.AppleVerifier(servicesID),
	}, nil
}

func (a *Apple) ProviderName() string { return "Apple" }
func (a *Apple) RedirectURL() string  { return a.redirectURL }

// AuthCodeURL builds the authorization request. Apple does not support PKCE on
// the web flow, so the verifier is ignored; one-time state and the nonce inside
// the ID token are what bind the response to this attempt.
func (a *Apple) AuthCodeURL(state, nonce, _ string) string {
	return appleauth.AuthorizeURL(appleauth.AuthorizeURLConfig{
		ClientID:     a.servicesID,
		RedirectURI:  a.redirectURL,
		State:        state,
		Nonce:        nonce,
		Scope:        []string{"name", "email"},
		ResponseType: "code",
		ResponseMode: "form_post",
	})
}

// Exchange trades the authorization code for a verified identity. The ID token
// comes straight off Apple's token endpoint and is still verified against
// Apple's keys, because the audience check is what rejects one issued for a
// different client.
func (a *Apple) Exchange(ctx context.Context, code, _, expectedNonce string) (*idtoken.Claims, error) {
	resp, err := a.client.ExchangeCode(ctx, code, a.redirectURL)
	if err != nil {
		if errors.Is(err, appleauth.ErrInvalidGrant) {
			return nil, fmt.Errorf("socialauth: apple rejected the authorization code: %w", err)
		}
		return nil, fmt.Errorf("socialauth: apple code exchange: %w", err)
	}
	if resp == nil || resp.IDToken == "" {
		return nil, errors.New("socialauth: apple token response carried no id_token")
	}

	claims, err := a.verifier.Verify(ctx, resp.IDToken)
	if err != nil {
		return nil, fmt.Errorf("socialauth: verifying apple id_token: %w", err)
	}
	if err := checkIdentity(claims, expectedNonce); err != nil {
		return nil, err
	}
	return claims, nil
}

// checkIdentity is the part neither provider does for us: the nonce proves the
// token was minted for this request rather than replayed, and an unverified
// address is how federated login turns into account takeover.
func checkIdentity(claims *idtoken.Claims, expectedNonce string) error {
	if expectedNonce == "" || claims.Nonce != expectedNonce {
		return errors.New("socialauth: id_token nonce does not match this authorization request")
	}
	if claims.Subject == "" {
		return errors.New("socialauth: id_token carried no subject")
	}
	if claims.Email == "" {
		return errors.New("socialauth: id_token carried no email claim")
	}
	if !claims.EmailVerified {
		return errors.New("socialauth: the provider did not report this email address as verified")
	}
	return nil
}
