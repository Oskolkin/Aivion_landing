package landscape

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// MemSource is an in-memory CoreDataSource + DetailDataSource for unit tests.
type MemSource struct {
	Feed      CoreFeed
	SKUExtra  map[int64]SKUDetailExtras
	CatExtra  map[int64]CategoryDetailExtras
	CampExtra map[int64]CampaignDetailExtras
	Err       error
}

func (m MemSource) LoadCoreFeed(_ context.Context, sealed factualapi.FactualReadContext, _, _ string, _ CoreProfileBundleV3) (CoreFeed, error) {
	if m.Err != nil {
		return CoreFeed{}, m.Err
	}
	if sealed.Mode() != factualapi.ModeSealedUniverse {
		return CoreFeed{}, fmt.Errorf("landscape: MemSource requires SEALED_UNIVERSE")
	}
	return m.Feed, nil
}

func (m MemSource) LoadSKUDetailExtras(_ context.Context, _ factualapi.FactualReadContext, skuIDs []int64, _ ExecutableProfileV3) (map[int64]SKUDetailExtras, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := map[int64]SKUDetailExtras{}
	for _, id := range skuIDs {
		if x, ok := m.SKUExtra[id]; ok {
			out[id] = x
		}
	}
	return out, nil
}

func (m MemSource) LoadCategoryDetailExtras(_ context.Context, _ factualapi.FactualReadContext, categoryIDs []int64, _ ExecutableProfileV3) (map[int64]CategoryDetailExtras, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := map[int64]CategoryDetailExtras{}
	for _, id := range categoryIDs {
		if x, ok := m.CatExtra[id]; ok {
			out[id] = x
		}
	}
	return out, nil
}

func (m MemSource) LoadCampaignDetailExtras(_ context.Context, _ factualapi.FactualReadContext, campaignIDs []int64, _ ExecutableProfileV3) (map[int64]CampaignDetailExtras, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := map[int64]CampaignDetailExtras{}
	for _, id := range campaignIDs {
		if x, ok := m.CampExtra[id]; ok {
			out[id] = x
		}
	}
	return out, nil
}
