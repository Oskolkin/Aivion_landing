package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// GlobalContextAIClient generates Global Semantic Context from Data Pack input.
type GlobalContextAIClient interface {
	GenerateGlobalContext(ctx context.Context, input GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error)
	RetryGlobalContext(ctx context.Context, input GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error)
}

// NewDeterministicGlobalContextFakeClient returns a fake client for dev/tests.
func NewDeterministicGlobalContextFakeClient() GlobalContextAIClient {
	return &deterministicGlobalContextFakeClient{}
}

type deterministicGlobalContextFakeClient struct{}

func (c *deterministicGlobalContextFakeClient) GenerateGlobalContext(
	_ context.Context,
	input GlobalContextGenerationInput,
) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error) {
	ctx := buildDeterministicGlobalContext(input)
	return &ctx, &GlobalContextRawResponse{
		Model:        "fake-global-context-deterministic",
		Content:      "deterministic fixture",
		RequestID:    "fake-gc-initial",
		InputTokens:  256,
		OutputTokens: 128,
	}, nil
}

func (c *deterministicGlobalContextFakeClient) RetryGlobalContext(
	_ context.Context,
	input GlobalContextRetryInput,
) (*aicontext.GlobalSemanticContext, *GlobalContextRawResponse, error) {
	ctx := buildDeterministicGlobalContext(input.Input)
	NormalizeGlobalSemanticContext(&ctx)
	return &ctx, &GlobalContextRawResponse{
		Model:        "fake-global-context-deterministic-retry",
		Content:      "deterministic fixture retry",
		RequestID:    "fake-gc-retry",
		InputTokens:  280,
		OutputTokens: 140,
	}, nil
}

func buildDeterministicGlobalContext(input GlobalContextGenerationInput) aicontext.GlobalSemanticContext {
	if input.DataPack == nil {
		return aicontext.GlobalSemanticContext{
			SchemaVersion:   aicontext.GlobalSemanticContextSchemaVersion,
			SellerAccountID: input.SellerAccountID,
			MetricDate:      input.MetricDate,
			DataPackHash:    input.DataPackHash,
			OverallStory:    "Недостаточно backend-данных для детерминированного контекста.",
			Confidence:      "low",
			SourceFactIDs:   []string{},
		}
	}
	pack := *input.DataPack
	factIDs := deterministicFactIDs(input)
	if len(factIDs) == 0 {
		factIDs = []string{}
	}

	var primary []aicontext.SemanticRisk
	var secondary []aicontext.SemanticRisk
	priority := []string{}

	campaignID, campaignName, campaignFact := firstCampaignFromPack(pack)
	if campaignID != "" && campaignFact != "" {
		primary = append(primary, aicontext.SemanticRisk{
			RiskID:          "risk_ad_001",
			Category:        "advertising",
			Severity:        "high",
			Title:           "Рекламные кампании требуют контроля ДРР",
			Rationale:       "Есть кампании с заметным расходом и сигналами эффективности на уровне кампаний.",
			EvidenceFactIDs: []string{campaignFact},
			Entities: []aicontext.SemanticEntityRef{
				{EntityType: "campaign", EntityID: campaignID, Name: campaignName, FactIDs: []string{campaignFact}},
			},
		})
		priority = append(priority, "advertising")
		if !containsString(factIDs, campaignFact) {
			factIDs = append(factIDs, campaignFact)
		}
	}

	skuID, skuName, skuFact := firstSKUFromPack(pack)
	if skuID != "" && skuFact != "" {
		secondary = append(secondary, aicontext.SemanticRisk{
			RiskID:          "risk_stock_001",
			Category:        "stock",
			Severity:        "medium",
			Title:           "Риск по остаткам ключевых SKU",
			Rationale:       "Есть SKU с сигналами дефицита или короткого покрытия.",
			EvidenceFactIDs: []string{skuFact},
			Entities: []aicontext.SemanticEntityRef{
				{EntityType: "sku", EntityID: skuID, Name: skuName, FactIDs: []string{skuFact}},
			},
		})
		priority = appendUnique(priority, "stock")
		if !containsString(factIDs, skuFact) {
			factIDs = append(factIDs, skuFact)
		}
	}

	if len(priority) == 0 {
		if accountFact := firstAccountFact(pack); accountFact != "" {
			primary = append(primary, aicontext.SemanticRisk{
				RiskID:          "risk_sales_001",
				Category:        "sales",
				Severity:        "medium",
				Title:           "Динамика продаж аккаунта требует внимания",
				Rationale:       "Есть факты по динамике продаж аккаунта.",
				EvidenceFactIDs: []string{accountFact},
				Entities: []aicontext.SemanticEntityRef{
					{EntityType: "account", EntityID: "account:seller", FactIDs: []string{accountFact}},
				},
			})
			priority = append(priority, "sales")
			if !containsString(factIDs, accountFact) {
				factIDs = append(factIDs, accountFact)
			}
		}
	}
	priority = appendUnique(priority, "sku")

	story := "Кабинет требует внимания по приоритетным направлениям на основе доступных фактов Data Pack."
	if len(primary) > 0 {
		story = "Кабинет под давлением из-за рекламных и операционных сигналов; приоритет — контроль кампаний и остатков."
	}

	limitations := append([]string(nil), input.DataQualityLimitations...)
	if len(limitations) == 0 && input.DataPack != nil {
		limitations = append(limitations, input.DataPack.DataQuality.Limitations...)
	}
	if input.DataPack != nil && len(input.DataPack.Tables.CampaignRollups) > 0 {
		limitations = append(limitations, "Рекламная аналитика доступна на уровне кампаний, без точной SKU-level атрибуции.")
	}
	limitations = compactStrings(limitations)

	return aicontext.GlobalSemanticContext{
		SchemaVersion:   aicontext.GlobalSemanticContextSchemaVersion,
		SellerAccountID: input.SellerAccountID,
		MetricDate:      input.MetricDate,
		DataPackHash:    input.DataPackHash,
		OverallStory:    story,
		PrimaryRisks:    primary,
		SecondaryRisks:  secondary,
		Opportunities:   nil,
		PriorityOrder:   priority,
		DataLimitations: limitations,
		Confidence:      "medium",
		SourceFactIDs:   factIDs,
	}
}

func deterministicFactIDs(input GlobalContextGenerationInput) []string {
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		ids := make([]string, 0, len(input.FactCatalog))
		for id := range input.FactCatalog {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 3 {
			ids = ids[:3]
		}
		if len(ids) == 0 {
			return []string{}
		}
		return ids
	}
	if input.DataPack == nil {
		return []string{}
	}
	return sortedFactIDs(input.DataPack.FactRegistry, 3)
}

func sortedFactIDs(registry map[string]aidatapack.Fact, limit int) []string {
	if len(registry) == 0 {
		return nil
	}
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

func firstCampaignFromPack(pack aidatapack.SellerAIDataPack) (entityID, name, factID string) {
	for id, ent := range pack.EntityRegistry.Campaigns {
		for factKey := range pack.FactRegistry {
			if strings.Contains(factKey, "campaign") {
				return id, ent.Name, factKey
			}
		}
	}
	for factKey := range pack.FactRegistry {
		if strings.Contains(factKey, "campaign") {
			return "", "", factKey
		}
	}
	return "", "", ""
}

func firstSKUFromPack(pack aidatapack.SellerAIDataPack) (entityID, name, factID string) {
	for id, ent := range pack.EntityRegistry.SKUs {
		for factKey := range pack.FactRegistry {
			if strings.Contains(factKey, "sku") || strings.Contains(factKey, "stock") {
				return id, ent.Name, factKey
			}
		}
	}
	return "", "", ""
}

func firstAccountFact(pack aidatapack.SellerAIDataPack) string {
	for factKey := range pack.FactRegistry {
		if strings.HasPrefix(factKey, "account.") {
			return factKey
		}
	}
	return ""
}

func appendUnique(list []string, v string) []string {
	if containsString(list, v) {
		return list
	}
	return append(list, v)
}

// ParseGlobalSemanticContext parses raw JSON into GlobalSemanticContext.
func ParseGlobalSemanticContext(raw string) (*aicontext.GlobalSemanticContext, error) {
	normalized, err := normalizeGlobalContextJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	var wire struct {
		SchemaVersion   string          `json:"schema_version"`
		SellerAccountID int64           `json:"seller_account_id"`
		MetricDate      string          `json:"metric_date"`
		DataPackHash    string          `json:"data_pack_hash"`
		OverallStory    string          `json:"overall_story"`
		PrimaryRisks    json.RawMessage `json:"primary_risks"`
		SecondaryRisks  json.RawMessage `json:"secondary_risks"`
		Opportunities   json.RawMessage `json:"opportunities"`
		PriorityOrder   json.RawMessage `json:"priority_order"`
		DataLimitations json.RawMessage `json:"data_limitations"`
		Confidence      string          `json:"confidence"`
		SourceFactIDs   json.RawMessage `json:"source_fact_ids"`
	}
	if err := json.Unmarshal(normalized, &wire); err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion:   wire.SchemaVersion,
		SellerAccountID: wire.SellerAccountID,
		MetricDate:      wire.MetricDate,
		DataPackHash:    wire.DataPackHash,
		OverallStory:    wire.OverallStory,
		PrimaryRisks:    parseFlexibleSemanticRisks(wire.PrimaryRisks),
		SecondaryRisks:  parseFlexibleSemanticRisks(wire.SecondaryRisks),
		Opportunities:   parseFlexibleSemanticSignals(wire.Opportunities),
		PriorityOrder:   parseFlexibleStringList(wire.PriorityOrder),
		DataLimitations: parseFlexibleStringSliceField(wire.DataLimitations),
		Confidence:      wire.Confidence,
		SourceFactIDs:   parseFlexibleStringList(wire.SourceFactIDs),
	}
	NormalizeGlobalSemanticContext(&ctx)
	return &ctx, nil
}
