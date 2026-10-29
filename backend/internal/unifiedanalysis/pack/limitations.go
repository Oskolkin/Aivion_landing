package pack

import (
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Frozen v1 limitation codes. Keep in sync with data_dictionary.json null_semantics text.
const (
	LimitationSKUFunnelUnavailable       = "sku_funnel_metrics_unavailable"
	LimitationSKUOrderQualityUnavailable = "sku_order_quality_unavailable"
	LimitationSKUReservedStockUnknown    = "sku_reserved_stock_unknown"
	LimitationSKUAdSpendNotAttributed    = "sku_ad_spend_not_attributed"
	LimitationCampaignSKUSpendUnknown    = "campaign_sku_spend_not_attributed"
	LimitationContentScoreUnavailable    = "product_content_score_unavailable"
	LimitationAttributeCompletionUnknown = "product_attribute_completion_unknown"
	LimitationAccountDataUnavailable     = "account_metrics_unavailable"
	LimitationFunnelDataUnavailable      = "account_funnel_unavailable"
	LimitationEnrichmentUnavailable      = "product_enrichment_unavailable"
)

// limitationDefaults describes severity/description/affected tables for each frozen code.
var limitationDefaults = map[string]contract.LimitationV1{
	LimitationSKUFunnelUnavailable: {
		Severity:       "medium",
		Description:    "Per-SKU funnel metrics (impressions, views, add_to_cart) are not tracked; only account-level funnel data is available.",
		AffectedTables: []string{"sku_summary.csv", "sku_daily.csv"},
	},
	LimitationSKUOrderQualityUnavailable: {
		Severity:       "medium",
		Description:    "Per-SKU delivered/cancelled/returns breakdown is not tracked; only order count and revenue are available at SKU granularity.",
		AffectedTables: []string{"sku_summary.csv", "sku_daily.csv"},
	},
	LimitationSKUReservedStockUnknown: {
		Severity:       "low",
		Description:    "Reserved stock is not tracked as a distinct field; only available and in-transit stock are known.",
		AffectedTables: []string{"sku_summary.csv", "sku_daily.csv"},
	},
	LimitationSKUAdSpendNotAttributed: {
		Severity:       "medium",
		Description:    "Advertising spend/revenue/orders are tracked at campaign level only; per-SKU attribution is not distributed and is left null.",
		AffectedTables: []string{"sku_summary.csv"},
	},
	LimitationCampaignSKUSpendUnknown: {
		Severity:       "medium",
		Description:    "campaign_sku.csv records campaign-to-SKU links only; per-pair spend/revenue/orders are not available and are left null.",
		AffectedTables: []string{"campaign_sku.csv"},
	},
	LimitationContentScoreUnavailable: {
		Severity:       "low",
		Description:    "Content score is not computed by any upstream source; the field is left null.",
		AffectedTables: []string{"sku_summary.csv", "product_enrichment.csv"},
	},
	LimitationAttributeCompletionUnknown: {
		Severity:       "low",
		Description:    "Attribute completion percentage is not computed by any upstream source; the field is left null.",
		AffectedTables: []string{"sku_summary.csv", "product_enrichment.csv"},
	},
	LimitationAccountDataUnavailable: {
		Severity:       "high",
		Description:    "daily_account_metrics rows were not found for the requested data window.",
		AffectedTables: []string{"account_summary.csv", "account_daily.csv"},
	},
	LimitationFunnelDataUnavailable: {
		Severity:       "medium",
		Description:    "product_analytics_daily account-scope rows were not found for the requested data window.",
		AffectedTables: []string{"account_summary.csv", "account_daily.csv"},
	},
	LimitationEnrichmentUnavailable: {
		Severity:       "high",
		Description:    "Ozon product/stock enrichment sync failed or has not run for this metric date.",
		AffectedTables: []string{"sku_summary.csv", "product_enrichment.csv"},
	},
}

// limitationCollector deduplicates limitation codes observed while building a pack.
type limitationCollector struct {
	codes map[string]struct{}
}

func newLimitationCollector() *limitationCollector {
	return &limitationCollector{codes: map[string]struct{}{}}
}

// Add records a frozen limitation code (no-op if the code is unknown or already added).
func (c *limitationCollector) Add(code string) {
	if _, ok := limitationDefaults[code]; !ok {
		return
	}
	c.codes[code] = struct{}{}
}

// Build returns the limitations document sorted by code ascending.
func (c *limitationCollector) Build() []contract.LimitationV1 {
	codes := make([]string, 0, len(c.codes))
	for code := range c.codes {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]contract.LimitationV1, 0, len(codes))
	for _, code := range codes {
		def := limitationDefaults[code]
		def.Code = code
		out = append(out, def)
	}
	return out
}

// BuildLimitationsDocument renders the limitations.json contract payload.
func BuildLimitationsDocument(limitations []contract.LimitationV1, coverage ...[]contract.DatasetCoverageV1) contract.LimitationsDocumentV1 {
	doc := contract.LimitationsDocumentV1{
		SchemaVersion: contract.AnalysisPackSchemaVersionV1,
		Limitations:   limitations,
	}
	if len(coverage) > 0 {
		doc.Coverage = coverage[0]
	}
	return doc
}

// BuildDailyCoverageMetadata describes sparse daily coverage vs the trend window.
// Gap dates are days inside the window with no observed account/funnel/ad rows.
func BuildDailyCoverageMetadata(idx *Indexes, periods Periods) []contract.DatasetCoverageV1 {
	window := dateRangeInclusive(periods.Trend.From, periods.Trend.To)
	accountDates := 0
	funnelDates := 0
	adDates := 0
	var accountGaps, funnelGaps []string
	for _, d := range window {
		if _, ok := idx.AccountMetricsByDate[d]; ok {
			accountDates++
		} else {
			accountGaps = append(accountGaps, d)
		}
		if _, ok := idx.FunnelByDate[d]; ok {
			funnelDates++
		} else {
			funnelGaps = append(funnelGaps, d)
		}
		hasAd := false
		for _, campaignID := range idx.CampaignUniverse {
			if _, ok := idx.CampaignMetricsByDate[campaignID][d]; ok {
				hasAd = true
				break
			}
		}
		if hasAd {
			adDates++
		}
	}
	capGaps := func(gaps []string) []string {
		const maxGaps = 31
		if len(gaps) > maxGaps {
			return append([]string(nil), gaps[:maxGaps]...)
		}
		return gaps
	}
	return []contract.DatasetCoverageV1{
		{
			Dataset:   "account_daily",
			Status:    sparseStatus(accountDates, len(window)),
			DatesWith: accountDates,
			GapDates:  capGaps(accountGaps),
			Note:      "Missing calendar days are omitted from account_daily.csv; they are not synthesized as zeros.",
		},
		{
			Dataset:   "account_funnel_daily",
			Status:    sparseStatus(funnelDates, len(window)),
			DatesWith: funnelDates,
			GapDates:  capGaps(funnelGaps),
			Note:      "Funnel gaps are omitted; absence is not treated as zero impressions/views.",
		},
		{
			Dataset:   "campaign_daily",
			Status:    sparseStatus(adDates, len(window)),
			DatesWith: adDates,
			Note:      "Campaign daily series is sparse (observed ad_metrics_daily rows only).",
		},
		{
			Dataset: "sku_daily",
			Status:  "sparse",
			Note:    "SKU daily series is sparse (observed daily_sku_metrics rows only).",
		},
	}
}

func sparseStatus(present, window int) string {
	if window <= 0 {
		return "missing"
	}
	if present == 0 {
		return "missing"
	}
	if present < window {
		return "sparse"
	}
	return "present"
}
