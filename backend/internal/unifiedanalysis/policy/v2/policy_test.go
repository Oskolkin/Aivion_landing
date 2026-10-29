package policyv2

import (
	"sort"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/thresholdpolicy"
)

// pinnedPolicyHashV2 pins the sha256 digest of the embedded
// analysis_policy_v2.json (LF-normalized). Changing the JSON file requires
// deliberately recomputing and updating both this constant and
// CanonicalPolicyHashV2 in policy.go — never edit the JSON without
// recomputing the hash.
const pinnedPolicyHashV2 = "sha256:35f92ae0ba05cd4bdc08423472cd4a89b6d71d1dbd6b8bdaf9487456fa6d9063"

func TestLoad_Succeeds(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.PolicyVersion != Version {
		t.Fatalf("policy_version=%q want=%q", p.PolicyVersion, Version)
	}
	if Version != "analysis_policy_v2" {
		t.Fatalf("Version=%q want=analysis_policy_v2", Version)
	}
}

func TestLoad_MemoizedAcrossCalls(t *testing.T) {
	p1, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatal("Load must return the same cached *Policy pointer across calls")
	}
}

func TestHash_Stable(t *testing.T) {
	h1, err := Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("Hash must be stable across calls: %s vs %s", h1, h2)
	}
	if len(h1) < len("sha256:") || h1[:7] != "sha256:" {
		t.Fatalf("Hash must be sha256:<hex> form, got %s", h1)
	}
}

func TestHash_PinnedValue(t *testing.T) {
	h, err := Hash()
	if err != nil {
		t.Fatal(err)
	}
	if pinnedPolicyHashV2 == "PLACEHOLDER_COMPUTE" {
		t.Fatalf("set pinnedPolicyHashV2 to %s", h)
	}
	if h != pinnedPolicyHashV2 {
		t.Fatalf("policy hash drift: got=%s want=%s (update pinnedPolicyHashV2 AND CanonicalPolicyHashV2 deliberately)", h, pinnedPolicyHashV2)
	}
	if h != CanonicalPolicyHashV2 {
		t.Fatalf("Hash()=%s must equal CanonicalPolicyHashV2=%s", h, CanonicalPolicyHashV2)
	}
}

func TestFindingTypeRegistry_NoDuplicatesAndComplete(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, ft := range p.FindingTypeRegistry {
		if seen[ft.FindingType] {
			t.Fatalf("duplicate finding_type %q", ft.FindingType)
		}
		seen[ft.FindingType] = true
	}

	want := []string{
		// sales
		"revenue_decline", "revenue_growth", "orders_decline", "orders_growth",
		"average_order_value_decline", "average_order_value_growth", "sales_stable",
		// funnel
		"traffic_decline", "impression_to_view_decline", "view_to_cart_decline",
		"cart_to_order_decline", "order_to_delivery_decline", "funnel_conversion_improved",
		// stock
		"stockout", "critical_coverage", "short_coverage", "no_sales_with_stock",
		"slow_turnover", "surplus", "coverage_healthy",
		// advertising
		"high_drr_campaign", "spend_without_orders", "ad_share_growth",
		"ad_share_decline", "campaign_efficiency_improved", "budget_concentration",
		// sku
		"sku_orders_drop", "key_sku_stock_risk", "key_sku_growth", "key_sku_concentration",
		// category
		"category_underperformance", "category_growth", "category_concentration",
		// data_quality
		"history_missing", "source_incomplete", "data_stale",
	}
	for _, ftype := range want {
		if !seen[ftype] {
			t.Fatalf("finding_type_registry missing required finding_type %q", ftype)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("finding_type_registry has %d entries, want exactly %d (got=%v)", len(seen), len(want), p.FindingTypesSorted())
	}
}

func TestFindingTypeRegistry_FieldsWellFormed(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	validDirections := map[string]bool{"positive": true, "negative": true, "neutral": true}
	validSeverities := map[string]bool{"critical": true, "important": true, "watch": true, "none": true}
	validScopes := map[string]bool{"account": true, "funnel": true, "sku": true, "campaign": true, "category": true}
	for _, ft := range p.FindingTypeRegistry {
		for _, d := range ft.AllowedDirections {
			if !validDirections[d] {
				t.Fatalf("finding_type %q has invalid direction %q", ft.FindingType, d)
			}
		}
		for _, s := range ft.AllowedSeverities {
			if !validSeverities[s] {
				t.Fatalf("finding_type %q has invalid severity %q", ft.FindingType, s)
			}
		}
		for _, s := range ft.AllowedScopes {
			if !validScopes[s] {
				t.Fatalf("finding_type %q has invalid scope %q", ft.FindingType, s)
			}
		}
	}
}

func TestActionTypeRegistry_MatchesV10ContractExactly(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := p.ActionTypesSorted()
	want := append([]string(nil), contract.AllowedActionTypesV1...)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("action_type_registry has %d types, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("action_type_registry[%d]=%q want=%q (got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}

func TestActionTypeToCategory_AllMappedAndMatchesProduction(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, actionType := range contract.AllowedActionTypesV1 {
		gotCategory, ok := p.CategoryForActionType(actionType)
		if !ok {
			t.Fatalf("action_type_to_category missing entry for %q", actionType)
		}
		wantCategory, ok := actions.ActionCategoryForType(actions.ActionType(actionType))
		if !ok {
			t.Fatalf("actions.ActionCategoryForType has no category for %q (production mapping drifted)", actionType)
		}
		if gotCategory != string(wantCategory) {
			t.Fatalf("action_type_to_category[%s]=%q want=%q (production actions.ActionCategoryForType)", actionType, gotCategory, wantCategory)
		}
	}
	// Exact map given in the task spec.
	wantExact := map[string]string{
		"stock_replenishment":       "stock",
		"stock_redistribution":      "stock",
		"slow_turnover_review":      "slow_turnover",
		"no_sales_with_stock":       "slow_turnover",
		"sku_orders_drop":           "sku",
		"key_sku_protection":        "sku",
		"ad_high_drr_review":        "advertising",
		"ad_spend_no_orders":        "advertising",
		"ad_budget_reallocation":    "advertising",
		"funnel_weak_transition":    "funnel",
		"card_conversion_review":    "funnel",
		"category_underperformance": "category",
		"sales_drop_review":         "sales",
	}
	if len(p.ActionTypeToCategory) != len(wantExact) {
		t.Fatalf("action_type_to_category has %d entries, want %d", len(p.ActionTypeToCategory), len(wantExact))
	}
	for actionType, wantCategory := range wantExact {
		if got := p.ActionTypeToCategory[actionType]; got != wantCategory {
			t.Fatalf("action_type_to_category[%s]=%q want=%q", actionType, got, wantCategory)
		}
	}
}

func TestActionTypeRegistry_RequiredFindingTypesResolve(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range p.ActionTypeRegistry {
		if len(at.RequiredFindingTypes) == 0 {
			t.Fatalf("action_type %q has no required_finding_types", at.ActionType)
		}
		for _, ftype := range at.RequiredFindingTypes {
			if _, ok := p.FindingType(ftype); !ok {
				t.Fatalf("action_type %q references unknown finding_type %q", at.ActionType, ftype)
			}
		}
	}
}

func TestActionLimits_MatchContractDefaults(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.ActionLimits.MaxActions != contract.MaxActionsV1 {
		t.Fatalf("action_limits.max_actions=%d want=%d", p.ActionLimits.MaxActions, contract.MaxActionsV1)
	}
	if p.ActionLimits.MaxTopActions != contract.MaxTopActionsV1 {
		t.Fatalf("action_limits.max_top_actions=%d want=%d", p.ActionLimits.MaxTopActions, contract.MaxTopActionsV1)
	}
	if p.ActionLimits.MinActionsMaterial != contract.MinActionsMaterialV1 {
		t.Fatalf("action_limits.min_actions_material=%d want=%d", p.ActionLimits.MinActionsMaterial, contract.MinActionsMaterialV1)
	}
	if p.ActionLimits.MinRequiredMaterial != contract.MinRequiredActionsMaterialV1 {
		t.Fatalf("action_limits.min_required_material=%d want=%d", p.ActionLimits.MinRequiredMaterial, contract.MinRequiredActionsMaterialV1)
	}
	if p.ActionLimits.MaxActions != 15 {
		t.Fatalf("action_limits.max_actions=%d want=15", p.ActionLimits.MaxActions)
	}
	if p.ActionLimits.MaxTopActions != 3 {
		t.Fatalf("action_limits.max_top_actions=%d want=3", p.ActionLimits.MaxTopActions)
	}
}

func TestThresholds_MatchProductionRegistry(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Thresholds) != len(thresholdpolicy.Registry) {
		t.Fatalf("thresholds has %d entries, want %d (production thresholdpolicy.Registry)", len(p.Thresholds), len(thresholdpolicy.Registry))
	}
	for _, prod := range thresholdpolicy.Registry {
		got, ok := p.Threshold(prod.PolicyID)
		if !ok {
			t.Fatalf("policy_v2 missing threshold for policy_id %q", prod.PolicyID)
		}
		if got.MetricCode != prod.MetricCode {
			t.Fatalf("threshold[%s].metric_code=%q want=%q", prod.PolicyID, got.MetricCode, prod.MetricCode)
		}
		if got.Value != prod.Value {
			t.Fatalf("threshold[%s].value=%v want=%v", prod.PolicyID, got.Value, prod.Value)
		}
	}

	high, ok := p.Threshold(thresholdpolicy.PolicyHighDRRCampaignThreshold)
	if !ok {
		t.Fatal("missing high_drr_campaign_threshold")
	}
	if high.Value != 80 {
		t.Fatalf("high_drr_campaign_threshold value=%v want=80", high.Value)
	}
	if high.Operator != ">" {
		t.Fatalf("high_drr_campaign_threshold operator=%q want=\">\"", high.Operator)
	}

	low, ok := p.Threshold(thresholdpolicy.PolicyLowStockCoverageDays)
	if !ok {
		t.Fatal("missing low_stock_coverage_days_threshold")
	}
	if low.Value != 7 {
		t.Fatalf("low_stock_coverage_days_threshold value=%v want=7", low.Value)
	}
	if low.Operator != "<" {
		t.Fatalf("low_stock_coverage_days_threshold operator=%q want=\"<\"", low.Operator)
	}
}

func TestLimitationRegistry_IncludesExistingAndNewCodes(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	existingPackCodes := []string{
		"sku_funnel_metrics_unavailable",
		"sku_order_quality_unavailable",
		"sku_reserved_stock_unknown",
		"sku_ad_spend_not_attributed",
		"campaign_sku_spend_not_attributed",
		"product_content_score_unavailable",
		"product_attribute_completion_unknown",
		"account_metrics_unavailable",
		"account_funnel_unavailable",
		"product_enrichment_unavailable",
	}
	newCodes := []string{
		"future_supply_visibility_limited",
		"history_missing",
		"source_incomplete",
		"data_stale",
	}
	for _, code := range append(existingPackCodes, newCodes...) {
		if _, ok := p.Limitation(code); !ok {
			t.Fatalf("limitation_registry missing code %q", code)
		}
	}
}

func TestSKUFocusLimits_MatchV11SchemaCaps(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"order_drop":          7,
		"stock_risk":          7,
		"no_sales_with_stock": 7,
		"slow_turnover":       7,
		"key_skus":            7,
	}
	for k, v := range want {
		if got := p.SKUFocusLimits[k]; got != v {
			t.Fatalf("sku_focus_limits[%s]=%d want=%d", k, got, v)
		}
	}
}

func TestCategoryLimits(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.CategoryLimits.MaxItems != 10 {
		t.Fatalf("category_limits.max_items=%d want=10", p.CategoryLimits.MaxItems)
	}
}

func TestRecommendedCheckTypes_HasFunnelStockAdvertising(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"funnel", "stock", "advertising"} {
		if len(p.RecommendedCheckTypes[domain]) == 0 {
			t.Fatalf("recommended_check_types[%s] must not be empty", domain)
		}
	}
}

func TestUserFacingTextFields_NonEmptyAndHasTextLimits(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.UserFacingTextFields) == 0 {
		t.Fatal("user_facing_text_fields must not be empty")
	}
	for _, field := range p.UserFacingTextFields {
		if !p.IsUserFacingTextField(field) {
			t.Fatalf("IsUserFacingTextField(%q) = false, want true", field)
		}
		if _, ok := p.TextLimit(field); !ok {
			t.Fatalf("user_facing_text_field %q has no text_limits entry", field)
		}
	}
}

func TestValidate_CurrentEmbeddedDocumentPasses(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("embedded policy must self-validate: %v", err)
	}
}

func TestValidate_RejectsDuplicateFindingType(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	broken := *p
	broken.FindingTypeRegistry = append(append([]FindingTypeEntry(nil), p.FindingTypeRegistry...), p.FindingTypeRegistry[0])
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate must reject duplicate finding_type")
	}
}

func TestValidate_RejectsUnmappedActionType(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	broken := *p
	broken.ActionTypeToCategory = map[string]string{}
	for k, v := range p.ActionTypeToCategory {
		broken.ActionTypeToCategory[k] = v
	}
	delete(broken.ActionTypeToCategory, "stock_replenishment")
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate must reject an action_type missing from action_type_to_category")
	}
}

func TestValidate_RejectsInvalidThresholdOperator(t *testing.T) {
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	broken := *p
	broken.Thresholds = append([]ThresholdEntry(nil), p.Thresholds...)
	broken.Thresholds[0].Operator = "not_an_operator"
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate must reject an invalid threshold operator")
	}
}

// TestProductionDoesNotAutoSwitch documents (rather than enforces via the
// compiler) that adopting analysis_policy_v2 in the runtime pipeline is a
// deliberate, separate change: this package is not imported by
// unifiedanalysis/service.go, unifiedanalysis/pack, or
// unifiedanalysis/thresholdpolicy today.
func TestProductionDoesNotAutoSwitch(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must be set")
	}
	// thresholdpolicy remains the production registry; analysis_policy_v2
	// is additive and must not redefine or shadow it.
	if _, ok := thresholdpolicy.Lookup("high_drr_campaign_threshold"); !ok {
		t.Fatal("production thresholdpolicy registry must remain independently intact")
	}
}
