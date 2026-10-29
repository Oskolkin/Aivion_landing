//go:build integration

package analysis_test

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func TestPrompt96CoreQueryCount500FullV4(t *testing.T) {
	pool, counter := newCountedPool(t)
	n := prompt94MeasureBuildCore(t, pool, counter, 500)
	t.Logf("PROMPT96 Core 500 SKU query count = %d", n)
	if n > coreQueryCountCeiling500 {
		t.Fatalf("query count %d exceeds ceiling %d", n, coreQueryCountCeiling500)
	}
}

func TestPrompt96CoreQueryShape2000FullV4(t *testing.T) {
	pool, counter := newCountedPool(t)
	q500 := prompt94MeasureBuildCore(t, pool, counter, 500)
	counter.Reset()
	q2000 := prompt94MeasureBuildCore(t, pool, counter, 2000)
	t.Logf("PROMPT96 Core query counts: 500=%d 2000=%d", q500, q2000)
	if q2000 > q500+coreQueryShapeOverhead {
		t.Fatalf("2000-SKU query count %d exceeds 500-SKU %d + overhead %d", q2000, q500, coreQueryShapeOverhead)
	}
}

func TestPrompt96DetailQueryCountMaxBudgetFullV4(t *testing.T) {
	TestPrompt94DetailQueryCountMaxBudget(t)
}

func TestPrompt96AdvertisingTrafficAttributionUseIndependentPeriods(t *testing.T) {
	pool, _ := newCountedPool(t)
	f := prompt94ScaleSeed(t, pool, 40)
	ctx := context.Background()
	trafficTo := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	attrTo := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	searchTo := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	queryTo := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	funnelTo := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	financeTo := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	knowledge := f.cut.Add(-time.Minute)
	upd := func(family string, to time.Time) {
		t.Helper()
		tag, err := pool.Exec(ctx, `UPDATE domain_watermarks SET latest_complete_business_date=$3::date, latest_mature_business_date=$3::date, history_available_to=$3::date, assessed_at=$4
			WHERE seller_account_id=$1 AND fact_family=$2`, f.seller, family, to, knowledge)
		if err != nil {
			t.Fatal(err)
		}
		if tag.RowsAffected() == 0 {
			return
		}
		if _, err := pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,state_hash,assessed_at,knowledge_time)
			SELECT seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,state_hash,assessed_at,$3
			FROM domain_watermarks WHERE seller_account_id=$1 AND fact_family=$2`, f.seller, family, knowledge); err != nil {
			t.Fatal(err)
		}
	}
	upd("ADVERTISING.TRAFFIC", trafficTo)
	upd("ADVERTISING.ATTRIBUTION", attrTo)
	upd("SEARCH.SELLER_SUMMARY", searchTo)
	upd("SEARCH.QUERY_DETAILS", queryTo)
	upd("FUNNEL.DAILY_ADDITIVE", funnelTo)
	upd("FINANCE.DAILY_LEDGER", financeTo)

	_, binding, sealed := prompt94SealFixture(t, f)
	api := prompt94FactualAPI(t, pool)
	builder := landscape.CoreBuilder{
		Source:   landscape.FactualAPISource{API: api},
		Profiles: landscape.DefaultCoreProfilesV5(),
	}
	core, _, _, err := builder.BuildCore(context.Background(), sealed, binding.ID, binding.Hash, f.bundle.Hash())
	if err != nil {
		t.Fatal(err)
	}
	var trafficWin, attrWin *landscape.DateRange
	for i := range core.DomainWindows {
		w := &core.DomainWindows[i]
		if w.Domain == "advertising" && w.FactFamily == "ADVERTISING.TRAFFIC" {
			trafficWin = w.Current
		}
		if w.Domain == "advertising" && w.FactFamily == "ADVERTISING.ATTRIBUTION" {
			attrWin = w.Current
		}
	}
	if trafficWin == nil || attrWin == nil {
		t.Fatalf("missing advertising windows traffic=%v attr=%v", trafficWin, attrWin)
	}
	wantTrafficTo := canonicaljson.Date(trafficTo)
	wantAttrTo := canonicaljson.Date(attrTo)
	if trafficWin.To != wantTrafficTo {
		t.Fatalf("traffic window to=%v want %v", trafficWin.To, wantTrafficTo)
	}
	if attrWin.To != wantAttrTo {
		t.Fatalf("attribution window to=%v want %v", attrWin.To, wantAttrTo)
	}
	if trafficWin.From == attrWin.From && trafficWin.To == attrWin.To {
		t.Fatal("traffic and attribution windows must be allowed to differ")
	}
	for _, c := range core.Campaigns {
		if c.Performance != nil && c.Performance.Traffic != nil && c.Performance.Traffic.CurrentPeriod != nil {
			if c.Performance.Traffic.CurrentPeriod.To != wantTrafficTo {
				t.Errorf("campaign %d traffic period to=%v want %v", c.Identity.CampaignID, c.Performance.Traffic.CurrentPeriod.To, wantTrafficTo)
			}
		}
		if c.Performance != nil && c.Performance.Attribution != nil && c.Performance.Attribution.CurrentPeriod != nil {
			if c.Performance.Attribution.CurrentPeriod.To != wantAttrTo {
				t.Errorf("campaign %d attribution period to=%v want %v", c.Identity.CampaignID, c.Performance.Attribution.CurrentPeriod.To, wantAttrTo)
			}
		}
	}
	t.Logf("Traffic current: %v → %v", trafficWin.From, trafficWin.To)
	t.Logf("Attribution current: %v → %v", attrWin.From, attrWin.To)
}
