package factualapi

import (
	"bufio"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVNextConsumersCannotBypassFactualBoundary(t *testing.T) {
	allow := map[string]bool{}
	f, e := os.Open(filepath.Join("..", "..", "..", "docs", "architecture", "v1", "factual-api-legacy-allowlist.txt"))
	if e != nil {
		t.Fatal(e)
	}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(strings.Split(scan.Text(), "#")[0])
		if line != "" {
			allow[filepath.ToSlash(line)] = true
		}
	}
	_ = f.Close()
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join("..", "..")
	e = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, pe := os.ReadFile(path)
		if pe != nil {
			return pe
		}
		parsed, pe := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
		if pe != nil {
			return pe
		}
		usesAPI, persistenceImport, canonicalAdapterImport := false, false, false
		for _, im := range parsed.Imports {
			p := strings.Trim(im.Path.Value, "\"")
			usesAPI = usesAPI || strings.HasSuffix(p, "/internal/factualapi")
			persistenceImport = persistenceImport || strings.Contains(p, "/internal/dbgen") || strings.Contains(p, "/internal/db") || strings.Contains(p, "pgx")
			canonicalAdapterImport = canonicalAdapterImport ||
				(strings.Contains(p, "/internal/factual/") || strings.HasSuffix(p, "/internal/factual")) ||
				strings.Contains(p, "/internal/collection/catalog") || strings.Contains(p, "/internal/collection/sales") || strings.Contains(p, "/internal/collection/lifecycle") || strings.Contains(p, "/internal/collection/stock") || strings.Contains(p, "/internal/collection/price") || strings.Contains(p, "/internal/collection/advertising") || strings.Contains(p, "/internal/collection/accounthealth") || strings.Contains(p, "/internal/collection/supply")
			// Note: /internal/factualapi is the approved boundary and must NOT match /internal/factual.
		}
		body := strings.ToLower(string(source))
		canonicalSQL := false
		for _, table := range []string{"canonical_skus", "canonical_sku_versions", "sku_category_assignments", "sales_demand_daily", "sales_account_daily", "sales_orders", "sales_postings", "sales_posting_versions", "sales_returns", "sales_return_events", "sales_lifecycle_events", "inventory_pools", "inventory_locations", "stock_snapshots", "stock_snapshot_lines", "price_collection_snapshots", "price_snapshots", "advertising_campaigns", "advertising_campaign_versions", "advertising_target_intervals", "advertising_daily_facts", "advertising_product_daily_facts", "advertising_cpo_facts", "account_health_metrics", "account_health_snapshot_runs", "account_rating_snapshots", "account_rating_period_facts", "account_rating_threshold_observations", "fbs_error_index_facts", "fbs_posting_error_events", "supply_orders", "supply_order_versions", "supplies", "supply_versions", "supply_bundle_versions", "supply_planned_items", "supply_state_observations", "supply_acceptance_acts", "supply_acceptance_items"} {
			canonicalSQL = canonicalSQL || regexp.MustCompile(`\b(?:from|join|into|update|table)\s+`+table+`\b`).MatchString(body)
		}
		if (usesAPI && persistenceImport) || canonicalAdapterImport || canonicalSQL {
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			rel = filepath.ToSlash(rel)
			ok := false
			for prefix := range allow {
				if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
					ok = true
				}
			}
			if !ok {
				t.Errorf("new canonical consumer %s bypasses factualapi; deliberate allowlist change required", rel)
			}
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
