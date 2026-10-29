package factual

import (
	"os"
	"strings"
	"testing"
)

func functionBody(t *testing.T, source, start, next string) string {
	t.Helper()
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("function marker not found: %s", start)
	}
	j := strings.Index(s[i+len(start):], next)
	if j < 0 {
		t.Fatalf("next marker not found: %s", next)
	}
	return s[i : i+len(start)+j]
}

func TestLifecycleListsUseBoundedBatchQueries(t *testing.T) {
	p := functionBody(t, "lifecycle.go", "func (s *Service) ListPostings", "func (s *Service) ListLifecycleEvents")
	if strings.Contains(p, "s.GetPosting(") || !strings.Contains(p, "s.batchPostings(") {
		t.Fatal("ListPostings regressed to per-row GetPosting")
	}
	r := functionBody(t, "lifecycle.go", "func (s *Service) ListReturns", "func (s *Service) batchPostings")
	if strings.Contains(r, "s.GetReturn(") || !strings.Contains(r, "s.batchReturns(") {
		t.Fatal("ListReturns regressed to per-row GetReturn")
	}
}

func TestBatchGetSKUSearchSummaryUsesSetBasedSQL(t *testing.T) {
	body := functionBody(t, "search.go", "func (s *Service) BatchGetSKUSearchSummary", "func (s *Service) ListSKUSearchSummary")
	if strings.Contains(body, "s.GetSKUSearchSummary(") {
		t.Fatal("BatchGetSKUSearchSummary regressed to per-id GetSKUSearchSummary")
	}
	if !strings.Contains(body, "ANY($2)") && !strings.Contains(body, "unnest($2") {
		t.Fatal("BatchGetSKUSearchSummary missing bounded set-based SKU filter")
	}
}

func TestPrompt91BatchMethodsUseSetBasedSQL(t *testing.T) {
	ads := functionBody(t, "advertising.go", "func (s *Service) BatchGetSKUAdvertising", "func (s *Service) BatchGetCampaignPeriodPerformance")
	if strings.Contains(ads, "s.GetSKUAdvertising(") {
		t.Fatal("BatchGetSKUAdvertising regressed to per-id GetSKUAdvertising")
	}
	if !strings.Contains(ads, "promoted_canonical_sku_id=ANY($2)") {
		t.Fatal("BatchGetSKUAdvertising missing set-based SKU filter")
	}
	camp := functionBody(t, "advertising.go", "func (s *Service) BatchGetCampaignPeriodPerformance", "func ratioCountsPct")
	if strings.Contains(camp, "s.GetCampaignDailyFacts(") || strings.Contains(camp, "s.GetCampaignDRR(") {
		t.Fatal("BatchGetCampaignPeriodPerformance regressed to per-campaign reads")
	}
	if !strings.Contains(camp, "advertising_campaign_id=ANY($2)") {
		t.Fatal("BatchGetCampaignPeriodPerformance missing set-based campaign filter")
	}
	inboundSrc, e := os.ReadFile("supply.go")
	if e != nil {
		t.Fatal(e)
	}
	inbound := string(inboundSrc)
	i := strings.Index(inbound, "func (s *Service) BatchGetSKUInbound")
	if i < 0 {
		t.Fatal("BatchGetSKUInbound missing")
	}
	inbound = inbound[i:]
	if strings.Contains(inbound, "s.GetSKUInbound(") {
		t.Fatal("BatchGetSKUInbound regressed to per-id GetSKUInbound")
	}
	if !strings.Contains(inbound, "canonical_sku_id=ANY($2)") {
		t.Fatal("BatchGetSKUInbound missing set-based SKU filter")
	}
	cov := functionBody(t, "stock.go", "func (s *Service) BatchGetStockCoverage", "func coverageValue")
	if strings.Contains(cov, "s.GetStockCoverage(") {
		t.Fatal("BatchGetStockCoverage must not loop GetStockCoverage")
	}
	if !strings.Contains(cov, "s.BatchGetSKUStock(") || !strings.Contains(cov, "s.BatchGetSKUDemand(") {
		t.Fatal("BatchGetStockCoverage must reuse BatchGetSKUStock and BatchGetSKUDemand")
	}
}

func TestPostingCutoffQueryNeverReadsMutableProjection(t *testing.T) {
	g := functionBody(t, "lifecycle.go", "func (s *Service) GetPosting", "func (s *Service) ListPostings")
	for _, forbidden := range []string{"p.fulfillment_scheme", "p.order_resolution_status", "p.canonical_order_id"} {
		if strings.Contains(g, forbidden) {
			t.Fatalf("future-knowledge projection used: %s", forbidden)
		}
	}
	if !strings.Contains(g, "sales_posting_relation_versions") {
		t.Fatal("split relation is not cutoff-versioned")
	}
}
