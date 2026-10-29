package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
)

func openReadiness(args []string, name string) (*db.Postgres, int64, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	seller := fs.Int64("seller-id", 0, "seller_account_id")
	if e := fs.Parse(args); e != nil || *seller <= 0 {
		return nil, 0, fmt.Errorf("seller-id is required")
	}
	_ = config.LoadEnvFiles()
	cfg, e := config.Load()
	if e != nil {
		return nil, 0, e
	}
	pg, e := db.New(context.Background(), cfg.DB.URL)
	return pg, *seller, e
}
func runEvaluateReadiness(args []string) int {
	pg, seller, e := openReadiness(args, "evaluate-readiness")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer pg.Close()
	result, e := readiness.NewService(pg.Pool, readiness.RealClock{}, readiness.ProductionSemanticRegistry{}).Evaluate(context.Background(), seller)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	contextHash := ""
	if result.Context != nil {
		contextHash = result.Context.ContextHash()
	}
	printSafeJSON(map[string]any{"seller_account_id": result.SellerAccountID, "cutoff_at": result.CutoffAt, "policy_versions": result.Policies, "watermarks": result.Watermarks, "capabilities": result.Capabilities, "dq_issues": result.Issues, "candidate_dates": result.Candidates, "analysis_date": result.AnalysisDate, "context_hash": contextHash, "dq_run_id": result.DQRunID, "input_hash": result.InputHash, "result_hash": result.ResultHash})
	return 0
}
func runFactualSmoke(args []string) int {
	pg, seller, e := openReadiness(args, "factual-smoke")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer pg.Close()
	secret := strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_HMAC_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_SIGNING_KEY"))
	}
	signer, e := factualapi.NewTokenSigner([]byte(secret))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	ready, e := readiness.NewService(pg.Pool, readiness.RealClock{}, readiness.ProductionSemanticRegistry{}).Evaluate(context.Background(), seller)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	if ready.Context == nil {
		printSafeJSON(map[string]any{"seller_account_id": seller, "cutoff_at": ready.CutoffAt, "analysis_date": nil, "candidates": ready.Candidates, "issues": ready.Issues, "capabilities": ready.Capabilities})
		return 0
	}
	api := factual.NewService(pg.Pool, signer, 50, 500)
	ctx := context.Background()
	current, e := api.GetAccountDemand(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	previous, e := api.GetAccountDemand(ctx, *ready.Context, factualapi.PeriodPrevious7D, nil)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	skus, e := api.ListSKUDemand(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: 3})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	categories, e := api.ListCategoryDemand(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: 3})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	postings, e := api.ListPostings(ctx, *ready.Context, factualapi.PostingFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	events, e := api.ListLifecycleEvents(ctx, *ready.Context, factualapi.LifecycleEventFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	returns, e := api.ListReturns(ctx, *ready.Context, factualapi.ReturnFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	stocks, e := api.ListSKUStock(ctx, *ready.Context, factualapi.StockFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	prices, e := api.ListSKUPrices(ctx, *ready.Context, factualapi.PriceFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	campaigns, e := api.ListCampaigns(ctx, *ready.Context, factualapi.CampaignFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	adSummary, e := api.GetAdvertisingAccountSummary(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	spendShare, e := api.GetAdvertisingSpendShare(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	smoke := map[string]any{"context": map[string]any{"seller_account_id": seller, "analysis_date": ready.Context.AnalysisDate(), "cutoff_at": ready.Context.CutoffAt(), "context_hash": ready.Context.ContextHash(), "mode": ready.Context.Mode(), "policies": ready.Context.PolicyVersions()}, "account_current_7d": current, "account_previous_7d": previous, "sample_sku_demand": skus, "sample_category_demand": categories, "sample_posting": postings, "sample_events": events, "sample_returns": returns, "sample_stock": stocks, "sample_price": prices, "sample_campaign": campaigns, "advertising_account_summary": adSummary, "advertising_spend_share": spendShare, "capabilities": ready.Capabilities}
	smoke["funnel_search_impressions_current_7d"] = softFactualProbe(func() (any, error) {
		return api.GetAccountFunnelPeriod(ctx, *ready.Context, "SEARCH_IMPRESSIONS", factualapi.PeriodCurrent7D, nil)
	})
	smoke["funnel_search_impressions_previous_7d"] = softFactualProbe(func() (any, error) {
		return api.GetAccountFunnelPeriod(ctx, *ready.Context, "SEARCH_IMPRESSIONS", factualapi.PeriodPrevious7D, nil)
	})
	smoke["finance_account_summary"] = softFactualProbe(func() (any, error) {
		return api.GetFinanceAccountSummary(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil)
	})
	smoke["search_sku_summary_page_1"] = softFactualProbe(func() (any, error) {
		return api.ListSKUSearchSummary(ctx, *ready.Context, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: 3})
	})
	smoke["promotions_page_1"] = softFactualProbe(func() (any, error) {
		return api.ListPromotions(ctx, *ready.Context, factualapi.PromotionFilter{}, factualapi.PageRequest{PageSize: 3})
	})
	smoke["account_health_summary"] = softFactualProbe(func() (any, error) {
		return api.GetAccountHealthSummary(ctx, *ready.Context)
	})
	smoke["current_ratings"] = softFactualProbe(func() (any, error) {
		return api.GetCurrentRatings(ctx, *ready.Context)
	})
	smoke["supplies_page_1"] = softFactualProbe(func() (any, error) {
		return api.ListSupplies(ctx, *ready.Context, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 3})
	})
	if len(stocks.Items) > 0 {
		if coverage, ce := api.GetStockCoverage(ctx, *ready.Context, stocks.Items[0].CanonicalSKUID); ce == nil {
			smoke["sample_stock_coverage"] = coverage
		}
		smoke["sample_sku_inbound"] = softFactualProbe(func() (any, error) {
			return api.GetSKUInbound(ctx, *ready.Context, stocks.Items[0].CanonicalSKUID)
		})
	} else {
		smoke["sample_sku_inbound"] = map[string]any{"skipped": "no SKU available"}
	}
	if len(prices.Items) > 0 {
		if index, ie := api.GetSKUPriceIndex(ctx, *ready.Context, prices.Items[0].CanonicalSKUID); ie == nil {
			smoke["sample_price_index"] = index
		}
	}
	if len(campaigns.Items) > 0 {
		if drr, de := api.GetCampaignDRR(ctx, *ready.Context, campaigns.Items[0].ID, factualapi.PeriodCurrent7D, nil); de == nil {
			smoke["sample_campaign_drr"] = drr
		}
	}
	printSafeJSON(smoke)
	return 0
}
func printSafeJSON(v any) { raw, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(raw)) }

func softFactualProbe(probe func() (any, error)) any {
	value, err := probe()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return value
}
