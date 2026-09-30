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

// rememberOffer keeps Cloud's answer; a Cloud that sends none does not serve redirects.
func (s *service) rememberOffer(o *models.PoolLinkRedirectOffer) {
	if o == nil {
		o = &models.PoolLinkRedirectOffer{}
	}
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

// RedirectOffer is Cloud's offer and whether the instance is linked; an unreachable Cloud keeps the last answer, nil before any.
func (s *service) RedirectOffer(ctx context.Context) (*models.PoolLinkRedirectOffer, bool) {
	l, err := s.repo.Get(ctx)
	if err != nil {
		return nil, true // unknown reads as unreachable, never as "not linked"
	}
	if l == nil {
		return nil, false
	}
	fresh := func() (*models.PoolLinkRedirectOffer, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.offer.offer, !s.offer.at.IsZero() && time.Since(s.offer.at) < offerTTL
	}
	if o, ok := fresh(); ok {
		return o, true
	}
	s.offerFetch.Lock()
	defer s.offerFetch.Unlock()
	if o, ok := fresh(); ok {
		return o, true
	}
	cached, _ := fresh()
	var info models.PoolLinkInstanceInfo
	if xerr := s.clientFor(l).do(ctx, http.MethodGet, "/instance", nil, &info); xerr != nil {
		return cached, true
	}
	s.rememberOffer(info.Redirects)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offer.offer, true
}

func (s *service) ListRedirects(ctx context.Context) ([]models.DomainRedirect, *errx.Error) {
	l, xerr := s.link(ctx)
	if xerr != nil {
		return nil, xerr
	}
	var out struct {
		Data []models.DomainRedirect `json:"data"`
	}
	if xerr := s.clientFor(l).do(ctx, http.MethodGet, "/instance/redirects", nil, &out); xerr != nil {
		return nil, xerr
	}
	return out.Data, nil
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
