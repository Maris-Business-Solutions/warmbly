package integration

import (
	"context"
	"sort"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

// popularityTTL bounds how stale the instance-wide usage counts may be.
const popularityTTL = 10 * time.Minute

// curatedOrder breaks ties, so a new instance with no connections still lists
// the integrations most outbound teams reach for first.
var curatedOrder = []models.IntegrationProvider{
	models.IntegrationHubSpot,
	models.IntegrationSlack,
	models.IntegrationCalendly,
	models.IntegrationZapier,
	models.IntegrationSalesforce,
	models.IntegrationPipedrive,
	models.IntegrationMake,
	models.IntegrationN8N,
	models.IntegrationCalCom,
	models.IntegrationClose,
	models.IntegrationMillionVerifier,
	models.IntegrationDiscord,
	models.IntegrationCleanMyList,
}

// popularity returns workspaces per provider, cached; a failed read keeps the
// last good counts and falls back to the curated order alone.
func (s *service) popularity(ctx context.Context) map[models.IntegrationProvider]int {
	s.popMu.Lock()
	defer s.popMu.Unlock()
	if s.pop != nil && time.Since(s.popAt) < popularityTTL {
		return s.pop
	}
	counts, err := s.repo.WorkspacesByProvider(ctx)
	if err != nil {
		return s.pop
	}
	s.pop, s.popAt = counts, time.Now()
	return counts
}

// rankByPopularity sets each entry's Rank without reordering the slice.
func rankByPopularity(entries []models.IntegrationCatalogEntry, counts map[models.IntegrationProvider]int) {
	curated := make(map[models.IntegrationProvider]int, len(curatedOrder))
	for i, p := range curatedOrder {
		curated[p] = i
	}
	tie := func(p models.IntegrationProvider) int {
		if i, ok := curated[p]; ok {
			return i
		}
		return len(curatedOrder)
	}
	idx := make([]int, len(entries))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		pa, pb := entries[idx[a]].Provider, entries[idx[b]].Provider
		if counts[pa] != counts[pb] {
			return counts[pa] > counts[pb]
		}
		return tie(pa) < tie(pb)
	})
	for rank, i := range idx {
		entries[i].Rank = rank + 1
	}
}
