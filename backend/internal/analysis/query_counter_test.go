//go:build integration

package analysis_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync/atomic"
)

type queryCounter struct {
	n atomic.Int64
}

func (q *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	q.n.Add(1)
	return ctx
}

func (q *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (q *queryCounter) Count() int64 { return q.n.Load() }

func (q *queryCounter) Reset() { q.n.Store(0) }

func newCountedPool(t *testing.T) (*pgxpool.Pool, *queryCounter) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Fatal("set TEST_DATABASE_URL")
	}
	counter := &queryCounter{}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, counter
}

const (
	coreQueryCountCeiling500 = 100
	coreQueryShapeOverhead   = 50
	detailQueryCountCeiling  = 40
)

func prompt94MeasureBuildCore(t *testing.T, pool *pgxpool.Pool, counter *queryCounter, skuCount int) int64 {
	t.Helper()
	f := prompt94ScaleSeed(t, pool, skuCount)
	_, binding, sealed := prompt94SealFixture(t, f)
	api := prompt94FactualAPI(t, pool)
	builder := landscape.CoreBuilder{
		Source:   landscape.FactualAPISource{API: api},
		Profiles: landscape.DefaultCoreProfilesV5(),
	}
	counter.Reset()
	_, _, _, err := builder.BuildCore(context.Background(), sealed, binding.ID, binding.Hash, f.bundle.Hash())
	if err != nil {
		t.Fatalf("BuildCore(%d SKU): %v", skuCount, err)
	}
	return counter.Count()
}

func TestPrompt94CoreQueryCount500SKU(t *testing.T) {
	pool, counter := newCountedPool(t)
	n := prompt94MeasureBuildCore(t, pool, counter, 500)
	t.Logf("Core BuildCore query count (500 SKU) = %d", n)
	if n > coreQueryCountCeiling500 {
		t.Fatalf("query count %d exceeds ceiling %d for 500 SKU BuildCore", n, coreQueryCountCeiling500)
	}
}

func TestPrompt94CoreQueryShape2000SKU(t *testing.T) {
	pool, counter := newCountedPool(t)
	q500 := prompt94MeasureBuildCore(t, pool, counter, 500)
	counter.Reset()
	q2000 := prompt94MeasureBuildCore(t, pool, counter, 2000)
	t.Logf("Core BuildCore query counts: 500 SKU = %d, 2000 SKU = %d", q500, q2000)
	if q2000 > q500+coreQueryShapeOverhead {
		t.Fatalf("2000-SKU query count %d exceeds 500-SKU %d + overhead %d", q2000, q500, coreQueryShapeOverhead)
	}
	if q500 > 0 && float64(q2000)/float64(q500) >= 3 {
		t.Fatalf("query shape ratio %.2f >= 3 (500=%d 2000=%d)", float64(q2000)/float64(q500), q500, q2000)
	}
}

func TestPrompt94DetailQueryCountMaxBudget(t *testing.T) {
	pool, counter := newCountedPool(t)
	f := prompt94ScaleSeed(t, pool, 500)
	_, binding, sealed := prompt94SealFixture(t, f)
	api := prompt94FactualAPI(t, pool)
	coreBuilder := landscape.CoreBuilder{
		Source:   landscape.FactualAPISource{API: api},
		Profiles: landscape.DefaultCoreProfilesV5(),
	}
	core, _, coreHash, err := coreBuilder.BuildCore(context.Background(), sealed, binding.ID, binding.Hash, f.bundle.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if len(core.SKUs) < materiality.DiscoveryStandardBudget().MaxSKUs {
		t.Fatalf("expected >=%d eligible SKUs for max budget, got %d", materiality.DiscoveryStandardBudget().MaxSKUs, len(core.SKUs))
	}
	if len(core.Categories) < materiality.DiscoveryStandardBudget().MaxCategories {
		t.Fatalf("expected >=%d categories, got %d", materiality.DiscoveryStandardBudget().MaxCategories, len(core.Categories))
	}
	if len(core.Campaigns) < materiality.DiscoveryStandardBudget().MaxCampaigns {
		t.Fatalf("expected >=%d campaigns, got %d", materiality.DiscoveryStandardBudget().MaxCampaigns, len(core.Campaigns))
	}
	facts, err := materiality.CoreFactsFromLandscape(core)
	if err != nil {
		t.Fatal(err)
	}
	ranking := materiality.BuildMaterialityRanking(facts, materiality.CurrentPolicy(), coreHash)
	selection := materiality.SelectMaterialEntities(ranking, materiality.DiscoveryStandardBudget(), facts.Account)
	budget := materiality.DiscoveryStandardBudget()
	if got, want := len(selection.Categories), budget.MaxCategories; got != want {
		t.Fatalf("expected %d selected categories, got %d", want, got)
	}
	if got, want := len(selection.SKUs), budget.MaxSKUs; got != want {
		t.Fatalf("expected %d selected SKU, got %d", want, got)
	}
	if got, want := len(selection.Campaigns), budget.MaxCampaigns; got != want {
		t.Fatalf("expected %d selected campaigns, got %d", want, got)
	}
	t.Logf("selected_categories = %d", len(selection.Categories))
	t.Logf("selected_skus       = %d", len(selection.SKUs))
	t.Logf("selected_campaigns  = %d", len(selection.Campaigns))
	refs := materiality.SelectionEntityRefs(selection)
	detailBuilder := landscape.DetailBuilder{Source: landscape.FactualAPIDetailSource{API: api}}
	counter.Reset()
	_, _, _, err = detailBuilder.BuildDetail(context.Background(), sealed, landscape.DetailBuildInput{
		Core:          core,
		Selected:      refs,
		SelectionHash: selection.SelectionHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	n := counter.Count()
	t.Logf("Detail BuildDetail query count (15/40/15 budget) = %d", n)
	if n > detailQueryCountCeiling {
		t.Fatalf("detail query count %d exceeds ceiling %d", n, detailQueryCountCeiling)
	}
}

// Ensure query counter implements pgx tracer at compile time.
var _ interface {
	TraceQueryStart(context.Context, *pgx.Conn, pgx.TraceQueryStartData) context.Context
	TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData)
} = (*queryCounter)(nil)

func TestQueryCounterHelperSanity(t *testing.T) {
	pool, counter := newCountedPool(t)
	var n int
	if err := pool.QueryRow(context.Background(), fmt.Sprintf("SELECT %d", 1)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if counter.Count() < 1 {
		t.Fatal("expected tracer to count at least one query")
	}
	counter.Reset()
	if counter.Count() != 0 {
		t.Fatal("Reset did not zero counter")
	}
	_ = factualapi.CurrentPolicies()
}
