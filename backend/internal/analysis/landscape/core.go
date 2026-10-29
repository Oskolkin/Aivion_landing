package landscape

import (
	"context"
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// CoreBuilder builds Landscape Core sets from a CoreDataSource.
type CoreBuilder struct {
	Source   CoreDataSource
	Profiles CoreProfileBundleV4
}

// BuildCore assembles, validates, canonicalizes and hashes a Core LandscapeSet.
func (b CoreBuilder) BuildCore(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	universeID, universeHash, contractBundleHash string,
) (LandscapeSet, []byte, string, error) {
	if b.Source == nil {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: CoreDataSource required")
	}
	if sealed.Mode() != factualapi.ModeSealedUniverse {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: SEALED_UNIVERSE required")
	}
	if universeID == "" || universeHash == "" || contractBundleHash == "" {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: universe id/hash and contract bundle hash required")
	}
	if sealed.UniverseID() != "" && sealed.UniverseID() != universeID {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: universe id mismatch")
	}

	profiles := b.Profiles
	if profiles.IsZero() {
		profiles = DefaultCoreProfilesV5()
	}

	feed, err := b.Source.LoadCoreFeed(ctx, sealed, universeID, universeHash, profiles)
	if err != nil {
		return LandscapeSet{}, nil, "", err
	}
	deriveAccountBlocks(&feed, profiles.Account, sealed)
	deriveCategoryBlocks(&feed, profiles.Category)
	stampAdvertisingPeriodsFromWindows(&feed)

	set := LandscapeSet{
		Context: LandscapeContext{
			AnalysisDate:            canonicaljson.Date(sealed.AnalysisDate()),
			AnalysisCutoffAt:        sealed.CutoffAt().UTC(),
			SellerAccountID:         sealed.SellerAccountID(),
			FactualUniverseID:       universeID,
			FactualUniverseHash:     universeHash,
			ContractBundleHash:      contractBundleHash,
			LandscapeSchemaVersion:  LandscapeSchemaV5,
			LandscapeProfileVersion: CoreProfileVersionSealV5(),
			BusinessTimezone:        "Europe/Moscow",
			ContinuityMode:          ContinuityInitialNoBaseV1,
		},
		DomainWindows:      append([]DomainWindow(nil), feed.DomainWindows...),
		GlobalCapabilities: append([]CapabilityFlag(nil), feed.GlobalCapabilities...),
		GlobalLimitations:  append([]Limitation(nil), feed.GlobalLimitations...),
		Account:            feed.Account,
		Categories:         append([]CategoryLandscape(nil), feed.Categories...),
		SKUs:               append([]SKULandscape(nil), feed.SKUs...),
		Campaigns:          append([]CampaignLandscape(nil), feed.Campaigns...),
	}
	if set.Account.Profile == "" {
		set.Account.Profile = profiles.Account.Name
	}
	set.Account.Identity.SellerAccountID = sealed.SellerAccountID()
	normalizeCoreProfilesV3(&set, profiles)
	SortSet(&set)

	if err := ValidateLandscapeCore(set, sealed.SellerAccountID()); err != nil {
		return LandscapeSet{}, nil, "", err
	}
	if err := ValidateCoreProfileConformanceV3(set, profiles); err != nil {
		return LandscapeSet{}, nil, "", fmt.Errorf("landscape: profile v3 conformance: %w", err)
	}
	body, hash, err := MarshalAndHash(set)
	if err != nil {
		return LandscapeSet{}, nil, "", err
	}
	return set, body, hash, nil
}

// CoreProfileVersionSeal is the Core landscape profile seal for contracts / input hashes.
func CoreProfileVersionSeal() string {
	return CoreProfileVersionSealV5()
}

func normalizeCoreProfilesV3(set *LandscapeSet, profiles CoreProfileBundleV3) {
	set.Account.Profile = profiles.Account.Name
	for i := range set.Categories {
		set.Categories[i].Profile = profiles.Category.Name
	}
	for i := range set.SKUs {
		set.SKUs[i].Profile = profiles.SKU.Name
		set.SKUs[i].CampaignContext = nil
		set.SKUs[i].PromotionContext = nil
		set.SKUs[i].QueryContext = nil
	}
	for i := range set.Campaigns {
		set.Campaigns[i].Profile = profiles.Campaign.Name
		set.Campaigns[i].SKUContext = nil
	}
}

func normalizeCoreProfiles(set *LandscapeSet) {
	normalizeCoreProfilesV3(set, DefaultCoreProfilesV5())
}

// SortSet applies stable entity ordering by semantic key.
func SortSet(set *LandscapeSet) {
	sort.Slice(set.DomainWindows, func(i, j int) bool {
		a := set.DomainWindows[i]
		b := set.DomainWindows[j]
		if a.Domain != b.Domain {
			return a.Domain < b.Domain
		}
		return a.FactFamily < b.FactFamily
	})
	sort.Slice(set.GlobalCapabilities, func(i, j int) bool {
		if set.GlobalCapabilities[i].Key != set.GlobalCapabilities[j].Key {
			return set.GlobalCapabilities[i].Key < set.GlobalCapabilities[j].Key
		}
		return set.GlobalCapabilities[i].Scope < set.GlobalCapabilities[j].Scope
	})
	sort.Slice(set.GlobalLimitations, func(i, j int) bool {
		return set.GlobalLimitations[i].Code < set.GlobalLimitations[j].Code
	})
	sort.Slice(set.Categories, func(i, j int) bool {
		return set.Categories[i].Identity.CategoryID < set.Categories[j].Identity.CategoryID
	})
	sort.Slice(set.SKUs, func(i, j int) bool {
		return set.SKUs[i].Identity.CanonicalSKUID < set.SKUs[j].Identity.CanonicalSKUID
	})
	sort.Slice(set.Campaigns, func(i, j int) bool {
		return set.Campaigns[i].Identity.CampaignID < set.Campaigns[j].Identity.CampaignID
	})
}
