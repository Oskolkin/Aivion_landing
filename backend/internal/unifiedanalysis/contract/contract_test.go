package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func TestVersionsDistinctFromLegacyDataPack(t *testing.T) {
	const legacyDataPackSchema = "ai_data_pack_v1"
	if AnalysisPackSchemaVersionV1 == "" || UnifiedAnalysisOutputSchemaVersionV1 == "" {
		t.Fatal("versions must not be empty")
	}
	if AnalysisPackSchemaVersionV1 == legacyDataPackSchema {
		t.Fatalf("pack version collides with legacy %s", legacyDataPackSchema)
	}
	if UnifiedAnalysisOutputSchemaVersionV1 == legacyDataPackSchema {
		t.Fatalf("output version collides with legacy %s", legacyDataPackSchema)
	}
	if AnalysisPackSchemaVersionV1 == UnifiedAnalysisOutputSchemaVersionV1 {
		t.Fatal("pack and output versions must differ")
	}
}

func TestAnalysisPackFilesUniqueStableRequired(t *testing.T) {
	seen := map[string]bool{}
	var required []string
	for i, f := range AnalysisPackFilesV1 {
		if f.Name == "" {
			t.Fatalf("empty file name at %d", i)
		}
		if seen[f.Name] {
			t.Fatalf("duplicate file %s", f.Name)
		}
		seen[f.Name] = true
		ext := filepath.Ext(f.Name)
		if ext != ".json" && ext != ".csv" {
			t.Fatalf("unknown extension for %s", f.Name)
		}
		if f.Kind != "json" && f.Kind != "csv" {
			t.Fatalf("unknown kind %s for %s", f.Kind, f.Name)
		}
		if !f.Required {
			t.Fatalf("%s must be required (stable ZIP presence)", f.Name)
		}
		required = append(required, f.Name)
	}
	wantOrder := []string{
		"manifest.json",
		"data_dictionary.json",
		"analysis_policy.json",
		"limitations.json",
		"account_summary.csv",
		"account_daily.csv",
		"sku_summary.csv",
		"sku_daily.csv",
		"campaign_summary.csv",
		"campaign_daily.csv",
		"campaign_sku.csv",
		"product_enrichment.csv",
	}
	if !slices.Equal(required, wantOrder) {
		t.Fatalf("required/order=%v want=%v", required, wantOrder)
	}
	if !PackFilePresenceStable {
		t.Fatal("PackFilePresenceStable must be true for v1")
	}
	if AnalysisPackDailyHistoryDaysV1 != 30 {
		t.Fatalf("pack history days=%d", AnalysisPackDailyHistoryDaysV1)
	}
	if LegacyDataPackDailyHistoryDays != 14 {
		t.Fatalf("legacy history days=%d", LegacyDataPackDailyHistoryDays)
	}
}

func TestCSVColumnsContract(t *testing.T) {
	snake := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, f := range AnalysisPackFilesV1 {
		if f.Kind != "csv" {
			continue
		}
		cols, ok := AnalysisPackCSVColumnsV1[f.Name]
		if !ok {
			t.Fatalf("missing columns for %s", f.Name)
		}
		if len(cols) == 0 {
			t.Fatalf("empty columns for %s", f.Name)
		}
		if cols[0] != "row_id" {
			t.Fatalf("%s first column=%s want row_id", f.Name, cols[0])
		}
		seen := map[string]bool{}
		for _, c := range cols {
			if c == "" {
				t.Fatalf("%s has empty column", f.Name)
			}
			if !snake.MatchString(c) {
				t.Fatalf("%s column %q is not snake_case", f.Name, c)
			}
			if seen[c] {
				t.Fatalf("%s duplicate column %s", f.Name, c)
			}
			seen[c] = true
		}
	}
}

func TestPolicyDefaults(t *testing.T) {
	p := DefaultAnalysisPolicyV1()
	if p.MaxActions != 15 || p.MaxTopActions != 3 {
		t.Fatalf("max_actions=%d max_top_actions=%d", p.MaxActions, p.MaxTopActions)
	}
	if slices.Contains(p.AllowedPriorities, "growth") {
		t.Fatal("growth priority must not be allowed")
	}
	if slices.Contains(p.AllowedActionCategories, "data_quality") {
		t.Fatal("data_quality must stay out of user-facing v1 contract")
	}
}

func TestActionEnumsMatchActiveTaxonomy(t *testing.T) {
	wantTypes := make([]string, len(actions.ActionsAIFirstActiveActionTypes))
	for i, v := range actions.ActionsAIFirstActiveActionTypes {
		wantTypes[i] = string(v)
	}
	if !slices.Equal(AllowedActionTypesV1, wantTypes) {
		t.Fatalf("types=%v want=%v", AllowedActionTypesV1, wantTypes)
	}
	// Bidirectional: every contract type is active, every active type is in contract.
	activeSet := toSet(wantTypes)
	contractSet := toSet(AllowedActionTypesV1)
	for typ := range activeSet {
		if !contractSet[typ] {
			t.Fatalf("active type missing from contract: %s", typ)
		}
	}
	for typ := range contractSet {
		if !activeSet[typ] {
			t.Fatalf("contract type not in active taxonomy: %s", typ)
		}
	}

	wantCats := make([]string, len(actions.ActionsAIFirstActiveCategories))
	for i, v := range actions.ActionsAIFirstActiveCategories {
		wantCats[i] = string(v)
	}
	if !slices.Equal(AllowedActionCategoriesV1, wantCats) {
		t.Fatalf("categories=%v want=%v", AllowedActionCategoriesV1, wantCats)
	}
	catActive := toSet(wantCats)
	catContract := toSet(AllowedActionCategoriesV1)
	for c := range catActive {
		if !catContract[c] {
			t.Fatalf("active category missing from contract: %s", c)
		}
	}
	for c := range catContract {
		if !catActive[c] {
			t.Fatalf("contract category not active: %s", c)
		}
	}

	wantPri := []string{
		string(actions.ActionPriorityCritical),
		string(actions.ActionPriorityImportant),
		string(actions.ActionPriorityWatch),
	}
	if !slices.Equal(AllowedActionPrioritiesV1, wantPri) {
		t.Fatalf("priorities=%v want=%v", AllowedActionPrioritiesV1, wantPri)
	}
	priActive := toSet(wantPri)
	priContract := toSet(AllowedActionPrioritiesV1)
	for p := range priActive {
		if !priContract[p] {
			t.Fatalf("active priority missing from contract: %s", p)
		}
	}
	for p := range priContract {
		if !priActive[p] {
			t.Fatalf("contract priority not active: %s", p)
		}
	}

	// Explicit exclusions from user-facing Unified v1.
	excludedTypes := []string{
		string(actions.ActionTypeKeySKURisk),
		string(actions.ActionTypeAdHighDRR),
		string(actions.ActionTypeDataQualityIssue),
	}
	for _, typ := range excludedTypes {
		if contractSet[typ] {
			t.Fatalf("excluded legacy/internal type unexpectedly allowed: %s", typ)
		}
	}
	if catContract[string(actions.ActionCategoryDataQuality)] {
		t.Fatal("data_quality category must stay excluded from user-facing v1")
	}
	if priContract["growth"] {
		t.Fatal("growth priority must stay excluded")
	}
}

func TestInvalidAnalysisOutcomeRejectedByAllowlist(t *testing.T) {
	if slices.Contains(AllowedAnalysisOutcomesV1, "partial_success") {
		t.Fatal("unknown outcome must not be allowed")
	}
	for _, o := range AllowedAnalysisOutcomesV1 {
		if o == "" {
			t.Fatal("empty outcome")
		}
	}
}

func TestNullSemanticsCatalogCoversCriticalMetrics(t *testing.T) {
	for _, key := range []string{
		"orders_delta_pct", "revenue_delta_pct", "drr_pct", "ctr_pct",
		"conversion_pct", "stock_coverage_days", "average_daily_sales",
		"market_price_index", "average_daily_sales_30d", "stock_coverage_days_30d",
	} {
		if MetricNullSemanticsV1[key] == "" {
			t.Fatalf("missing null semantics for %s", key)
		}
	}
}

func TestManifestExampleFixture(t *testing.T) {
	raw := readTestdata(t, "manifest_v1.example.json")
	var m AnalysisPackManifestV1
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.SchemaVersion != AnalysisPackSchemaVersionV1 {
		t.Fatalf("schema=%s", m.SchemaVersion)
	}
	roundTripJSON(t, m)
}

func TestUnifiedOutputFixtures(t *testing.T) {
	cases := []struct {
		file              string
		outcome           string
		expectActions     bool
		expectLimitations bool
	}{
		{"unified_output_v1.valid.json", AnalysisOutcomeMaterialActionsFound, true, true},
		{"unified_output_v1.no_actions.json", AnalysisOutcomeNoMaterialActions, false, false},
		{"unified_output_v1.insufficient_data.json", AnalysisOutcomeInsufficientData, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw := readTestdata(t, tc.file)
			var out UnifiedAnalysisOutputV1
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if out.SchemaVersion != UnifiedAnalysisOutputSchemaVersionCurrent {
				t.Fatalf("schema=%s want %s", out.SchemaVersion, UnifiedAnalysisOutputSchemaVersionCurrent)
			}
			if out.AnalysisOutcome != tc.outcome {
				t.Fatalf("outcome=%s want=%s", out.AnalysisOutcome, tc.outcome)
			}
			if out.Dashboard.AccountSummary.Status == "" {
				t.Fatal("dashboard.account_summary.status required")
			}
			if tc.expectActions && len(out.Actions) == 0 {
				t.Fatal("expected actions")
			}
			if !tc.expectActions && len(out.Actions) != 0 {
				t.Fatalf("expected empty actions, got %d", len(out.Actions))
			}
			if tc.expectLimitations && len(out.Limitations) == 0 && tc.outcome == AnalysisOutcomeInsufficientData {
				t.Fatal("insufficient_data requires limitations")
			}
			assertNoLegacyIDs(t, raw)
			assertActionEnums(t, out)
			if tc.file == "unified_output_v1.valid.json" {
				assertValidFixtureLinks(t, out)
			}
			if tc.file == "unified_output_v1.no_actions.json" {
				if len(out.Dashboard.TopActionIDs) != 0 || len(out.Dashboard.RecommendedActionIDs) != 0 {
					t.Fatal("no_actions fixture must clear action id refs")
				}
			}
			roundTripJSON(t, out)
		})
	}
}

func assertValidFixtureLinks(t *testing.T, out UnifiedAnalysisOutputV1) {
	t.Helper()
	ids := map[string]bool{}
	for _, a := range out.Actions {
		if a.AnalysisActionID == "" {
			t.Fatal("empty analysis_action_id")
		}
		if ids[a.AnalysisActionID] {
			t.Fatalf("duplicate analysis_action_id %s", a.AnalysisActionID)
		}
		ids[a.AnalysisActionID] = true
	}
	if len(out.Dashboard.TopActionIDs) < 3 {
		t.Fatalf("top_action_ids=%d", len(out.Dashboard.TopActionIDs))
	}
	for _, id := range out.Dashboard.TopActionIDs {
		if !ids[id] {
			t.Fatalf("top_action_id %s missing from actions", id)
		}
	}
	for _, id := range out.Dashboard.RecommendedActionIDs {
		if !ids[id] {
			t.Fatalf("recommended_action_id %s missing from actions", id)
		}
	}
	if len(out.Dashboard.CategorySummaries) < 2 {
		t.Fatal("expected multiple category summaries")
	}
	scopes := map[string]bool{}
	for _, a := range out.Actions {
		scopes[a.Scope.ScopeType] = true
	}
	for _, need := range []string{"sku", "campaign", "funnel"} {
		if !scopes[need] {
			t.Fatalf("missing scope_type %s", need)
		}
	}
}

func assertActionEnums(t *testing.T, out UnifiedAnalysisOutputV1) {
	t.Helper()
	typeSet := toSet(AllowedActionTypesV1)
	catSet := toSet(AllowedActionCategoriesV1)
	priSet := toSet(AllowedActionPrioritiesV1)
	tierSet := toSet(AllowedExecutionTiersV1)
	confSet := toSet(AllowedActionConfidenceV1)
	scopeSet := toSet(AllowedActionScopeTypesV1)
	for _, a := range out.Actions {
		if !typeSet[a.Type] {
			t.Fatalf("type %s not allowed", a.Type)
		}
		if !catSet[a.Category] {
			t.Fatalf("category %s not allowed", a.Category)
		}
		if !priSet[a.Priority] {
			t.Fatalf("priority %s not allowed", a.Priority)
		}
		if !tierSet[a.ExecutionTier] {
			t.Fatalf("execution_tier %s not allowed", a.ExecutionTier)
		}
		if !confSet[a.Confidence] {
			t.Fatalf("confidence %s not allowed", a.Confidence)
		}
		if !scopeSet[a.Scope.ScopeType] {
			t.Fatalf("scope_type %s not allowed", a.Scope.ScopeType)
		}
	}
}

func assertNoLegacyIDs(t *testing.T, raw []byte) {
	t.Helper()
	s := string(raw)
	forbidden := []string{
		`"pipeline_run_id"`,
		`"actions_run_id"`,
		`"dashboard_run_id"`,
		`"global_context_hash"`,
		`"source_candidate_ids"`,
		`"narrative_run_id"`,
		`"prompt_version"`,
	}
	for _, key := range forbidden {
		if strings.Contains(s, key) {
			t.Fatalf("fixture contains forbidden legacy field %s", key)
		}
	}
}

func roundTripJSON(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again map[string]any
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("remarshal: %v", err)
	}
}

func toSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}
