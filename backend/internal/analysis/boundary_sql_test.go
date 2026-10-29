package analysis_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Forbidden canonical SoT tables that internal/analysis must not query directly
// (Factual API is the sole business-read boundary).
var forbiddenCanonicalTables = []string{
	"canonical_skus", "canonical_sku_versions", "sku_category_assignments",
	"sales_demand_daily", "sales_account_daily", "sales_orders", "sales_postings",
	"sales_posting_versions", "sales_returns", "sales_return_events", "sales_lifecycle_events",
	"inventory_pools", "inventory_locations", "stock_snapshots", "stock_snapshot_lines",
	"price_collection_snapshots", "price_snapshots",
	"advertising_campaigns", "advertising_campaign_versions", "advertising_target_intervals",
	"advertising_daily_facts", "advertising_product_daily_facts", "advertising_cpo_facts",
	"account_health_metrics", "account_health_snapshot_runs", "account_rating_snapshots",
	"account_rating_period_facts", "account_rating_threshold_observations",
	"fbs_error_index_facts", "fbs_posting_error_events",
	"supply_orders", "supply_order_versions", "supplies", "supply_versions",
	"supply_bundle_versions", "supply_planned_items", "supply_state_observations",
	"supply_acceptance_acts", "supply_acceptance_items",
	"seller_categories", "provider_categories", "provider_category_versions",
	"search_sku_summaries", "search_query_details", "finance_accruals",
	"promotions", "promotion_participants",
}

// Metadata tables analysis may touch directly (orchestration / universe / readiness freeze).
var allowedTablePrefixes = []string{
	"analysis_", "factual_universe", "materiality_", "domain_watermark",
	"business_capability_assessment", "data_quality_", "collection_domain_run",
	"collection_run",
}

var sqlTableRef = regexp.MustCompile(`(?i)\b(?:from|join|into|update|table)\s+([a-z_][a-z0-9_]*)\b`)

func TestAnalysisCanonicalSQLBoundary(t *testing.T) {
	root := "."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == "testdata" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lower := strings.ToLower(string(body))
		for _, table := range forbiddenCanonicalTables {
			re := regexp.MustCompile(`\b(?:from|join|into|update|table)\s+` + table + `\b`)
			if re.MatchString(lower) {
				t.Errorf("%s queries forbidden canonical table %s (use Factual API)", filepath.ToSlash(path), table)
			}
		}
		// Metadata-only packages may SQL; any other table name outside allowlist fails.
		for _, m := range sqlTableRef.FindAllStringSubmatch(lower, -1) {
			if len(m) < 2 {
				continue
			}
			table := m[1]
			if isSQLKeywordNoise(table) {
				continue
			}
			if !isAllowedAnalysisTable(table) {
				// Allow only if not a business table we already flagged; catch unknown SoT slips.
				if looksLikeCanonicalBusinessTable(table) {
					t.Errorf("%s queries non-allowlisted table %s", filepath.ToSlash(path), table)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isAllowedAnalysisTable(table string) bool {
	for _, p := range allowedTablePrefixes {
		if strings.HasPrefix(table, p) {
			return true
		}
	}
	switch table {
	case "users", "seller_accounts": // test/bootstrap identity only — still forbidden in non-test via walk skip of _test.go
		return false
	default:
		return false
	}
}

func looksLikeCanonicalBusinessTable(table string) bool {
	for _, p := range []string{
		"canonical_", "sales_", "stock_", "price_", "advertising_", "supply_",
		"search_", "finance_", "promotion", "inventory_", "sku_", "provider_",
		"seller_categor", "account_health", "account_rating", "fbs_",
	} {
		if strings.HasPrefix(table, p) {
			return true
		}
	}
	return false
}

func isSQLKeywordNoise(table string) bool {
	switch table {
	case "select", "where", "set", "values", "lateral", "only", "as", "on",
		"and", "or", "not", "null", "true", "false", "case", "when", "then",
		"else", "end", "limit", "offset", "order", "group", "having", "with",
		"exists", "inserted", "excluded", "now", "coalesce", "nullif":
		return true
	default:
		return false
	}
}
