package sendingdomain

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/mailhost"
	"github.com/warmbly/warmbly/internal/repository"
)

// CloudRedirects is Warmbly Cloud serving root redirects for this linked instance.
type CloudRedirects interface {
	// RedirectOffer is Cloud's offer and whether the instance is linked; a linked instance gets nil while Cloud is unreachable.
	RedirectOffer(ctx context.Context) (*models.PoolLinkRedirectOffer, bool)
	PutRedirect(ctx context.Context, domain string, in models.DomainRedirectRequest) (*models.DomainRedirect, *errx.Error)
	GetRedirect(ctx context.Context, domain string) (*models.DomainRedirect, *errx.Error)
	VerifyRedirect(ctx context.Context, domain string) (*models.DomainRedirect, *errx.Error)
	DeleteRedirect(ctx context.Context, domain string) *errx.Error
}

// WireCloud attaches the instance's link to Warmbly Cloud; optional.
func (s *Service) WireCloud(c CloudRedirects) { s.cloud = c }

const unlinkedMessage = "This instance is no longer connected to Warmbly Cloud. Reconnect it in Settings, or serve the redirect from this server."

// cloudServable says whether Cloud may take the domain. One Cloud already serves is only edited, so Cloud's own answer decides.
func (s *Service) cloudServable(ctx context.Context, orgID uuid.UUID, domain string, alreadyCloud bool) *errx.Error {
	if s.cloud == nil {
		return errx.NewWithIdentifier(errx.Conflict, ErrIDCloudUnavailable, "Connect this instance to Warmbly Cloud to serve redirects from there.")
	}
	if !alreadyCloud {
		offer, linked := s.cloud.RedirectOffer(ctx)
		switch {
		case !linked:
			return errx.NewWithIdentifier(errx.Conflict, ErrIDCloudUnavailable, "Connect this instance to Warmbly Cloud to serve redirects from there.")
		case offer == nil:
			// Linked, but Cloud has not answered since this process started.
			return errx.NewWithIdentifier(errx.ServiceUnavailable, ErrIDCloudUnreachable, "Warmbly Cloud could not be reached. Try again in a moment.")
		case !offer.Available:
			return errx.NewWithIdentifier(errx.Conflict, ErrIDCloudUnavailable, "Warmbly Cloud does not serve redirects for this instance right now.")
		}
	}
	taken, err := s.redirects.CloudServedElsewhere(ctx, orgID, domain)
	if err != nil {
		return errx.InternalError()
	}
	if taken {
		return errx.NewWithIdentifier(errx.Conflict, ErrIDTaken, "Another workspace on this instance already redirects this domain.")
	}
	return nil
}

// cloudRefusal keeps Cloud's own refusal (its code and sentence), and names Cloud when it could not be reached.
// Cloud's 401 is about the link token, never the caller's session, so it must not reach the dashboard as one.
func cloudRefusal(xerr *errx.Error) *errx.Error {
	switch {
	case cloudGone(xerr):
		return errx.NewWithIdentifier(errx.Conflict, ErrIDCloudUnavailable, "This instance is no longer linked to Warmbly Cloud. Reconnect it in Settings, or serve the redirect from this server.")
	case xerr.Code >= 500 || xerr.Code == errx.TooManyRequests:
		return errx.NewWithIdentifier(errx.ServiceUnavailable, ErrIDCloudUnreachable, "Warmbly Cloud could not be reached. Try again in a moment.")
	}
	return xerr
}

// cloudGone is a Cloud answer meaning the redirect is no longer served there: none there, or no link at all.
func cloudGone(xerr *errx.Error) bool {
	if xerr.Code == errx.Unauthorized || xerr.Code == errx.Forbidden {
		return true // a revoked link: Cloud dropped its redirects with it
	}
	switch xerr.Identifier {
	case ErrIDRemoteNotFound, "cloud_link_not_connected", "pool_link_revoked", "pool_link_instance_not_found", "unauthorized":
		return true
	}
	return false
}

// releaseCloud stops Cloud serving a domain before this instance forgets it did.
func (s *Service) releaseCloud(ctx context.Context, domain string) *errx.Error {
	if s.cloud == nil {
		return nil
	}
	if xerr := s.cloud.DeleteRedirect(ctx, domain); xerr != nil && !cloudGone(xerr) {
		return errx.NewWithIdentifier(errx.ServiceUnavailable, ErrIDCloudUnreachable,
			"Warmbly Cloud could not be reached, so the redirect is still served there. Try again in a moment.")
	}
	return nil
}

// mirror records Cloud's verdict on a cloud-served row as this row's own.
func (s *Service) mirror(ctx context.Context, r *models.DomainRedirect, remote *models.DomainRedirect) (*models.DomainRedirect, *errx.Error) {
	if err := s.redirects.SetCheck(ctx, r.ID, remote.Verified, remote.LastError); err != nil {
		if errors.Is(err, repository.ErrRedirectTaken) {
			return nil, errx.NewWithIdentifier(errx.Conflict, ErrIDTaken, "Another workspace on this instance already redirects this domain.")
		}
		return nil, errx.InternalError()
	}
	if err := s.redirects.SetRemote(ctx, r.ID, remote.ServeHost, remote.Records); err != nil {
		return nil, errx.InternalError()
	}
	if err := s.redirects.SetReach(ctx, r.ID, remote.Reach); err != nil {
		return nil, errx.InternalError()
	}
	return s.finish(ctx, r, nil)
}

// checkCloud asks Cloud for its verdict. A row Cloud lost is put back, an
// ended link stops the row, and Cloud being unreachable changes nothing.
func (s *Service) checkCloud(ctx context.Context, r *models.DomainRedirect, force bool) (*models.DomainRedirect, *errx.Error) {
	if s.cloud == nil {
		return s.unlink(ctx, r)
	}
	var remote *models.DomainRedirect
	var xerr *errx.Error
	if force {
		remote, xerr = s.cloud.VerifyRedirect(ctx, r.Domain)
	} else {
		remote, xerr = s.cloud.GetRedirect(ctx, r.Domain)
	}
	if xerr != nil && xerr.Identifier == ErrIDRemoteNotFound {
		www := r.IncludeWWW
		remote, xerr = s.cloud.PutRedirect(ctx, r.Domain, models.DomainRedirectRequest{TargetURL: r.TargetURL, IncludeWWW: &www})
	}
	if xerr != nil {
		if cloudGone(xerr) {
			return s.unlink(ctx, r)
		}
		// Cloud refusing the row (its limit, another workspace's domain) is an answer, not an outage.
		if xerr.Code < 500 && xerr.Code != errx.TooManyRequests {
			return s.stop(ctx, r, xerr.Message)
		}
		if err := s.redirects.SetCheck(ctx, r.ID, r.Verified, r.LastError); err != nil && !errors.Is(err, repository.ErrRedirectTaken) {
			return nil, errx.InternalError()
		}
		if force {
			return nil, cloudRefusal(xerr)
		}
		return s.finish(ctx, r, nil)
	}
	return s.mirror(ctx, r, remote)
}

func (s *Service) unlink(ctx context.Context, r *models.DomainRedirect) (*models.DomainRedirect, *errx.Error) {
	return s.stop(ctx, r, unlinkedMessage)
}

func (s *Service) stop(ctx context.Context, r *models.DomainRedirect, why string) (*models.DomainRedirect, *errx.Error) {
	if err := s.redirects.SetCheck(ctx, r.ID, false, why); err != nil {
		return nil, errx.InternalError()
	}
	if err := s.redirects.SetReach(ctx, r.ID, nil); err != nil {
		return nil, errx.InternalError()
	}
	return s.finish(ctx, r, nil)
}

// MarkCloudUnlinked stops every cloud-served redirect claiming to be live, once the link to Cloud ends.
func (s *Service) MarkCloudUnlinked(ctx context.Context) {
	_ = s.redirects.UnverifyCloudServed(ctx, unlinkedMessage)
}

// The Cloud side: rows served for a linked instance, proven against this deployment's own TXT value and tracking host.

// LinkedOffer is what this deployment offers a linked instance; unavailable without a tracking host.
func (s *Service) LinkedOffer(ctx context.Context, instanceID uuid.UUID) *models.PoolLinkRedirectOffer {
	offer := &models.PoolLinkRedirectOffer{Limit: config.PoolLinkRedirectLimit}
	if s.target() == "" {
		return offer
	}
	offer.Available, offer.Host = true, s.target()
	if n, err := s.redirects.CountLinked(ctx, instanceID); err == nil {
		offer.Used = n
	}
	return offer
}

func linkedNotFound() *errx.Error {
	return errx.NewWithIdentifier(errx.NotFound, ErrIDRemoteNotFound, "Warmbly Cloud does not serve a redirect for this domain.")
}

func (s *Service) LinkedList(ctx context.Context, inst *models.PoolLinkInstance) ([]models.DomainRedirect, *errx.Error) {
	list, err := s.redirects.ListLinked(ctx, inst.ID)
	if err != nil {
		return nil, errx.InternalError()
	}
	for i := range list {
		s.decorate(&list[i], nil)
	}
	return list, nil
}

func (s *Service) LinkedGet(ctx context.Context, inst *models.PoolLinkInstance, domain string) (*models.DomainRedirect, *errx.Error) {
	r, err := s.redirects.GetLinked(ctx, inst.ID, normalizeDomain(domain))
	if err != nil {
		return nil, errx.InternalError()
	}
	if r == nil {
		return nil, linkedNotFound()
	}
	s.decorate(r, nil)
	return r, nil
}

// LinkedSet serves (or updates) a redirect for a linked instance. Ownership is
// proven here, by this deployment's own TXT value and its own tracking host,
// never taken on the instance's word.
func (s *Service) LinkedSet(ctx context.Context, inst *models.PoolLinkInstance, domain string, in models.DomainRedirectRequest) (*models.DomainRedirect, *errx.Error) {
	domain = normalizeDomain(domain)
	if domain == "" || !plausibleDomain(domain) {
		return nil, errx.NewWithIdentifier(errx.BadRequest, ErrIDNotYours, "Enter a domain.")
	}
	if mailhost.SharedProvider(domain) {
		return nil, errx.NewWithIdentifier(errx.BadRequest, ErrIDConsumerDomain, "A shared email provider's domain cannot be redirected.")
	}
	target, xerr := cleanTarget(in.TargetURL, domain)
	if xerr != nil {
		return nil, xerr
	}
	if s.target() == "" {
		return nil, errx.NewWithIdentifier(errx.Conflict, ErrIDCloudUnavailable, "Warmbly Cloud does not serve redirects right now.")
	}
	www := true
	if in.IncludeWWW != nil {
		www = *in.IncludeWWW
	}
	existing, err := s.redirects.GetLinked(ctx, inst.ID, domain)
	if err != nil {
		return nil, errx.InternalError()
	}
	if existing == nil {
		other, err := s.redirects.Get(ctx, inst.OrganizationID, domain)
		if err != nil {
			return nil, errx.InternalError()
		}
		if other != nil {
			return nil, errx.NewWithIdentifier(errx.Conflict, ErrIDTaken, "Your Warmbly Cloud workspace already has a redirect for this domain. Remove it there first.")
		}
	}
	instanceID := inst.ID
	r := &models.DomainRedirect{ID: uuid.New(), OrganizationID: inst.OrganizationID, Domain: domain, TargetURL: target, IncludeWWW: www,
		VerifyToken: s.proof.Value(inst.OrganizationID, domain), ServedBy: models.RedirectServedByInstance, LinkedInstanceID: &instanceID}
	ok, err := s.redirects.UpsertLinked(ctx, r, inst.CreatedBy, config.PoolLinkRedirectLimit)
	if err != nil {
		return nil, errx.InternalError()
	}
	if !ok {
		return nil, errx.NewWithIdentifier(errx.Conflict, ErrIDLimit, "This instance has reached the number of redirects Warmbly Cloud serves for it.")
	}
	out, xerr := s.check(ctx, r, true)
	// A new row the instance is told was refused must not stay behind, counted and unlisted.
	if xerr != nil && existing == nil {
		_, _ = s.redirects.DeleteLinked(ctx, inst.ID, domain)
	}
	return out, xerr
}

func (s *Service) LinkedVerify(ctx context.Context, inst *models.PoolLinkInstance, domain string) (*models.DomainRedirect, *errx.Error) {
	r, err := s.redirects.GetLinked(ctx, inst.ID, normalizeDomain(domain))
	if err != nil {
		return nil, errx.InternalError()
	}
	if r == nil {
		return nil, linkedNotFound()
	}
	return s.check(ctx, r, true)
}

func (s *Service) LinkedDelete(ctx context.Context, inst *models.PoolLinkInstance, domain string) *errx.Error {
	ok, err := s.redirects.DeleteLinked(ctx, inst.ID, normalizeDomain(domain))
	if err != nil {
		return errx.InternalError()
	}
	if !ok {
		return linkedNotFound()
	}
	return nil
}

// plausibleDomain refuses what cannot be a registrable name before any DNS work.
func plausibleDomain(d string) bool {
	if len(d) > 253 || !strings.Contains(d, ".") {
		return false
	}
	for _, c := range d {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
