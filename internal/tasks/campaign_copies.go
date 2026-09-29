package tasks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/mailhdr"
)

// campaignCopies resolves who is copied on one send: the campaign's own CC and
// BCC, then the lead's copied contacts. An address that is suppressed, is the
// lead's own, or already appears earlier is left off, so a copy can never
// reach someone the lead's email would not have been allowed to.
func (s *tasksService) campaignCopies(ctx context.Context, orgID uuid.UUID, campaign *models.Campaign, contact *models.Contact) (cc, bcc []string, err error) {
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(contact.Email)): true}
	keep := func(addr string) (bool, error) {
		bare := strings.ToLower(mailhdr.Bare(addr))
		if bare == "" || seen[bare] {
			return false, nil
		}
		if s.advanced != nil {
			suppressed, _, xerr := s.advanced.ShouldSuppressRecipient(ctx, orgID, bare)
			if xerr != nil {
				return false, xerr
			}
			if suppressed {
				return false, nil
			}
		}
		seen[bare] = true
		return true, nil
	}

	for _, a := range campaign.CC {
		ok, kerr := keep(a)
		if kerr != nil {
			return nil, nil, kerr
		}
		if ok {
			cc = append(cc, a)
		}
	}
	for _, a := range campaign.BCC {
		ok, kerr := keep(a)
		if kerr != nil {
			return nil, nil, kerr
		}
		if ok {
			bcc = append(bcc, a)
		}
	}

	copies, lerr := s.campaignProgressRepo.ListLeadCC(ctx, campaign.ID, contact.ID)
	if lerr != nil {
		return nil, nil, lerr
	}
	for _, c := range copies {
		// The status already applied suppression, bounces and verification.
		if !c.Copied() {
			continue
		}
		bare := strings.ToLower(strings.TrimSpace(c.Email))
		if bare == "" || seen[bare] {
			continue
		}
		seen[bare] = true
		cc = append(cc, c.Email)
	}
	return cc, bcc, nil
}
