package universe

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

const (
	entityKindSKU             = "SKU"
	entityKindCampaign        = "CAMPAIGN"
	entityKindCategoryMapping = "CATEGORY_MAPPING"
	unmappedCategorySemantic  = "UNMAPPED"
	entitySetPageSize         = 200
)

// factualEntitySource is the Factual API surface required to freeze Universe entity sets.
// Kept narrow so unit tests can stub without implementing the full factualapi.API.
type factualEntitySource interface {
	ListSKUs(context.Context, factualapi.FactualReadContext, factualapi.SKUFilter, factualapi.PageRequest) (factualapi.Page[factualapi.SKU], error)
	ListCampaigns(context.Context, factualapi.FactualReadContext, factualapi.CampaignFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Campaign], error)
	ListCategories(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.Category], error)
}

// BuildEntitySets freezes SKU / campaign / category-mapping semantic members via Factual API
// under the exact resolved read context (never direct canonical SQL).
func BuildEntitySets(ctx context.Context, api factualapi.API, rc factualapi.FactualReadContext) ([]EntitySet, error) {
	if api == nil {
		return nil, fmt.Errorf("universe: BuildEntitySets requires factualapi.API")
	}
	return buildEntitySets(ctx, api, rc)
}

func buildEntitySets(ctx context.Context, api factualEntitySource, rc factualapi.FactualReadContext) ([]EntitySet, error) {
	cats, err := listAllCategories(ctx, api, rc)
	if err != nil {
		return nil, err
	}
	providerBySellerCat := make(map[int64]int64, len(cats))
	for _, c := range cats {
		providerBySellerCat[c.ID] = c.ProviderCategoryID
	}

	skus, err := listAllSKUs(ctx, api, rc)
	if err != nil {
		return nil, err
	}
	skuMembers := make([]string, 0, len(skus))
	mappingMembers := make([]string, 0, len(skus))
	for _, sk := range skus {
		if sk.OzonSKU == 0 {
			continue
		}
		ozon := strconv.FormatInt(sk.OzonSKU, 10)
		skuMembers = append(skuMembers, ozon)
		mappingMembers = append(mappingMembers, categoryMappingMember(sk, providerBySellerCat))
	}

	camps, err := listAllCampaigns(ctx, api, rc)
	if err != nil {
		return nil, err
	}
	campMembers := make([]string, 0, len(camps))
	for _, c := range camps {
		if c.ProviderCampaignID == 0 {
			continue
		}
		campMembers = append(campMembers, strconv.FormatInt(c.ProviderCampaignID, 10))
	}

	return []EntitySet{
		newEntitySet(entityKindSKU, skuMembers),
		newEntitySet(entityKindCampaign, campMembers),
		newEntitySet(entityKindCategoryMapping, mappingMembers),
	}, nil
}

func categoryMappingMember(sk factualapi.SKU, providerBySellerCat map[int64]int64) string {
	ozon := strconv.FormatInt(sk.OzonSKU, 10)
	if sk.Category.MappingStatus == "UNMAPPED" || sk.Category.CategoryID == nil {
		return unmappedCategorySemantic + ":" + ozon
	}
	providerID, ok := providerBySellerCat[*sk.Category.CategoryID]
	if !ok || providerID == 0 {
		return unmappedCategorySemantic + ":" + ozon
	}
	// Semantic leaf category = provider (Ozon) category id — never seller_category_id / canonical_sku_id.
	return strconv.FormatInt(providerID, 10) + ":" + ozon
}

func newEntitySet(kind string, members []string) EntitySet {
	sort.Strings(members)
	return EntitySet{
		EntityKind:  kind,
		Members:     members,
		MemberCount: int64(len(members)),
		SetHash:     canonicaljson.Hash(members),
	}
}

func listAllSKUs(ctx context.Context, api factualEntitySource, rc factualapi.FactualReadContext) ([]factualapi.SKU, error) {
	var out []factualapi.SKU
	token := ""
	for {
		page, err := api.ListSKUs(ctx, rc, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: entitySetPageSize, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

func listAllCampaigns(ctx context.Context, api factualEntitySource, rc factualapi.FactualReadContext) ([]factualapi.Campaign, error) {
	var out []factualapi.Campaign
	token := ""
	for {
		page, err := api.ListCampaigns(ctx, rc, factualapi.CampaignFilter{}, factualapi.PageRequest{PageSize: entitySetPageSize, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

func listAllCategories(ctx context.Context, api factualEntitySource, rc factualapi.FactualReadContext) ([]factualapi.Category, error) {
	var out []factualapi.Category
	token := ""
	for {
		page, err := api.ListCategories(ctx, rc, factualapi.PageRequest{PageSize: entitySetPageSize, PageToken: token})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if page.NextPageToken == "" {
			return out, nil
		}
		token = page.NextPageToken
	}
}

// ReadContextFromResolved builds a ModeLatestStable FactualReadContext from the accepted
// RESOLVED_CONTEXT manifest (exact cutoff/bindings). Used by SEAL_UNIVERSE before a sealed
// universe exists — never re-resolves "latest".
func ReadContextFromResolved(m resolve.Manifest) (factualapi.FactualReadContext, error) {
	if m.SellerAccountID <= 0 || m.CutoffAt.IsZero() || time.Time(m.AnalysisDate).IsZero() {
		return factualapi.FactualReadContext{}, fmt.Errorf("universe: resolved context missing seller/analysis_date/cutoff")
	}
	policies := m.Policies
	if policies.API == "" {
		policies = factualapi.CurrentPolicies()
	}

	seenKey := map[string]bool{}
	var domains []factualapi.DomainBinding
	for _, s := range m.Sources {
		if s.FactFamily == "" {
			continue
		}
		key := factualapi.FactFamilyKey{Domain: s.Domain, FactFamily: s.FactFamily}.String()
		if seenKey[key] {
			continue
		}
		var runID int64
		if s.LastAcceptedDomainRunID != nil {
			runID = *s.LastAcceptedDomainRunID
		}
		if runID <= 0 {
			continue
		}
		seenKey[key] = true
		var finalizedAt *time.Time
		if s.SourceFinalizedAt != nil {
			x := s.SourceFinalizedAt.UTC()
			finalizedAt = &x
		}
		domains = append(domains, factualapi.DomainBinding{
			Domain: s.Domain, FactFamily: s.FactFamily, DomainRunID: runID, FinalizedAt: finalizedAt,
			ContractKey: s.SourceContractKey, ContractVersion: s.SourceContractVersion, ContractHash: s.SourceContractHash,
			WatermarkHash: s.WatermarkHash,
		})
	}

	var caps []factualapi.CapabilityBinding
	for _, c := range m.Capabilities {
		var aid int64
		if c.AssessmentID != nil {
			aid = *c.AssessmentID
		}
		if aid <= 0 {
			continue
		}
		assessed := c.AssessedAt
		if assessed.After(m.CutoffAt) {
			assessed = m.CutoffAt
		}
		caps = append(caps, factualapi.CapabilityBinding{
			CapabilityKey: c.Key, Scope: c.Scope, FinalState: c.FinalState, ReasonCode: c.ReasonCode,
			AssessmentID: aid, AssessedAt: assessed,
			ContractState: c.ContractState, AccessState: c.AccessState, SourceState: c.SourceState, SemanticState: c.SemanticState,
			CoverageState: c.CoverageState, TemporalState: c.TemporalState, HistoryState: c.HistoryState, PopulationState: c.PopulationState,
		})
	}

	return factualapi.NewReadContext(factualapi.ContextSpec{
		SellerAccountID: m.SellerAccountID,
		AnalysisDate:    time.Time(m.AnalysisDate),
		CutoffAt:        m.CutoffAt,
		Mode:            factualapi.ModeLatestStable,
		Policies:        policies,
		Domains:         domains,
		Capabilities:    caps,
	})
}
