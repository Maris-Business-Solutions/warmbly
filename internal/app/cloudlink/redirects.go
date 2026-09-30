package cloudlink

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// offerTTL keeps the dashboard's redirect choice from calling Cloud on every render.
const offerTTL = time.Minute

type cachedOffer struct {
	offer *models.PoolLinkRedirectOffer
	at    time.Time
}

func (s *service) rememberOffer(o *models.PoolLinkRedirectOffer) {
	s.mu.Lock()
	s.offer = cachedOffer{offer: o, at: time.Now()}
	s.mu.Unlock()
}

func (s *service) forgetOffer() {
	s.mu.Lock()
	s.offer = cachedOffer{}
	s.mu.Unlock()
}

func (s *service) OnDisconnect(fn func(context.Context)) {
	s.disconnected = append(s.disconnected, fn)
}

// RedirectOffer is what Cloud serves for this instance, and whether the
// instance is linked at all. A Cloud that cannot be reached keeps the last
// answer; with none yet, the offer is nil while linked stays true.
func (s *service) RedirectOffer(ctx context.Context) (*models.PoolLinkRedirectOffer, bool) {
	l, err := s.repo.Get(ctx)
	if err != nil || l == nil {
		return nil, false
	}
	s.mu.Lock()
	cached := s.offer
	s.mu.Unlock()
	if !cached.at.IsZero() && time.Since(cached.at) < offerTTL {
		return cached.offer, true
	}
	var info models.PoolLinkInstanceInfo
	if xerr := s.clientFor(l).do(ctx, http.MethodGet, "/instance", nil, &info); xerr != nil {
		return cached.offer, true
	}
	s.rememberOffer(info.Redirects)
	return info.Redirects, true
}

func redirectPath(domain string) string { return "/instance/redirects/" + url.PathEscape(domain) }

func (s *service) redirectCall(ctx context.Context, method, path string, body any) (*models.DomainRedirect, *errx.Error) {
	l, xerr := s.link(ctx)
	if xerr != nil {
		return nil, xerr
	}
	var out models.DomainRedirect
	if xerr := s.clientFor(l).do(ctx, method, path, body, &out); xerr != nil {
		return nil, xerr
	}
	return &out, nil
}

// PutRedirect leaves the offer cached: Cloud enforces its own limit, and a bulk move must not re-read the offer per domain.
func (s *service) PutRedirect(ctx context.Context, domain string, in models.DomainRedirectRequest) (*models.DomainRedirect, *errx.Error) {
	// Cloud serves it itself; the instance's own choice of server means nothing there.
	in.ServedBy = ""
	return s.redirectCall(ctx, http.MethodPut, redirectPath(domain), in)
}

func (s *service) GetRedirect(ctx context.Context, domain string) (*models.DomainRedirect, *errx.Error) {
	return s.redirectCall(ctx, http.MethodGet, redirectPath(domain), nil)
}

func (s *service) VerifyRedirect(ctx context.Context, domain string) (*models.DomainRedirect, *errx.Error) {
	return s.redirectCall(ctx, http.MethodPost, redirectPath(domain)+"/verify", nil)
}

func (s *service) DeleteRedirect(ctx context.Context, domain string) *errx.Error {
	l, xerr := s.link(ctx)
	if xerr != nil {
		return xerr
	}
	return s.clientFor(l).do(ctx, http.MethodDelete, redirectPath(domain), nil, nil)
}
