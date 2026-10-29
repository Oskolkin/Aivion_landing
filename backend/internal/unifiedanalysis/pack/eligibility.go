package pack

import (
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// SKUEligibilityReason classifies why a discovered SKU is or is not analysis-eligible.
type SKUEligibilityReason string

const (
	EligibilityOK                 SKUEligibilityReason = "eligible"
	EligibilityInvalidSKU         SKUEligibilityReason = "invalid_sku"
	EligibilityNoProduct          SKUEligibilityReason = "no_product_record"
	EligibilityArchivedOrDeleted  SKUEligibilityReason = "archived_or_deleted"
	EligibilityIncompleteIdentity SKUEligibilityReason = "incomplete_identity"
	EligibilityTechnicalTitle     SKUEligibilityReason = "technical_title"
	EligibilityNoBusinessSource   SKUEligibilityReason = "no_business_source"
	EligibilityHistoricalRelation SKUEligibilityReason = "historical_relation_only"
)

// SKUEligibilityDiagnostics summarizes discovery vs actionable universe.
type SKUEligibilityDiagnostics struct {
	TotalDiscovered          int `json:"total_discovered_sku"`
	AnalysisEligible         int `json:"analysis_eligible_sku"`
	ExcludedTechnical        int `json:"excluded_technical_sku"`
	ExcludedHistorical       int `json:"excluded_historical_sku"`
	ExcludedIncomplete       int `json:"excluded_incomplete_sku"`
	ExcludedArchived         int `json:"excluded_archived_sku"`
	ExcludedNoBusinessSource int `json:"excluded_no_business_source_sku"`
	ExcludedNoProduct        int `json:"excluded_no_product_sku"`
}

// ClassifySKUAnalysisEligibility decides whether a SKU may appear in Analysis Pack
// tables, SKU Focus, Actions, risk drivers, and valid_sku_ids.
//
// Centralized so Pack builders do not re-implement eligibility filters.
func ClassifySKUAnalysisEligibility(idx *Indexes, sku string) (bool, SKUEligibilityReason) {
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return false, EligibilityInvalidSKU
	}
	id, err := strconv.ParseInt(sku, 10, 64)
	if err != nil || id <= 0 {
		return false, EligibilityInvalidSKU
	}

	m, ok := idx.SKUMeta[sku]
	if !ok {
		return false, EligibilityInvalidSKU
	}

	product, hasProduct := idx.ProductBySKU[sku]
	if !hasProduct {
		// Related-SKU / campaign-only discoveries without a seller product record.
		if onlyFromRelationsOrCampaign(idx, sku) {
			return false, EligibilityHistoricalRelation
		}
		return false, EligibilityNoProduct
	}
	if product.IsArchived {
		return false, EligibilityArchivedOrDeleted
	}
	if product.Status.Valid {
		st := strings.ToLower(strings.TrimSpace(product.Status.String))
		if st == "deleted" || st == "archived" || st == "removed" {
			return false, EligibilityArchivedOrDeleted
		}
	}

	offerID := strings.TrimSpace(m.OfferID)
	if offerID == "" && product.OfferID.Valid {
		offerID = strings.TrimSpace(product.OfferID.String)
	}
	title := strings.TrimSpace(m.Title)
	if title == "" {
		title = strings.TrimSpace(product.Name)
	}
	if offerID == "" || title == "" {
		return false, EligibilityIncompleteIdentity
	}
	if isTechnicalSKUTitle(title, sku) {
		return false, EligibilityTechnicalTitle
	}

	if !hasBusinessSource(idx, sku, product) {
		return false, EligibilityNoBusinessSource
	}
	return true, EligibilityOK
}

// IsSKUAnalysisEligible is the boolean form of ClassifySKUAnalysisEligibility.
func IsSKUAnalysisEligible(idx *Indexes, sku string) bool {
	ok, _ := ClassifySKUAnalysisEligibility(idx, sku)
	return ok
}

// BuildActionableSKUUniverse returns the sorted analysis-eligible SKU list and diagnostics.
func BuildActionableSKUUniverse(idx *Indexes, discovered []string) ([]string, SKUEligibilityDiagnostics) {
	diag := SKUEligibilityDiagnostics{TotalDiscovered: len(discovered)}
	out := make([]string, 0, len(discovered))
	for _, sku := range discovered {
		ok, reason := ClassifySKUAnalysisEligibility(idx, sku)
		if ok {
			out = append(out, sku)
			diag.AnalysisEligible++
			continue
		}
		switch reason {
		case EligibilityTechnicalTitle, EligibilityInvalidSKU:
			diag.ExcludedTechnical++
		case EligibilityHistoricalRelation:
			diag.ExcludedHistorical++
		case EligibilityIncompleteIdentity:
			diag.ExcludedIncomplete++
		case EligibilityArchivedOrDeleted:
			diag.ExcludedArchived++
		case EligibilityNoBusinessSource:
			diag.ExcludedNoBusinessSource++
		case EligibilityNoProduct:
			diag.ExcludedNoProduct++
		default:
			diag.ExcludedIncomplete++
		}
	}
	return out, diag
}

func isTechnicalSKUTitle(title, sku string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	if lower == "sku "+strings.ToLower(sku) {
		return true
	}
	if strings.HasPrefix(lower, "sku ") && strings.TrimPrefix(lower, "sku ") == strings.ToLower(sku) {
		return true
	}
	if strings.HasPrefix(lower, "dev-offer-") || strings.HasPrefix(lower, "technical ") {
		return true
	}
	return false
}

func hasBusinessSource(idx *Indexes, sku string, _ dbgen.Product) bool {
	if byDate, ok := idx.SKUMetricsByDate[sku]; ok && len(byDate) > 0 {
		return true
	}
	if agg, ok := idx.StockBySKU[sku]; ok && agg.HasData {
		return true
	}
	if _, ok := idx.ProductInfoBySKU[sku]; ok {
		return true
	}
	if _, ok := idx.campaignLinkedSKUs[sku]; ok {
		return true
	}
	for _, link := range idx.CampaignSKULinks {
		if link.SKU == sku {
			return true
		}
	}
	return false
}

func onlyFromRelationsOrCampaign(idx *Indexes, sku string) bool {
	_, hasProduct := idx.ProductBySKU[sku]
	if hasProduct {
		return false
	}
	_, hasMetrics := idx.SKUMetricsByDate[sku]
	_, hasStock := idx.StockBySKU[sku]
	_, hasInfo := idx.ProductInfoBySKU[sku]
	if hasMetrics || hasStock || hasInfo {
		return false
	}
	_, related := idx.relatedOnlySKUs[sku]
	_, campaign := idx.campaignLinkedSKUs[sku]
	return related || campaign
}
