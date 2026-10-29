package landscape

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// DetailBuildInput carries Core, selection and materiality seal for Detail build.
type DetailBuildInput struct {
	Core          LandscapeSet
	Selected      []EntityRef
	SelectionHash string
}

// DetailBuilder builds Landscape Detail for Account + selected entities only.
type DetailBuilder struct {
	Source DetailDataSource
}

// BuildDetail projects Core → Detail for selected EntityRefs and attaches
// bounded supporting context (never silently truncates without metadata).
func (b DetailBuilder) BuildDetail(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	in DetailBuildInput,
) (LandscapeSet, []byte, string, error) {
	core := in.Core
	selected := in.Selected
	if sealed.Mode() != factualapi.ModeSealedUniverse {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: SEALED_UNIVERSE required")
	}
	if err := ValidateLandscapeCore(core, sealed.SellerAccountID()); err != nil {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: invalid core input: %w", err)
	}

	wantCat := map[int64]struct{}{}
	wantSKU := map[int64]struct{}{}
	wantCamp := map[int64]struct{}{}
	for _, r := range selected {
		switch r.Kind {
		case EntityCategory:
			wantCat[r.ID] = struct{}{}
		case EntitySKU:
			wantSKU[r.ID] = struct{}{}
		case EntityCampaign:
			wantCamp[r.ID] = struct{}{}
		case EntityAccount:
			// always included
		default:
			return LandscapeSet{}, nil, "", fmt.Errorf("landscape: forbidden entity kind %q", r.Kind)
		}
	}

	profiles := DefaultDetailProfilesV5()

	out := LandscapeSet{
		Context:            core.Context,
		DomainWindows:      append([]DomainWindow(nil), core.DomainWindows...),
		GlobalCapabilities: append([]CapabilityFlag(nil), core.GlobalCapabilities...),
		GlobalLimitations:  append([]Limitation(nil), core.GlobalLimitations...),
		Account:            core.Account,
	}
	out.Context.LandscapeProfileVersion = DetailProfileVersionSealV5()
	out.Context.LandscapeSchemaVersion = LandscapeSchemaV5
	out.Context.MaterialitySelectionHash = in.SelectionHash
	out.Account.Profile = profiles.Account.Name

	for _, c := range core.Categories {
		if _, ok := wantCat[c.Identity.CategoryID]; !ok {
			continue
		}
		c.Profile = profiles.Category.Name
		out.Categories = append(out.Categories, c)
	}
	for _, s := range core.SKUs {
		if _, ok := wantSKU[s.Identity.CanonicalSKUID]; !ok {
			continue
		}
		s.Profile = profiles.SKU.Name
		out.SKUs = append(out.SKUs, s)
	}
	for _, c := range core.Campaigns {
		if _, ok := wantCamp[c.Identity.CampaignID]; !ok {
			continue
		}
		c.Profile = profiles.Campaign.Name
		out.Campaigns = append(out.Campaigns, c)
	}

	skuProfile := profiles.SKU
	campProfile := profiles.Campaign

	if b.Source != nil && len(wantSKU) > 0 {
		ids := make([]int64, 0, len(wantSKU))
		for id := range wantSKU {
			ids = append(ids, id)
		}
		extras, err := b.Source.LoadSKUDetailExtras(ctx, sealed, ids, skuProfile)
		if err != nil {
			return LandscapeSet{}, nil, "", err
		}
		campaignSpec, _ := skuProfile.Supporting("sku.detail.campaign_context")
		promotionSpec, _ := skuProfile.Supporting("sku.detail.promotion_context")
		querySpec, _ := skuProfile.Supporting("sku.detail.query_context")
		for i := range out.SKUs {
			id := out.SKUs[i].Identity.CanonicalSKUID
			ex, ok := extras[id]
			if !ok {
				continue
			}
			cc := ApplySupportingTruncation(ex.CampaignContext, campaignSpec.Limit,
				nz(campaignSpec.SortBasis, "PAID_SPEND_DESC"), nz(campaignSpec.SemanticScope, "TOP_N_BY_PAID_SPEND"))
			pc := ApplySupportingTruncation(ex.PromotionContext, promotionSpec.Limit,
				nz(promotionSpec.SortBasis, "PROMOTION_ID_ASC"), nz(promotionSpec.SemanticScope, "TOP_N_ACTIVE_OR_CANDIDATE"))
			qc := ApplySupportingTruncation(ex.QueryContext, querySpec.Limit,
				nz(querySpec.SortBasis, "PROVIDER_SEARCH_RANK"), nz(querySpec.SemanticScope, "TOP_N_PROVIDER_RANKED"))
			if ex.CampaignContext.State != "" {
				out.SKUs[i].CampaignContext = &cc
			}
			if ex.PromotionContext.State != "" {
				out.SKUs[i].PromotionContext = &pc
			}
			if ex.QueryContext.State != "" {
				out.SKUs[i].QueryContext = &qc
			}
			if ex.Funnel != nil {
				out.SKUs[i].Funnel = ex.Funnel
			}
		}
	}

	var campDetailExtras map[int64]CampaignDetailExtras

	if b.Source != nil && len(wantCat) > 0 {
		ids := make([]int64, 0, len(wantCat))
		for id := range wantCat {
			ids = append(ids, id)
		}
		catExtras, err := b.Source.LoadCategoryDetailExtras(ctx, sealed, ids, profiles.Category)
		if err != nil {
			return LandscapeSet{}, nil, "", err
		}
		for i := range out.Categories {
			id := out.Categories[i].Identity.CategoryID
			ex, ok := catExtras[id]
			if !ok {
				continue
			}
			if ex.Funnel != nil {
				out.Categories[i].Funnel = ex.Funnel
			}
			if ex.Search != nil {
				out.Categories[i].Search = ex.Search
			}
			if ex.Promotions != nil {
				out.Categories[i].Promotions = ex.Promotions
			}
		}
	}

	if b.Source != nil && len(wantCamp) > 0 {
		ids := make([]int64, 0, len(wantCamp))
		for id := range wantCamp {
			ids = append(ids, id)
		}
		extras, err := b.Source.LoadCampaignDetailExtras(ctx, sealed, ids, campProfile)
		if err != nil {
			return LandscapeSet{}, nil, "", err
		}
		campDetailExtras = extras
		skuSpec, _ := campProfile.Supporting("campaign.detail.sku_context")
		for i := range out.Campaigns {
			id := out.Campaigns[i].Identity.CampaignID
			ex, ok := extras[id]
			if !ok {
				continue
			}
			sc := ApplySupportingTruncation(ex.SKUContext, skuSpec.Limit,
				nz(skuSpec.SortBasis, "PAID_SPEND_DESC"), nz(skuSpec.SemanticScope, "TOP_N_OBSERVED_PERFORMANCE"))
			out.Campaigns[i].SKUContext = &sc
		}
	}

	deriveCampaignDetailBlocks(out.Campaigns, campDetailExtras, campProfile)

	SortSet(&out)
	if err := ValidateLandscapeDetail(out, sealed.SellerAccountID()); err != nil {
		return LandscapeSet{}, nil, "", err
	}
	if err := ValidateDetailProfileConformanceV3(out, profiles); err != nil {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: profile v3 conformance: %w", err)
	}
	body, hash, err := MarshalAndHash(out)
	if err != nil {
		return LandscapeSet{}, nil, "", err
	}
	return out, body, hash, nil
}

// DetailProfileVersionSeal is the Detail landscape profile seal for contracts / input hashes.
func DetailProfileVersionSeal() string {
	return DetailProfileVersionSealV5()
}

func nz(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
