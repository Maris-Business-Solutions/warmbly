package email

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"

	"github.com/warmbly/warmbly/internal/models"
)

// AvatarSaver stores a mailbox's profile photo.
type AvatarSaver interface {
	Save(ctx context.Context, orgID, id uuid.UUID, data []byte) error
}

// WireAvatars lets a connect keep the photo its provider returns.
func (s *emailService) WireAvatars(a AvatarSaver) {
	s.avatars = a
}

const graphPhotoURL = "https://graph.microsoft.com/v1.0/me/photos/240x240/$value"

// captureAvatar keeps a Microsoft mailbox's photo, read once with the connect
// handshake's token like the send identity. Google's mailbox consent carries no
// profile scope, so a per-mailbox Google sign-in has no photo to read.
func (s *emailService) captureAvatar(acc *models.Email, provider models.InboxProvider, tok *oauth2.Token) {
	if s.avatars == nil || acc == nil || acc.OrganizationID == nil || provider != models.InboxProviderOutlook || tok == nil {
		return
	}
	orgID, id, access := *acc.OrganizationID, acc.ID, tok.AccessToken
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, graphPhotoURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return
		}
		if err := s.avatars.Save(ctx, orgID, id, data); err != nil {
			log.Debug().Err(err).Str("email_account_id", id.String()).Msg("mailbox photo not stored")
		}
	}()
}
