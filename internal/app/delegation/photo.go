package delegation

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
)

const photoMaxBytes = 4 << 20

// ErrPhotoUnavailable is a grant that cannot be asked right now, which says nothing about the photo.
var ErrPhotoUnavailable = errors.New("delegation: mailbox photo cannot be read through this grant")

// MailboxPhoto reads the profile photo a delegated mailbox's directory holds,
// with the scopes the grant already has; nil, nil when none is set.
func (s *Service) MailboxPhoto(ctx context.Context, accountID uuid.UUID) ([]byte, error) {
	d, xerr := s.store.GetDelegation(ctx, accountID)
	if xerr != nil {
		return nil, xerr
	}
	if d == nil {
		return nil, ErrPhotoUnavailable
	}
	g, err := s.repo.GetByID(ctx, d.GrantID)
	if err != nil {
		return nil, err
	}
	if g == nil || g.OrganizationID != d.OrganizationID || !covers(g, d.Email) || g.Status != "active" {
		return nil, ErrPhotoUnavailable
	}
	switch d.Provider {
	case models.InboxProviderGoogle:
		// The directory is read as the administrator the grant names, like the domain listing.
		if !s.googleEnabled() || g.AdminEmail == "" {
			return nil, ErrPhotoUnavailable
		}
		ts, err := s.googleSource(g.AdminEmail, config.GoogleDirectoryScope)
		if err != nil {
			return nil, err
		}
		var p struct {
			PhotoData string `json:"photoData"`
		}
		if err := s.getJSON(ctx, ts, s.adminBase+"/users/"+url.PathEscape(d.Email)+"/photos/thumbnail", &p); err != nil {
			return nil, notFoundIsNone(err)
		}
		return decodeWebSafe(p.PhotoData)
	case models.InboxProviderOutlook:
		if !s.microsoftEnabled() {
			return nil, ErrPhotoUnavailable
		}
		return s.getBytes(ctx, s.microsoftSource(g.Tenant), s.graphBase+"/users/"+url.PathEscape(d.Email)+"/photos/240x240/$value")
	}
	return nil, ErrPhotoUnavailable
}

// getBytes reads a binary Graph resource; nil, nil on 404, which is how Graph says there is no photo.
func (s *Service) getBytes(ctx context.Context, ts oauth2.TokenSource, u string) ([]byte, error) {
	tok, err := ts.Token()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, &apiError{status: resp.StatusCode}
	}
	return io.ReadAll(io.LimitReader(resp.Body, photoMaxBytes))
}

func notFoundIsNone(err error) error {
	var ae *apiError
	if errors.As(err, &ae) && ae.status == http.StatusNotFound {
		return nil
	}
	return err
}

// decodeWebSafe reads the Directory API's photoData, web-safe base64 with or without padding.
func decodeWebSafe(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	v = strings.TrimRight(v, "=")
	return base64.RawURLEncoding.DecodeString(v)
}
