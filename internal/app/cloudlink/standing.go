package cloudlink

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// knownHealthState keeps an unrecognised state from a newer cloud out of the gates.
func knownHealthState(state string) bool {
	switch models.WarmupHealthState(state) {
	case models.WarmupHealthHealthy, models.WarmupHealthWatch, models.WarmupHealthThrottled,
		models.WarmupHealthQuarantined, models.WarmupHealthBlocked:
		return true
	}
	return false
}

// recordStanding stores what the cloud reported. An absent standing leaves
// the recorded one in place: a cloud that could not read it has not lifted it.
func (s *service) recordStanding(ctx context.Context, accountID uuid.UUID, h *models.WarmupHealthInfo) (models.WarmupHealthState, bool) {
	if h == nil || !knownHealthState(h.State) {
		return "", false
	}
	prev, err := s.repo.SetStanding(ctx, accountID, h)
	if err != nil {
		log.Warn().Err(err).Str("account_id", accountID.String()).Msg("cloud link: warmup standing could not be recorded")
		return "", false
	}
	return prev, true
}

// carryStanding keeps a cloud quarantine or block in force on the local pool
// row a mailbox rejoins when it leaves the cloud.
func (s *service) carryStanding(ctx context.Context, m models.CloudLinkMailbox) {
	h := m.Standing
	if h == nil || h.BlockedUntil == nil || !h.BlockedUntil.After(time.Now()) {
		return
	}
	if st := models.WarmupHealthState(h.State); st != models.WarmupHealthQuarantined && st != models.WarmupHealthBlocked {
		return
	}
	if err := s.repo.CarryStanding(ctx, m.EmailAccountID, h); err != nil {
		log.Warn().Err(err).Str("account_id", m.EmailAccountID.String()).Msg("cloud link: warmup standing could not be carried to the local pool")
	}
}

func (s *service) SyncStanding(ctx context.Context) ([]models.CloudLinkStandingChange, *errx.Error) {
	l, err := s.repo.Get(ctx)
	if err != nil {
		return nil, errx.InternalError()
	}
	if l == nil {
		return nil, nil
	}
	enrolled, err := s.repo.List(ctx)
	if err != nil {
		return nil, errx.InternalError()
	}
	if len(enrolled) == 0 {
		return nil, nil
	}
	byRemote, xerr := s.fetchStanding(ctx, l)
	if xerr != nil {
		return nil, xerr
	}
	var changes []models.CloudLinkStandingChange
	for _, m := range enrolled {
		h := byRemote[m.RemoteID]
		prev, ok := s.recordStanding(ctx, m.EmailAccountID, h)
		if !ok || prev == "" || prev == models.WarmupHealthState(h.State) {
			continue
		}
		changes = append(changes, models.CloudLinkStandingChange{
			EmailAccountID: m.EmailAccountID,
			Previous:       prev,
			Current:        models.WarmupHealthState(h.State),
			Reason:         h.Reason,
		})
	}
	return changes, nil
}

// fetchStanding reads every enrolled mailbox's standing, falling back to the
// full mailbox listing on a cloud that predates the standing route.
func (s *service) fetchStanding(ctx context.Context, l *models.CloudLink) (map[uuid.UUID]*models.WarmupHealthInfo, *errx.Error) {
	out := map[uuid.UUID]*models.WarmupHealthInfo{}
	var standing []models.PoolLinkMailboxStanding
	xerr := s.clientFor(l).do(ctx, http.MethodGet, "/instance/standing", nil, &standing)
	if xerr == nil {
		for _, st := range standing {
			out[st.RemoteID] = st.Health
		}
		return out, nil
	}
	if xerr.Code != errx.NotFound || strings.HasPrefix(xerr.Identifier, "pool_link_") {
		return nil, xerr
	}
	var states []models.PoolLinkMailboxState
	if xerr := s.clientFor(l).do(ctx, http.MethodGet, "/instance/mailboxes", nil, &states); xerr != nil {
		return nil, xerr
	}
	for i := range states {
		out[states[i].RemoteID] = states[i].Health
	}
	return out, nil
}
