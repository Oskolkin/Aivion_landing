package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
)

func domainByName(ds []domainEvidence, name string) (domainEvidence, bool) {
	for _, d := range ds {
		if d.domain == name {
			return d, true
		}
	}
	return domainEvidence{}, false
}
func maxMature(m map[string]bool, through time.Time) *time.Time {
	var out *time.Time
	for k, ok := range m {
		if !ok {
			continue
		}
		v, e := time.Parse("2006-01-02", k)
		if e == nil && !v.After(through) && (out == nil || v.After(*out)) {
			x := v
			out = &x
		}
	}
	return out
}
func wm(domain, family string, observed, complete, mature, from, to *time.Time, d domainEvidence, at time.Time) Watermark {
	var run *int64
	sourceContractHash := ""
	semanticMappingHash := d.semanticMappingHash
	if semanticMappingHash == "" && d.mappingVersion != "" {
		semanticMappingHash = factualapi.StableHash("semantic_mapping", d.mappingVersion)
	}
	sourceRevisionHash := d.providerRevision
	if sourceRevisionHash != "" && sourceRevisionHash == semanticMappingHash {
		sourceRevisionHash = "" // legacy aliased mapping hash — not a provider revision
	}
	if d.runID > 0 {
		x := d.runID
		run = &x
		sourceContractHash = factualapi.DeriveSourceContractHash(d.contractKey, d.contractVersion, d.endpointFamily)
	}
	w := Watermark{
		Domain: domain, FactFamily: family, LatestObservedAt: observed, LatestCompleteBusinessDate: complete,
		LatestMatureBusinessDate: mature, HistoryFrom: from, HistoryTo: to, DomainRunID: run,
		SourceContractHash: sourceContractHash, SemanticMappingHash: semanticMappingHash, SourceRevisionHash: sourceRevisionHash,
		AssessedAt: at,
	}
	w.StateHash = factualapi.StableHash(domain, family, tstr(observed), tstr(complete), tstr(mature), tstr(from), tstr(to), fmt.Sprint(d.runID), sourceContractHash)
	return w
}
func snapshotWatermark(w Watermark, snapshot *time.Time) Watermark {
	w.LatestCompleteSnapshotAt = snapshot
	w.StateHash = factualapi.StableHash(w.StateHash, tstr(snapshot))
	return w
}
func tstr(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339Nano)
}
func (s *Service) watermarks(ds []domainEvidence, st evidenceStats, at time.Time) []Watermark {
	cat, _ := domainByName(ds, "CATALOG")
	sales, _ := domainByName(ds, "SALES_DEMAND")
	post, _ := domainByName(ds, "POSTINGS")
	ret, _ := domainByName(ds, "RETURNS")
	stock, _ := domainByName(ds, "STOCK")
	price, _ := domainByName(ds, "PRICE")
	ads, _ := domainByName(ds, "ADVERTISING")
	funnel, _ := domainByName(ds, "FUNNEL")
	finance, _ := domainByName(ds, "FINANCE")
	search, _ := domainByName(ds, "SEARCH")
	promotions, _ := domainByName(ds, "PROMOTIONS")
	health, _ := domainByName(ds, "ACCOUNT_HEALTH")
	supply, _ := domainByName(ds, "SUPPLY")
	mature := maxMature(st.matureDates, lastCompletedMoscow(at))
	return []Watermark{
		wm("CATALOG", "CATALOG.SKU_UNIVERSE", timePtr(cat.finalized), nil, nil, nil, nil, cat, at),
		wm("SALES_DEMAND", "SALES.ACCOUNT_DEMAND", st.latestSalesObserved, st.salesCompleteThrough, mature, st.salesFrom, st.salesTo, sales, at),
		wm("SALES_DEMAND", "SALES.SKU_DEMAND", st.latestSalesObserved, st.salesCompleteThrough, mature, st.salesFrom, st.salesTo, sales, at),
		wm("POSTINGS", "LIFECYCLE.POSTINGS", st.latestPostingObserved, nil, nil, nil, nil, post, at),
		wm("POSTINGS", "LIFECYCLE.EVENTS", st.latestPostingObserved, nil, nil, nil, nil, post, at),
		wm("RETURNS", "LIFECYCLE.RETURNS", st.latestReturnObserved, nil, nil, nil, nil, ret, at),
		snapshotWatermark(wm("STOCK", "STOCK.SNAPSHOT", st.latestStockObserved, nil, nil, st.latestStockObserved, st.latestStockObserved, stock, at), st.latestStockComplete),
		snapshotWatermark(wm("PRICE", "PRICE.SNAPSHOT", st.latestPriceObserved, nil, nil, st.latestPriceObserved, st.latestPriceObserved, price, at), st.latestPriceComplete),
		wm("ADVERTISING", "ADVERTISING.TRAFFIC", st.latestAdvertisingObserved, st.adTrafficCompleteThrough, nil, st.adHistoryFrom, st.adHistoryTo, ads, at),
		wm("ADVERTISING", "ADVERTISING.ATTRIBUTION", st.latestAdvertisingObserved, st.adTrafficCompleteThrough, st.adMatureThrough, st.adHistoryFrom, st.adHistoryTo, ads, at),
		wm("FUNNEL", "FUNNEL.DAILY_ADDITIVE", st.funnelLatestObserved, st.funnelAdditiveTo, nil, st.funnelAdditiveFrom, st.funnelAdditiveTo, funnel, at),
		wm("FUNNEL", "FUNNEL.DIRECT_PERIOD", st.funnelLatestObserved, st.funnelDirectTo, nil, st.funnelDirectFrom, st.funnelDirectTo, funnel, at),
		wm("FINANCE", "FINANCE.DAILY_LEDGER", st.financeLatestObserved, st.financeCompleteThrough, nil, st.financeHistoryFrom, st.financeHistoryTo, finance, at),
		snapshotWatermark(wm("FINANCE", "FINANCE.BALANCE", st.financeBalanceLatestObserved, nil, nil, st.financeBalanceLatestObserved, st.financeBalanceLatestObserved, finance, at), st.financeBalanceLatestObserved),
		wm("SEARCH", "SEARCH.SELLER_SUMMARY", st.searchSummaryObserved, st.searchSummaryTo, nil, st.searchSummaryFrom, st.searchSummaryTo, search, at),
		wm("SEARCH", "SEARCH.QUERY_DETAILS", st.searchDetailObserved, st.searchDetailTo, nil, st.searchDetailFrom, st.searchDetailTo, search, at),
		wm("SEARCH", "SEARCH.MARKET_SEARCH", st.searchMarketObserved, st.searchMarketTo, nil, st.searchMarketFrom, st.searchMarketTo, search, at),
		wm("PROMOTIONS", "PROMOTIONS.DEFINITIONS", timePtr(promotions.finalized), nil, nil, nil, nil, promotions, at),
		wm("PROMOTIONS", "PROMOTIONS.MEMBERSHIP", timePtr(promotions.finalized), nil, nil, nil, nil, promotions, at),
		snapshotWatermark(wm("PROMOTIONS", "PROMOTIONS.ELIGIBILITY", timePtr(promotions.finalized), nil, nil, nil, nil, promotions, at), timePtr(promotions.finalized)),
		snapshotWatermark(wm("ACCOUNT_HEALTH", "CURRENT_SNAPSHOT", st.healthSnapshotObserved, nil, nil, nil, nil, health, at), st.healthSnapshotObserved),
		wm("ACCOUNT_HEALTH", "RATING_HISTORY", st.healthHistoryObserved, nil, nil, st.healthHistoryFrom, st.healthHistoryTo, health, at),
		snapshotWatermark(wm("ACCOUNT_HEALTH", "FBS_INDEX", st.healthFBSIndexObserved, nil, nil, nil, nil, health, at), st.healthFBSIndexObserved),
		wm("ACCOUNT_HEALTH", "FBS_ERROR_EVENTS", st.healthFBSErrorObserved, nil, nil, nil, nil, health, at),
		wm("SUPPLY", "SUPPLY.STATE", st.supplyStateObserved, nil, nil, nil, nil, supply, at),
		snapshotWatermark(wm("SUPPLY", "SUPPLY.BUNDLE", st.supplyBundleObserved, nil, nil, nil, nil, supply, at), st.supplyBundleCompleteObserved),
		snapshotWatermark(wm("SUPPLY", "SUPPLY.ACCEPTANCE", st.supplyAcceptanceObserved, nil, nil, nil, nil, supply, at), st.supplyAcceptanceCompleteObserved)}
}
func timePtr(v time.Time) *time.Time {
	if v.IsZero() {
		return nil
	}
	x := v
	return &x
}
func (s *Service) issues(ds []domainEvidence, st evidenceStats, at time.Time) []DQIssue {
	var out []DQIssue
	for _, d := range []string{"CATALOG", "SALES_DEMAND", "POSTINGS", "RETURNS"} {
		if _, ok := domainByName(ds, d); !ok {
			out = append(out, DQIssue{Domain: d, Dimension: "COMPLETENESS", Code: "AUTHORITATIVE_DOMAIN_UNAVAILABLE", Severity: "ERROR", ReadinessEffect: "FAIL_CAPABILITY", RetryClass: "DATA_STATE"})
		}
	}
	if st.skuCount > 0 && st.mappedCount < st.skuCount {
		out = append(out, DQIssue{Domain: "CATALOG", Dimension: "RESOLUTION", Code: "CATEGORY_MAPPING_PARTIAL", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"total_skus": st.skuCount, "mapped_skus": st.mappedCount, "unmapped_skus": st.skuCount - st.mappedCount}})
	}
	if st.productTypeResolutionIssues > 0 {
		out = append(out, DQIssue{Domain: "CATALOG", Dimension: "RESOLUTION", Code: "PRODUCT_TYPE_RESOLUTION_PARTIAL", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"affected_skus": st.productTypeResolutionIssues}})
	}
	for domain, code := range st.currentDomainError {
		if domain != "CATALOG" && domain != "SALES_DEMAND" && domain != "POSTINGS" && domain != "RETURNS" {
			continue
		}
		switch code {
		case "SOURCE_CONTRACT_MISMATCH":
			out = append(out, DQIssue{Domain: domain, Dimension: "SEMANTIC", Code: code, Severity: "ERROR", ReadinessEffect: "FAIL_DOMAIN", RetryClass: "STRUCTURAL"})
		case "ORDER_IDENTITY_CONFLICT", "POSTING_SCHEME_CONFLICT", "POSTING_LINEAGE_CYCLE":
			out = append(out, DQIssue{Domain: domain, Dimension: "INTEGRITY", Code: code, Severity: "CRITICAL", ReadinessEffect: "FAIL_DOMAIN", RetryClass: "STRUCTURAL"})
		}
	}
	for day, status := range st.reconciliation {
		switch status {
		case "EXPLAINED_GAP":
			d, _ := time.Parse("2006-01-02", day)
			out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "RECONCILIATION", Code: "SALES_RECONCILIATION_EXPLAINED_GAP", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", PeriodStart: &d, PeriodEnd: &d})
		case "UNEXPLAINED_GAP", "NOT_COMPARABLE":
			d, _ := time.Parse("2006-01-02", day)
			out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "RECONCILIATION", Code: "SALES_RECONCILIATION_" + status, Severity: "ERROR", ReadinessEffect: "BLOCK_ANALYSIS_DATE", RetryClass: "DATA_STATE", PeriodStart: &d, PeriodEnd: &d})
		}
	}
	if st.unknownEvents > 0 {
		out = append(out, DQIssue{Domain: "POSTINGS", Dimension: "TEMPORAL", Code: "LIFECYCLE_TIME_UNKNOWN", Severity: "INFO", ReadinessEffect: "LIMITATION_ONLY", RetryClass: "DATA_STATE", Details: map[string]any{"unknown_events": st.unknownEvents}})
	}
	if st.missingSalesDays > 0 {
		out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "COMPLETENESS", Code: "SALES_DAILY_COVERAGE_HOLE", Severity: "ERROR", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"missing_days": st.missingSalesDays}})
	}
	if st.salesDays > 0 && st.salesDays < 14 {
		out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "FRESHNESS_MATURITY", Code: "SALES_HISTORY_INSUFFICIENT", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"history_days": st.salesDays}})
	}
	if len(st.matureDates) < 14 {
		out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "FRESHNESS_MATURITY", Code: "SALES_MATURITY_INSUFFICIENT", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"mature_days": len(st.matureDates)}})
	}
	if st.mixedCurrencyDays > 0 {
		out = append(out, DQIssue{Domain: "SALES_DEMAND", Dimension: "VALIDITY", Code: "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE", Severity: "ERROR", ReadinessEffect: "FAIL_CAPABILITY", RetryClass: "DATA_STATE", Details: map[string]any{"affected_days": st.mixedCurrencyDays}})
	}
	if st.postingOrderResolutionIssues+st.postingSKUResolutionIssues > 0 {
		out = append(out, DQIssue{Domain: "POSTINGS", Dimension: "RESOLUTION", Code: "LIFECYCLE_LINK_UNRESOLVED", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"unresolved_order_links": st.postingOrderResolutionIssues, "unresolved_sku_sets": st.postingSKUResolutionIssues}})
	}
	if st.missingPostingSKUs > 0 {
		out = append(out, DQIssue{Domain: "POSTINGS", Dimension: "RESOLUTION", Code: "SOURCE_SKU_MISSING", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"product_lines": st.missingPostingSKUs}})
	}
	if st.returnResolutionIssues > 0 {
		out = append(out, DQIssue{Domain: "RETURNS", Dimension: "RESOLUTION", Code: "LIFECYCLE_LINK_UNRESOLVED", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"affected_returns": st.returnResolutionIssues}})
	}
	if st.stockFailedBatches > 0 {
		out = append(out, DQIssue{Domain: "STOCK", Dimension: "COMPLETENESS", Code: "SOURCE_BATCH_UNAVAILABLE", Severity: "ERROR", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "TRANSIENT", Details: map[string]any{"failed_batches": st.stockFailedBatches}})
	}
	if st.stockMissingResponse > 0 {
		out = append(out, DQIssue{Domain: "STOCK", Dimension: "COMPLETENESS", Code: "MISSING_PROVIDER_RESPONSE_ROW", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"missing_response_skus": st.stockMissingResponse}})
	}
	if st.stockUnresolved > 0 {
		out = append(out, DQIssue{Domain: "STOCK", Dimension: "RESOLUTION", Code: "STOCK_SKU_RESOLUTION_PARTIAL", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"unresolved_skus": st.stockUnresolved}})
	}
	if st.stockAmbiguousPools > 0 {
		out = append(out, DQIssue{Domain: "STOCK", Dimension: "SEMANTIC", Code: "POOL_SEMANTICS_PENDING", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "PENDING_ACCEPTANCE", Details: map[string]any{"positions": st.stockAmbiguousPools}})
	}
	if st.stockPendingSemantics > 0 {
		out = append(out, DQIssue{Domain: "STOCK", Dimension: "SEMANTIC", Code: "STOCK_QUANTITY_MAPPING_PENDING_ACCEPTANCE", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "PENDING_ACCEPTANCE", Details: map[string]any{"positions": st.stockPendingSemantics}})
	}
	if st.priceUnresolved > 0 {
		out = append(out, DQIssue{Domain: "PRICE", Dimension: "RESOLUTION", Code: "PRICE_SKU_RESOLUTION_PARTIAL", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"unresolved_skus": st.priceUnresolved}})
	}
	if st.priceInvalid > 0 {
		out = append(out, DQIssue{Domain: "PRICE", Dimension: "VALIDITY", Code: "PRICE_CURRENT_VALUE_INVALID", Severity: "ERROR", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"invalid_prices": st.priceInvalid}})
	}
	if st.pricePendingSemantics > 0 {
		out = append(out, DQIssue{Domain: "PRICE", Dimension: "SEMANTIC", Code: "PRICE_MAPPING_PENDING_ACCEPTANCE", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "PENDING_ACCEPTANCE"})
	}
	if st.priceMixedCurrencies > 0 {
		out = append(out, DQIssue{Domain: "PRICE", Dimension: "VALIDITY", Code: "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE", Severity: "ERROR", ReadinessEffect: "FAIL_CAPABILITY", RetryClass: "DATA_STATE"})
	}
	if st.adUnknownFamilies > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "SEMANTIC", Code: "AD_UNKNOWN_PROVIDER_ENUM", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "CONTRACT_CHANGE", Details: map[string]any{"campaigns": st.adUnknownFamilies}})
	}
	if st.adUnknownTargetScopes > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "SEMANTIC", Code: "TARGET_SCOPE_UNKNOWN", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "PENDING_ACCEPTANCE", Details: map[string]any{"campaigns": st.adUnknownTargetScopes}})
	}
	if st.adUnresolvedTargets > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "RESOLUTION", Code: "SKU_IDENTITY_UNRESOLVED", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"targets": st.adUnresolvedTargets}})
	}
	if st.adPendingAttribution > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "FRESHNESS_MATURITY", Code: "AD_ATTRIBUTION_PENDING_ACCEPTANCE", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "PENDING_ACCEPTANCE", Details: map[string]any{"facts": st.adPendingAttribution}})
	}
	if st.adReconciliationIssues > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "RECONCILIATION", Code: "SPEND_RECONCILIATION_DIFFERENT", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE"})
	}
	if st.adMixedCurrencies > 0 {
		out = append(out, DQIssue{Domain: "ADVERTISING", Dimension: "VALIDITY", Code: "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE", Severity: "ERROR", ReadinessEffect: "FAIL_CAPABILITY", RetryClass: "DATA_STATE"})
	}
	if st.financePartialDays > 0 {
		out = append(out, DQIssue{Domain: "FINANCE", Dimension: "COMPLETENESS", Code: "FINANCE_DAILY_LEDGER_PARTIAL", Severity: "ERROR", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"partial_days": st.financePartialDays}})
	}
	if st.financeUnmappedTypes > 0 {
		out = append(out, DQIssue{Domain: "FINANCE", Dimension: "SEMANTIC", Code: "FINANCE_PROVIDER_TYPE_UNMAPPED", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "CONTRACT_CHANGE", Details: map[string]any{"provider_types": st.financeUnmappedTypes}})
	}
	if st.financeMixedCurrencies > 0 {
		out = append(out, DQIssue{Domain: "FINANCE", Dimension: "VALIDITY", Code: "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE", Severity: "ERROR", ReadinessEffect: "FAIL_CAPABILITY", RetryClass: "DATA_STATE"})
	}
	if st.healthUnmappedMetrics > 0 {
		out = append(out, DQIssue{Domain: "ACCOUNT_HEALTH", Dimension: "SEMANTIC", Code: "UNMAPPED_RATING_KEY", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "CONTRACT_CHANGE", Details: map[string]any{"metrics": st.healthUnmappedMetrics}})
	}
	if st.healthUnresolvedPostings > 0 {
		out = append(out, DQIssue{Domain: "ACCOUNT_HEALTH", Dimension: "RESOLUTION", Code: "HEALTH_POSTING_UNRESOLVED", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"events": st.healthUnresolvedPostings}})
	}
	if st.supplyPartialBundles > 0 {
		out = append(out, DQIssue{Domain: "SUPPLY", Dimension: "COMPLETENESS", Code: "SUPPLY_BUNDLE_PARTIAL", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "DATA_STATE", Details: map[string]any{"partial_bundles": st.supplyPartialBundles}})
	}
	if st.supplyUnknownStates > 0 {
		out = append(out, DQIssue{Domain: "SUPPLY", Dimension: "SEMANTIC", Code: "SUPPLY_STATE_UNKNOWN", Severity: "WARNING", ReadinessEffect: "DEGRADE_TO_PARTIAL", RetryClass: "CONTRACT_CHANGE", Details: map[string]any{"observations": st.supplyUnknownStates}})
	}
	return out
}

func sourceState(d domainEvidence) string {
	if d.runID == 0 {
		return "NOT_ATTEMPTED"
	}
	if d.status == "PARTIAL" || d.completeness == "PARTIAL" {
		return "PARTIAL"
	}
	return "SUCCESS"
}
func finalCapability(contract, access, source, semantic, coverage, history string) string {
	if contract == "REMOVED" || semantic == "INVALID" || semantic == "INCOMPATIBLE" || source == "FAILED" {
		return "FAILED"
	}
	if access != "ACCESSIBLE" || source == "NOT_ATTEMPTED" {
		return "UNAVAILABLE"
	}
	if semantic != "VALIDATED" || coverage != "FULL" || history == "PARTIAL" || source == "PARTIAL" {
		return "PARTIAL"
	}
	return "READY"
}
func (s *Service) capabilities(ds []domainEvidence, st evidenceStats, at time.Time) []Capability {
	var out []Capability
	for _, key := range CapabilityKeys() {
		assessmentKey := key
		if family := capabilityAssessmentFamily[key]; family != "" {
			assessmentKey = family
		}
		domain := capabilityDomain[key]
		d, _ := domainByName(ds, domain)
		for _, scope := range []string{"CURRENT_COLLECTION", "HISTORICAL_QUERY"} {
			source := sourceState(d)
			contract := "ACTIVE"
			temporal := "CURRENT"
			if domain == "CATALOG" && d.runID > 0 && st.skuCount == 0 {
				source = "EMPTY_VALID"
			}
			if scope == "CURRENT_COLLECTION" {
				switch st.currentDomainState[domain] {
				case "FAILED", "CANCELLED":
					source = "FAILED"
				case "RUNNING":
					source = "PARTIAL"
				}
			}
			semantic := s.semantics.State(assessmentKey)
			coverage := "FULL"
			history := "AVAILABLE"
			population := "NON_EMPTY"
			reason := ""
			if d.runID == 0 {
				contract = "UNKNOWN"
				temporal = "UNKNOWN"
				history = "NONE"
			}
			if domain == "CATALOG" {
				if st.skuCount == 0 {
					population = "EMPTY"
				}
				if key == "CATALOG.CATEGORY_ATTRIBUTION" && st.mappedCount < st.skuCount {
					coverage = "PARTIAL"
					reason = "CATEGORY_MAPPING_PARTIAL"
				}
			}
			if domain == "SALES_DEMAND" {
				if st.skuCount == 0 {
					access := "ACCESSIBLE"
					if d.runID == 0 {
						access = "UNKNOWN"
					}
					out = append(out, Capability{Key: key, Scope: scope, Domain: domain, FinalState: "NOT_APPLICABLE", ContractState: contract, AccessState: access, SourceState: source, SemanticState: semantic, CoverageState: "UNKNOWN", TemporalState: temporal, HistoryState: "NONE", PopulationState: "EMPTY", ReasonCode: "EMPTY_SKU_UNIVERSE", AssessedAt: at, DomainRunID: runPtr(d)})
					continue
				}
				if st.salesDays < 14 {
					history = "INSUFFICIENT"
					coverage = "PARTIAL"
					reason = "INSUFFICIENT_HISTORY"
				}
				if st.explainedSalesDays > 0 {
					coverage = "PARTIAL"
					reason = "SALES_RECONCILIATION_EXPLAINED_GAP"
				}
			}
			postingLimited := key == "LIFECYCLE.POSTINGS" && (st.postingOrderResolutionIssues+st.postingSKUResolutionIssues > 0)
			orderLimited := key == "LIFECYCLE.ORDERS" && st.postingOrderResolutionIssues > 0
			eventLimited := key == "LIFECYCLE.EVENTS" && (st.unknownEvents > 0 || st.postingOrderResolutionIssues > 0)
			if domain == "POSTINGS" && (postingLimited || orderLimited || eventLimited) {
				coverage = "PARTIAL"
				if postingLimited || orderLimited || st.postingOrderResolutionIssues > 0 {
					reason = "LIFECYCLE_LINK_UNRESOLVED"
				} else {
					reason = "LIFECYCLE_TIME_UNKNOWN"
				}
			}
			if domain == "RETURNS" && st.returnResolutionIssues > 0 {
				coverage = "PARTIAL"
				reason = "LIFECYCLE_LINK_UNRESOLVED"
			}
			if domain == "STOCK" {
				population = "NON_EMPTY"
				if st.stockSnapshotCount == 0 {
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "NO_STOCK_SNAPSHOT"
				}
				if st.stockFailedBatches+st.stockUnresolved+st.stockAmbiguousPools+st.stockPendingSemantics > 0 {
					coverage = "PARTIAL"
					if reason == "" {
						reason = "STOCK_SEMANTICS_OR_COMPLETENESS_PARTIAL"
					}
				}
				if (key == "STOCK.COVERAGE_7D" || key == "STOCK.COVERAGE_30D") && st.stockPendingSemantics > 0 {
					coverage = "PARTIAL"
					reason = "SELLABLE_MAPPING_PENDING_ACCEPTANCE"
				}
			}
			if domain == "PRICE" {
				if st.priceSnapshotCount == 0 {
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "NO_PRICE_SNAPSHOT"
				}
				if st.priceUnresolved+st.priceInvalid+st.pricePendingSemantics+st.priceMixedCurrencies > 0 {
					coverage = "PARTIAL"
					if reason == "" {
						reason = "PRICE_SEMANTICS_OR_COMPLETENESS_PARTIAL"
					}
				}
				if key == "PRICE.CUSTOMER_PRICE" {
					coverage = "PARTIAL"
					reason = "PREMIUM_ENRICHMENT_OPTIONAL_OR_UNAVAILABLE"
				}
			}
			if domain == "ADVERTISING" {
				if st.adCampaignCount == 0 {
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "NO_CAMPAIGNS_OBSERVED"
				}
				// Source-family aware: do not degrade independent capabilities from unrelated issues.
				switch key {
				case "ADVERTISING.CAMPAIGN_UNIVERSE", "ADVERTISING.CAMPAIGN_CONFIG":
					if st.adUnknownFamilies > 0 {
						coverage = "PARTIAL"
						reason = "AD_UNKNOWN_PROVIDER_ENUM"
					}
				case "ADVERTISING.TARGET_SCOPE":
					if st.adUnknownTargetScopes > 0 {
						coverage = "PARTIAL"
						reason = "TARGET_SCOPE_UNKNOWN"
					}
					if st.adUnresolvedTargets > 0 {
						coverage = "PARTIAL"
						reason = "SKU_IDENTITY_UNRESOLVED"
					}
				case "ADVERTISING.CAMPAIGN_TRAFFIC", "ADVERTISING.ACCOUNT_SPEND", "ADVERTISING.ACCOUNT_SPEND_SHARE":
					if st.adMixedCurrencies > 0 {
						coverage = "PARTIAL"
						reason = "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE"
					}
					if key != "ADVERTISING.CAMPAIGN_TRAFFIC" && st.adReconciliationIssues > 0 {
						coverage = "PARTIAL"
						reason = "SPEND_RECONCILIATION_DIFFERENT"
					}
				case "ADVERTISING.SKU_TRAFFIC":
					coverage = "PARTIAL"
					reason = "DIRECT_SKU_FACT_COVERAGE_SOURCE_DEPENDENT"
				case "ADVERTISING.CAMPAIGN_ATTRIBUTION", "ADVERTISING.SKU_ATTRIBUTION", "ADVERTISING.CAMPAIGN_DRR":
					if st.adPendingAttribution > 0 {
						coverage = "PARTIAL"
						reason = "AD_ATTRIBUTION_PENDING_ACCEPTANCE"
					}
				case "ADVERTISING.ACCOUNT_ATTRIBUTION_DRR":
					coverage = "PARTIAL"
					reason = "ATTRIBUTION_AGGREGATION_PENDING_ACCEPTANCE"
				}
				// CPO soft-fail must not force campaign universe/traffic to PARTIAL via shared domain source alone.
				if key == "ADVERTISING.CAMPAIGN_UNIVERSE" || key == "ADVERTISING.CAMPAIGN_CONFIG" || key == "ADVERTISING.CAMPAIGN_TRAFFIC" {
					if source == "PARTIAL" && st.adCampaignCount > 0 && st.adDailyCount > 0 {
						source = "SUCCESS"
					}
				}
			}
			if domain == "FUNNEL" {
				count := st.funnelAdditiveCount
				if assessmentKey == "FUNNEL.DIRECT_PERIOD" {
					count = st.funnelDirectCount
				}
				if count == 0 {
					source = "NOT_ATTEMPTED"
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "NO_FUNNEL_METRICS"
				}
			}
			if domain == "FINANCE" {
				if assessmentKey == "FINANCE.BALANCE" {
					if st.financeBalanceCount == 0 {
						source = "NOT_ATTEMPTED"
						population = "EMPTY"
						history = "NONE"
						coverage = "UNKNOWN"
						reason = "NO_FINANCE_BALANCE_SNAPSHOT"
					}
				} else if st.financeDayCount == 0 {
					source = "NOT_ATTEMPTED"
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "NO_FINANCE_DAILY_LEDGER"
				} else {
					if st.financePartialDays > 0 {
						coverage = "PARTIAL"
						reason = "FINANCE_DAILY_LEDGER_PARTIAL"
					}
					if (assessmentKey == "FINANCE.ACCOUNT_NET" || assessmentKey == "FINANCE.DAILY_LEDGER") && st.financeMixedCurrencies > 0 {
						coverage = "PARTIAL"
						reason = "MULTIPLE_CURRENCIES_NOT_AGGREGATABLE"
					}
					if (assessmentKey == "FINANCE.ACCRUAL_BREAKDOWN" || assessmentKey == "FINANCE.TYPE_DICTIONARY") && st.financeUnmappedTypes > 0 {
						coverage = "PARTIAL"
						reason = "FINANCE_PROVIDER_TYPE_UNMAPPED"
					}
				}
			}
			if domain == "SEARCH" {
				count := st.searchSummaryCount
				switch assessmentKey {
				case "SEARCH.QUERY_DETAILS":
					count = st.searchDetailCount
					if st.searchUnresolvedDetails > 0 {
						coverage = "PARTIAL"
						reason = "SKU_IDENTITY_UNRESOLVED"
					}
				case "SEARCH.CATEGORY_SUMMARY":
					count = st.searchDetailCount
					coverage = "PARTIAL"
					reason = "DETAIL_GMV_NOT_SELLER_SEARCH_TOTAL"
				case "SEARCH.MARKET_SEARCH":
					count = st.searchMarketCount
				}
				if count == 0 {
					source = "NOT_ATTEMPTED"
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					if assessmentKey == "SEARCH.MARKET_SEARCH" {
						reason = "PREMIUM_MARKET_SEARCH_UNAVAILABLE"
					} else {
						reason = "SEARCH_FACT_FAMILY_UNAVAILABLE"
					}
				} else if assessmentKey == "SEARCH.SELLER_SUMMARY" && source == "PARTIAL" {
					// Optional details and Premium Market Search cannot degrade
					// the independently successful seller-summary family.
					source = "SUCCESS"
				}
				var latest *time.Time
				switch assessmentKey {
				case "SEARCH.SELLER_SUMMARY":
					latest = st.searchSummaryTo
				case "SEARCH.QUERY_DETAILS", "SEARCH.CATEGORY_SUMMARY":
					latest = st.searchDetailTo
				case "SEARCH.MARKET_SEARCH":
					latest = st.searchMarketTo
				}
				if latest != nil && latest.Before(lastCompletedMoscow(at)) {
					temporal = "LAGGED"
					if coverage == "FULL" {
						coverage = "PARTIAL"
						reason = "EXPECTED_PROVIDER_LAG"
					}
				}
			}
			if domain == "PROMOTIONS" && d.runID == 0 {
				population = "EMPTY"
				history = "NONE"
				coverage = "UNKNOWN"
				reason = "NO_PROMOTIONS_COLLECTION"
			}
			if domain == "ACCOUNT_HEALTH" {
				count := st.healthSnapshotCount
				switch key {
				case "HEALTH.RATING_HISTORY":
					count = st.healthHistoryCount
				case "HEALTH.THRESHOLDS":
					count = st.healthThresholdCount
				case "HEALTH.PENALTY_EVENTS":
					count = st.healthPenaltyCount
				case "HEALTH.FBS_ERROR_INDEX":
					count = st.healthFBSIndexCount
				case "HEALTH.FBS_POSTING_ERRORS":
					count = st.healthFBSErrorCount
					if st.healthUnresolvedPostings > 0 {
						coverage = "PARTIAL"
						reason = "HEALTH_POSTING_UNRESOLVED"
					}
				}
				if count == 0 {
					source = "NOT_ATTEMPTED"
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					reason = "HEALTH_FACT_FAMILY_UNAVAILABLE"
				} else {
					if source == "PARTIAL" {
						source = "SUCCESS"
					}
					if (key == "HEALTH.CURRENT_RATINGS" || key == "HEALTH.RATING_HISTORY" || key == "HEALTH.THRESHOLDS") && st.healthUnmappedMetrics > 0 {
						coverage = "PARTIAL"
						reason = "UNMAPPED_RATING_KEY"
					}
				}
			}
			if domain == "SUPPLY" {
				count := st.supplyStateCount
				if assessmentKey == "SUPPLY.BUNDLE" || assessmentKey == "SUPPLY.SKU_INBOUND" {
					count = st.supplyBundleCount
				}
				switch assessmentKey {
				case "SUPPLY.ACCEPTANCE", "SUPPLY.ACCEPTANCE_SUMMARY":
					count = st.supplyAcceptanceCount
				case "SUPPLY.ACCEPTANCE_PRODUCTS":
					count = st.supplyAcceptanceItemCount
				}
				if count == 0 {
					source = "NOT_ATTEMPTED"
					population = "EMPTY"
					history = "NONE"
					coverage = "UNKNOWN"
					if assessmentKey == "SUPPLY.ACCEPTANCE" || assessmentKey == "SUPPLY.ACCEPTANCE_SUMMARY" || assessmentKey == "SUPPLY.ACCEPTANCE_PRODUCTS" {
						reason = "BETA_ACCEPTANCE_UNAVAILABLE"
					} else {
						reason = "NO_SUPPLY_FACTS"
					}
				}
				if (assessmentKey == "SUPPLY.BUNDLE" || assessmentKey == "SUPPLY.SKU_INBOUND") && st.supplyPartialBundles > 0 {
					coverage = "PARTIAL"
					reason = "SUPPLY_BUNDLE_PARTIAL"
				}
				if assessmentKey == "SUPPLY.STATE" && st.supplyUnknownStates > 0 {
					coverage = "PARTIAL"
					reason = "SUPPLY_STATE_UNKNOWN"
				}
				if (assessmentKey == "SUPPLY.ACCEPTANCE" || assessmentKey == "SUPPLY.ACCEPTANCE_SUMMARY" || assessmentKey == "SUPPLY.ACCEPTANCE_PRODUCTS") && st.supplyAcceptancePartial > 0 {
					coverage = "PARTIAL"
					reason = "SUPPLY_ACCEPTANCE_PARTIAL"
				}
			}
			access := "ACCESSIBLE"
			if scope == "CURRENT_COLLECTION" {
				access = st.currentDomainAccess[domain]
				if access == "" {
					access = st.connectionAccess
					if domain == "STOCK" || domain == "PRICE" || domain == "ADVERTISING" || domain == "FUNNEL" || domain == "FINANCE" || domain == "SEARCH" || domain == "PROMOTIONS" || domain == "ACCOUNT_HEALTH" || domain == "SUPPLY" {
						access = st.canonicalOptionalAccess
					}
				}
			} else if d.runID == 0 {
				access = "UNKNOWN"
			}
			final := finalCapability(contract, access, source, semantic, coverage, history)
			if semantic == "PENDING_ACCEPTANCE" && reason == "" {
				reason = "SOURCE_SEMANTICS_PENDING_ACCEPTANCE"
			}
			details := map[string]any{"expected_entities": st.skuCount, "observed_entities": st.skuCount, "resolved_entities": st.mappedCount, "history_days": st.salesDays, "sales_exposure_total": st.categoryExposureTotal, "sales_exposure_mapped": st.categoryExposureMapped}
			if st.skuCount > 0 {
				details["resolution_ratio"] = fmt.Sprintf("%d/%d", st.mappedCount, st.skuCount)
			}
			if st.categoryExposureTotal != "0" {
				details["sales_exposure_ratio"] = st.categoryExposureMapped + "/" + st.categoryExposureTotal
			}
			out = append(out, Capability{Key: key, Scope: scope, Domain: domain, FinalState: final, ContractState: contract, AccessState: access, SourceState: source, SemanticState: semantic, CoverageState: coverage, TemporalState: temporal, HistoryState: history, PopulationState: population, ReasonCode: reason, Details: details, AssessedAt: at, DomainRunID: runPtr(d)})
		}
	}
	return out
}
func runPtr(d domainEvidence) *int64 {
	if d.runID <= 0 {
		return nil
	}
	v := d.runID
	return &v
}

// shouldAppendWatermarkRevision implements A→A / A→B / A→B→A semantics:
// append when there is no prior revision or the semantic state_hash differs
// from the latest revision (including returning to a prior hash).
func shouldAppendWatermarkRevision(latestRevHash string, hasLatest bool, newHash string) bool {
	if !hasLatest {
		return true
	}
	return latestRevHash != newHash
}

func (s *Service) persistWatermark(ctx context.Context, seller int64, w *Watermark) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)

	var latestRevHash string
	e = tx.QueryRow(ctx, `SELECT state_hash FROM domain_watermark_revisions
		WHERE seller_account_id=$1 AND domain=$2 AND fact_family=$3
		ORDER BY knowledge_time DESC, id DESC LIMIT 1`, seller, w.Domain, w.FactFamily).Scan(&latestRevHash)
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	appendRevision := shouldAppendWatermarkRevision(latestRevHash, e == nil, w.StateHash)
	// A→A (identical semantic state_hash as latest revision): update projection only.

	_, e = tx.Exec(ctx, `INSERT INTO domain_watermarks(seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,semantic_mapping_hash,source_revision_hash,state_hash,assessed_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),$14,$15)ON CONFLICT(seller_account_id,domain,fact_family)DO UPDATE SET latest_observed_at=EXCLUDED.latest_observed_at,latest_complete_business_date=EXCLUDED.latest_complete_business_date,latest_mature_business_date=EXCLUDED.latest_mature_business_date,latest_complete_snapshot_at=EXCLUDED.latest_complete_snapshot_at,history_available_from=EXCLUDED.history_available_from,history_available_to=EXCLUDED.history_available_to,last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id,source_contract_hash=EXCLUDED.source_contract_hash,semantic_mapping_hash=EXCLUDED.semantic_mapping_hash,source_revision_hash=EXCLUDED.source_revision_hash,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at`, seller, w.Domain, w.FactFamily, w.LatestObservedAt, w.LatestCompleteBusinessDate, w.LatestMatureBusinessDate, w.LatestCompleteSnapshotAt, w.HistoryFrom, w.HistoryTo, w.DomainRunID, w.SourceContractHash, w.SemanticMappingHash, w.SourceRevisionHash, w.StateHash, w.AssessedAt)
	if e != nil {
		return e
	}
	if appendRevision {
		_, e = tx.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,semantic_mapping_hash,source_revision_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),$14,$15,$15)`,
			seller, w.Domain, w.FactFamily, w.LatestObservedAt, w.LatestCompleteBusinessDate, w.LatestMatureBusinessDate,
			w.LatestCompleteSnapshotAt, w.HistoryFrom, w.HistoryTo, w.DomainRunID, w.SourceContractHash, w.SemanticMappingHash, w.SourceRevisionHash, w.StateHash, w.AssessedAt)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// PersistWatermarkForTest exposes persistWatermark for Prompt №8.2 integration tests.
func (s *Service) PersistWatermarkForTest(ctx context.Context, seller int64, w *Watermark) error {
	return s.persistWatermark(ctx, seller, w)
}
func (s *Service) persistIssue(ctx context.Context, seller, run int64, i DQIssue, at time.Time) error {
	b, _ := json.Marshal(i.Details)
	_, e := s.pool.Exec(ctx, `INSERT INTO data_quality_issues(data_quality_run_id,seller_account_id,domain,dimension,code,period_start,period_end,severity,readiness_effect,retry_class,details_json,detected_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)ON CONFLICT DO NOTHING`, run, seller, i.Domain, i.Dimension, i.Code, i.PeriodStart, i.PeriodEnd, i.Severity, i.ReadinessEffect, i.RetryClass, b, at)
	return e
}
func (s *Service) persistCapability(ctx context.Context, seller, run int64, c *Capability) error {
	b, _ := json.Marshal(c.Details)
	var id int64
	e := s.pool.QueryRow(ctx, `WITH inserted AS(INSERT INTO business_capability_assessments(seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,reason_code,details_json,policy_version,assessed_at,source_domain_run_id)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,''),$16,$17,$18,$19)ON CONFLICT DO NOTHING RETURNING id)SELECT id FROM inserted UNION ALL SELECT id FROM business_capability_assessments WHERE data_quality_run_id=$2 AND capability_key=$3 AND capability_scope=$4 LIMIT 1`, seller, run, c.Key, c.Scope, c.Domain, c.FinalState, c.ContractState, c.AccessState, c.SourceState, c.SemanticState, c.CoverageState, c.TemporalState, c.HistoryState, c.PopulationState, c.ReasonCode, b, factualapi.CurrentPolicies().Capability, c.AssessedAt, c.DomainRunID).Scan(&id)
	c.ID = id
	return e
}
