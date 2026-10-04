package integration

import (
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

func TestRankByPopularity(t *testing.T) {
	entries := []models.IntegrationCatalogEntry{
		{Provider: models.IntegrationDiscord},
		{Provider: models.IntegrationHubSpot},
		{Provider: models.IntegrationSlack},
	}
	rankByPopularity(entries, nil)
	if entries[1].Rank != 1 || entries[2].Rank != 2 || entries[0].Rank != 3 {
		t.Fatalf("curated order not applied: %+v", entries)
	}

	rankByPopularity(entries, map[models.IntegrationProvider]int{models.IntegrationDiscord: 4, models.IntegrationSlack: 4})
	if entries[2].Rank != 1 || entries[0].Rank != 2 || entries[1].Rank != 3 {
		t.Fatalf("usage should rank first with curated ties: %+v", entries)
	}
	if entries[0].Provider != models.IntegrationDiscord {
		t.Fatal("ranking must not reorder the slice")
	}
}
