package aidatapack

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

// DataPackAvailabilitySplit reports total vs available rows in a table.
type DataPackAvailabilitySplit struct {
	Total        int `json:"total"`
	Available    int `json:"available"`
	NotAvailable int `json:"not_available"`
}

type AuditReport struct {
	SchemaVersion string `json:"schema_version"`
	Valid         bool   `json:"valid"`
	SellerID      int64  `json:"seller_id"`
	MetricDate    string `json:"metric_date"`
	Counts        struct {
		SKURollups           int `json:"sku_rollups"`
		StockPositions       int `json:"stock_positions"`
		ProductEnrichment    int `json:"product_enrichment"`
		CampaignRollups      int `json:"campaign_rollups"`
		CampaignProductLinks int `json:"campaign_product_links"`
		FactRegistry         int `json:"fact_registry"`
	} `json:"counts"`
	Shape struct {
		FactRegistryType     string `json:"fact_registry_type"`
		ConstraintsSource    string `json:"constraints_source"`
		TablesHasConstraints bool   `json:"tables_has_constraints"`
	} `json:"shape"`
	Quality struct {
		ProductEnrichmentSource string `json:"product_enrichment_source,omitempty"`
		FinanceStatus           string `json:"finance_status"`
		HasSecretFields         bool   `json:"has_secret_fields"`
		HasBusinessConclusions  bool   `json:"has_business_conclusions"`
	} `json:"quality"`
	CampaignScenarios struct {
		HighDRRCount            int     `json:"high_drr_count"`
		ElevatedDRRCount        int     `json:"elevated_drr_count"`
		HighDRRThresholdPct     float64 `json:"high_drr_threshold_pct"`
		ElevatedDRRThresholdPct float64 `json:"elevated_drr_threshold_pct"`
		SpendWithoutOrdersCount int     `json:"spend_without_orders_count"`
		HealthyCount            int     `json:"healthy_count"`
		InactiveByStateCount    int     `json:"inactive_by_state_count"`
		NoActivityLast7dCount   int     `json:"no_activity_last_7d_count"`
	} `json:"campaign_scenarios"`
	Availability struct {
		StockPositions    DataPackAvailabilitySplit `json:"stock_positions"`
		ProductEnrichment DataPackAvailabilitySplit `json:"product_enrichment"`
		SKURollups        DataPackAvailabilitySplit `json:"sku_rollups,omitempty"`
		CampaignRollups   DataPackAvailabilitySplit `json:"campaign_rollups,omitempty"`
	} `json:"availability"`
	StockGradeDistribution        map[string]int            `json:"stock_grade_distribution"`
	DuplicateCampaignProductLinks int                       `json:"duplicate_campaign_product_links"`
	CampaignProductLinks          CampaignProductLinksAudit `json:"campaign_product_links"`
	ValidationError               string                    `json:"validation_error,omitempty"`
}

// CampaignProductLinksAudit reports link cardinality and duplicate checks.
type CampaignProductLinksAudit struct {
	SourceCount           int `json:"source_count"`
	DistinctByCampaignSKU int `json:"distinct_by_campaign_sku"`
	DuplicateCount        int `json:"duplicate_count"`
}

// AuditDataPack builds an acceptance audit report for a data pack.
func AuditDataPack(pack *SellerAIDataPack) AuditReport {
	report := AuditReport{
		SchemaVersion: pack.SchemaVersion,
		SellerID:      pack.SellerContext.SellerAccountID,
		MetricDate:    pack.SellerContext.MetricDate,
	}
	report.Counts.SKURollups = len(pack.Tables.SKURollups)
	report.Counts.StockPositions = len(pack.Tables.StockPositions)
	report.Counts.ProductEnrichment = len(pack.Tables.ProductEnrichment)
	report.Counts.CampaignRollups = len(pack.Tables.CampaignRollups)
	report.Counts.CampaignProductLinks = len(pack.Tables.CampaignProductLinks)
	report.Counts.FactRegistry = len(pack.FactRegistry)

	raw, _ := json.Marshal(pack)
	report.Shape.FactRegistryType = factRegistryJSONType(raw)
	report.Shape.ConstraintsSource = "top_level_only"
	report.Shape.TablesHasConstraints = tablesJSONHasConstraints(raw)

	if enrich, ok := pack.DataQuality.Sources["product_enrichment"].(map[string]any); ok {
		if src, ok := enrich["source"].(string); ok {
			report.Quality.ProductEnrichmentSource = src
		}
	}
	report.Quality.FinanceStatus = pack.Tables.FinanceRollup.DataStatus
	report.Quality.HasSecretFields = jsonContainsAny(raw, forbiddenSecretSubstrings)
	if err := validateForbiddenBusinessKeysInJSON(raw); err != nil {
		report.Quality.HasBusinessConclusions = true
	}

	report.CampaignScenarios.HighDRRThresholdPct = analytics.AdEfficiencyHighDRRThresholdPct
	report.CampaignScenarios.ElevatedDRRThresholdPct = 50
	for _, c := range pack.Tables.CampaignRollups {
		w := c.Last7d
		if isInactiveCampaignState(c.State) {
			report.CampaignScenarios.InactiveByStateCount++
		}
		if campaignHasNoActivityLast7d(w) {
			report.CampaignScenarios.NoActivityLast7dCount++
		}
		if w.SpendWithoutOrders {
			report.CampaignScenarios.SpendWithoutOrdersCount++
		}
		if w.DRRPct != nil && *w.DRRPct > analytics.AdEfficiencyHighDRRThresholdPct {
			report.CampaignScenarios.HighDRRCount++
		}
		if w.DRRPct != nil && *w.DRRPct > 50 {
			report.CampaignScenarios.ElevatedDRRCount++
		}
		if w.ROAS != nil && *w.ROAS >= 1 && w.Spend > 0 {
			report.CampaignScenarios.HealthyCount++
		}
	}

	report.StockGradeDistribution = map[string]int{}
	for _, row := range pack.Tables.StockPositions {
		if row.TurnoverGrade != "" {
			report.StockGradeDistribution[row.TurnoverGrade]++
		}
	}

	seen := map[string]struct{}{}
	for _, link := range pack.Tables.CampaignProductLinks {
		key := link.CampaignExternalID + ":" + link.SKU
		if _, ok := seen[key]; ok {
			report.DuplicateCampaignProductLinks++
		}
		seen[key] = struct{}{}
	}
	report.CampaignProductLinks = CampaignProductLinksAudit{
		SourceCount:           len(pack.Tables.CampaignProductLinks),
		DistinctByCampaignSKU: len(seen),
		DuplicateCount:        report.DuplicateCampaignProductLinks,
	}

	report.Availability.StockPositions = availabilitySplit(pack.Tables.StockPositions)
	report.Availability.ProductEnrichment = availabilitySplitEnrichment(pack.Tables.ProductEnrichment)
	report.Availability.SKURollups = availabilitySplitRollups(pack.Tables.SKURollups)
	report.Availability.CampaignRollups = availabilitySplitCampaigns(pack.Tables.CampaignRollups)

	if err := ValidateDataPack(pack); err != nil {
		report.ValidationError = err.Error()
	} else {
		report.Valid = true
	}
	return report
}

func factRegistryJSONType(raw []byte) string {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return "unknown"
	}
	fr, ok := root["fact_registry"]
	if !ok || len(fr) == 0 {
		return "missing"
	}
	if fr[0] == '[' {
		return "array"
	}
	if fr[0] == '{' {
		return "object"
	}
	return "unknown"
}

func tablesJSONHasConstraints(raw []byte) bool {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return false
	}
	tables, ok := root["tables"]
	if !ok {
		return false
	}
	var t map[string]json.RawMessage
	if err := json.Unmarshal(tables, &t); err != nil {
		return false
	}
	_, ok = t["constraints"]
	return ok
}

func jsonContainsAny(raw []byte, needles []string) bool {
	lower := strings.ToLower(string(raw))
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

// isInactiveCampaignState reports Ozon campaign states that are not actively serving ads.
func isInactiveCampaignState(state string) bool {
	s := strings.ToUpper(strings.TrimSpace(state))
	switch s {
	case "CAMPAIGN_STATE_INACTIVE", "CAMPAIGN_STATE_STOPPED", "CAMPAIGN_STATE_PAUSED",
		"CAMPAIGN_STATE_ARCHIVED", "CAMPAIGN_STATE_FINISHED":
		return true
	}
	return strings.Contains(s, "INACTIVE") || strings.Contains(s, "STOPPED") || strings.Contains(s, "PAUSED")
}

// campaignHasNoActivityLast7d is true when last_7d has zero spend and zero impressions.
// This is independent of campaign state: a stopped campaign may still have residual impressions in metrics.
func campaignHasNoActivityLast7d(w CampaignMetricsWindow) bool {
	return w.Spend == 0 && w.Impressions == 0
}
