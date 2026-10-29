package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

func main() {
	var (
		sellerAccountID                  int64
		email                            string
		password                         string
		adminEmail                       string
		sellerName                       string
		anchorDateRaw                    string
		days                             int
		products                         int
		seed                             int64
		reset                            bool
		resetPassword                    bool
		withAdmin                        bool
		validateOnly                     bool
		validateAlertGeneration          bool
		validateRecommendationGeneration bool
		validateDerived                  bool
		resetDerived                     bool
		validatePricingLegacy            bool
		asOfDateRaw                      string
	)

	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "existing seller_account_id (optional)")
	flag.StringVar(&email, "email", "demo@example.com", "demo user email when seller-account-id is not set")
	flag.StringVar(&password, "password", "password123", "password for newly created users (bcrypt)")
	flag.StringVar(&adminEmail, "admin-email", "admin@example.com", "admin user email when --with-admin-user=true")
	flag.StringVar(&sellerName, "seller-name", "Demo Ozon Seller", "seller account display name on create")
	flag.StringVar(&anchorDateRaw, "anchor-date", "today", `anchor calendar date in UTC: "today" or YYYY-MM-DD`)
	flag.IntVar(&days, "days", 90, "history depth in days for orders/sales (relative to anchor-date)")
	flag.IntVar(&products, "products", 80, "number of synthetic products")
	flag.Int64Var(&seed, "seed", devseed.MVPDefaultSeed, "deterministic RNG seed base")
	flag.BoolVar(&reset, "reset", true, "delete existing MVP-shaped data for this seller before insert")
	flag.BoolVar(&resetPassword, "reset-password", true, "when demo/admin (or seller owner) already exists, overwrite password_hash with bcrypt of --password")
	flag.BoolVar(&withAdmin, "with-admin-user", true, "create/find admin user (separate from demo seller)")
	flag.BoolVar(&validateOnly, "validate-only", false, "validate options, DB ping, and seeded data for --seller-account-id (no writes)")
	flag.BoolVar(&validateAlertGeneration, "validate-alert-generation", false, "run production alerts engine for --seller-account-id and assert non-empty groups (no seed writes)")
	flag.BoolVar(&validateRecommendationGeneration, "validate-recommendation-generation", false, "run production recommendation generator (OpenAI) for --seller-account-id; requires open alerts")
	flag.BoolVar(&validateDerived, "validate-derived", false, "read-only: verify alerts, recommendations, chat, and admin artifacts exist after manual MVP testing (--seller-account-id required)")
	flag.BoolVar(&validatePricingLegacy, "validate-pricing-legacy", false, "with --validate-only: enforce deprecated price_risk/overstock pricing scenario counts (not required for active MVP)")
	flag.BoolVar(&resetDerived, "reset-derived", false, "with --validate-alert-generation: delete this seller's alerts and alert_runs before running the engine")
	flag.StringVar(&asOfDateRaw, "as-of-date", "", `optional for --validate-alert-generation / --validate-recommendation-generation: YYYY-MM-DD (default: MAX(daily_account_metrics.metric_date) for seller)`)
	flag.Parse()

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	anchor, err := devseed.ParseMVPAnchorDate(anchorDateRaw, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "anchor-date: %v\n", err)
		os.Exit(2)
	}

	opts := devseed.MVPSeedOptions{
		SellerAccountID:                  sellerAccountID,
		Email:                            email,
		Password:                         password,
		ResetPassword:                    resetPassword,
		SellerName:                       sellerName,
		AdminEmail:                       adminEmail,
		WithAdminUser:                    withAdmin,
		AnchorDate:                       anchor,
		Days:                             days,
		ProductsTarget:                   products,
		Seed:                             seed,
		Reset:                            reset,
		ValidateOnly:                     validateOnly,
		ValidateAlertGeneration:          validateAlertGeneration,
		ValidateRecommendationGeneration: validateRecommendationGeneration,
		ValidateDerived:                  validateDerived,
		ValidatePricingLegacy:            validatePricingLegacy,
		EncryptionKey:                    cfg.Auth.EncryptionKey,
	}

	if err := devseed.ValidateMVPSeedOptions(opts); err != nil {
		fmt.Fprintf(os.Stderr, "invalid options: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer postgres.Close()

	var asOfPtr *time.Time
	if validateAlertGeneration || validateRecommendationGeneration {
		var errParse error
		asOfPtr, errParse = parseOptionalAsOfDate(asOfDateRaw)
		if errParse != nil {
			fmt.Fprintf(os.Stderr, "as-of-date: expected YYYY-MM-DD: %v\n", errParse)
			os.Exit(2)
		}
	}

	if opts.ValidateAlertGeneration {
		if err := devseed.ValidateMVPDatabase(ctx, postgres.Pool, opts); err != nil {
			fmt.Fprintf(os.Stderr, "validate database: %v\n", err)
			os.Exit(1)
		}
		ok, err := devseed.ValidateMVPAlertGeneration(ctx, postgres.Pool, devseed.MVPAlertGenerationValidateOptions{
			SellerAccountID: opts.SellerAccountID,
			ResetDerived:    resetDerived,
			AsOfDate:        asOfPtr,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "validate-alert-generation: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println("validate-alert-generation: OK")
		return
	}

	if opts.ValidateRecommendationGeneration {
		if err := devseed.ValidateMVPDatabase(ctx, postgres.Pool, opts); err != nil {
			fmt.Fprintf(os.Stderr, "validate database: %v\n", err)
			os.Exit(1)
		}
		svc, err := devseed.NewRecommendationServiceForMVPDev(postgres.Pool, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "recommendation service: %v\n", err)
			os.Exit(1)
		}
		ok, err := devseed.ValidateMVPRecommendationGeneration(ctx, postgres.Pool, cfg, svc, devseed.MVPRecommendationGenerationValidateOptions{
			SellerAccountID: opts.SellerAccountID,
			AsOfDate:        asOfPtr,
		})
		if errors.Is(err, devseed.ErrRecommendationValidateOpenAIMissing) {
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "validate-recommendation-generation: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println("validate-recommendation-generation: OK")
		return
	}

	if opts.ValidateDerived {
		if err := devseed.ValidateMVPDatabase(ctx, postgres.Pool, opts); err != nil {
			fmt.Fprintf(os.Stderr, "validate database: %v\n", err)
			os.Exit(1)
		}
		rows, ok := devseed.ValidateMVPDerivedSeller(ctx, postgres.Pool, opts.SellerAccountID)
		devseed.PrintMVPValidationTable(rows)
		fmt.Println()
		if !ok {
			fmt.Println("Hints: exercise the full product flow for this seller (Alerts → Recommendations → Chat), use /app/admin where applicable (e.g. view traces / raw AI), then re-run --validate-derived.")
			os.Exit(1)
		}
		fmt.Println("validate-derived: OK (manual MVP results present)")
		return
	}

	if validateOnly {
		if err := devseed.ValidateMVPDatabase(ctx, postgres.Pool, opts); err != nil {
			fmt.Fprintf(os.Stderr, "validate database: %v\n", err)
			os.Exit(1)
		}
		if opts.SellerAccountID <= 0 {
			fmt.Fprintf(os.Stderr, "validate-only: pass --seller-account-id to verify MVP seeded data for that seller\n")
			os.Exit(2)
		}
		rows, ok := devseed.ValidateMVPSeededSeller(ctx, postgres.Pool, opts)
		devseed.PrintMVPValidationTable(rows)
		fmt.Println()
		devseed.PrintMVPValidationHints()
		if !ok {
			os.Exit(1)
		}
		recon, reconErr := devseed.ReconcileSellerAnalysisData(ctx, postgres.Pool, devseed.ReconciliationOptions{
			SellerAccountID: opts.SellerAccountID,
			MetricDate:      opts.AnchorDate,
			HistoryDays:     30,
		})
		if reconErr != nil {
			fmt.Fprintf(os.Stderr, "reconciliation: %v\n", reconErr)
			os.Exit(1)
		}
		if !recon.Valid || len(recon.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "reconciliation failed: errors=%d warnings=%d\n", len(recon.Errors), len(recon.Warnings))
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println("validate-only: OK (seed-level checks + reconciliation passed)")
		return
	}

	if err := devseed.AssertDevSeedAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	result, err := devseed.SeedMVP(ctx, postgres.Pool, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}

	printSummary(password, result)
	printNextSteps()
}

func parseOptionalAsOfDate(raw string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	return &t, nil
}

func printSummary(password string, r *devseed.MVPSeedResult) {
	fmt.Println("--- MVP dev seed summary ---")
	fmt.Printf("Demo login:     %s\n", r.DemoUserEmail)
	fmt.Printf("Demo password:  %s\n", password)
	if r.AdminEmail != "" {
		fmt.Printf("Admin login:    %s\n", r.AdminEmail)
		fmt.Printf("Admin password: %s\n", password)
		fmt.Printf("Password updated: demo=%t admin=%t\n", r.DemoPasswordUpdated, r.AdminPasswordUpdated)
	} else {
		fmt.Println("Admin login:    (skipped)")
		fmt.Printf("Password updated: demo=%t admin=n/a\n", r.DemoPasswordUpdated)
	}
	fmt.Printf("seller_account_id: %d\n", r.SellerAccountID)
	fmt.Printf("products:              %d\n", r.ProductsCount)
	fmt.Printf("orders:                %d\n", r.OrdersCount)
	fmt.Printf("sales:                 %d\n", r.SalesCount)
	fmt.Printf("stocks:                %d\n", r.StocksCount)
	fmt.Printf("ad_campaigns:          %d\n", r.AdCampaignsCount)
	fmt.Printf("ad_metric_rows:        %d\n", r.AdMetricRows)
	fmt.Printf("pricing_rules:         %d\n", r.PricingRulesCount)
	fmt.Printf("effective_constraints: %d\n", r.EffectiveConstraints)
	fmt.Printf("sync_jobs:             %d\n", r.SyncJobsCount)
	fmt.Printf("import_jobs:           %d\n", r.ImportJobsCount)
	fmt.Printf("anchor_date:             %s\n", r.AnchorDate)
	fmt.Printf("generated_date_start:    %s\n", r.GeneratedDateStart)
	fmt.Printf("generated_date_end:      %s\n", r.GeneratedDateEnd)
	fmt.Printf("expected_days:           %d\n", r.ExpectedDays)
	fmt.Printf("account_metric_rows:     %d\n", r.AccountMetricRows)
	if r.AccountMetricMinDate != "" || r.AccountMetricMaxDate != "" {
		fmt.Printf("account_metric_dates:    %s .. %s\n", r.AccountMetricMinDate, r.AccountMetricMaxDate)
	}
	fmt.Printf("sku_metric_rows:         %d\n", r.SKUMetricRows)
	fmt.Printf("sku_metric_distinct_dates: %d\n", r.SKUMetricDistinctDates)
	fmt.Printf("future_business_rows_found: %d\n", r.FutureBusinessRowsFound)
	fmt.Printf("product_analytics_rows:    %d\n", r.ProductAnalyticsRows)
	fmt.Printf("product_analytics_dates:   %d\n", r.ProductAnalyticsDistinctDates)
	fmt.Printf("funnel_available_for_anchor: %t\n", r.FunnelAvailableForAnchor)
	fmt.Printf("products_with_primary_image: %d\n", r.ProductsWithPrimaryImage)
}

func printNextSteps() {
	fmt.Println()
	fmt.Println("--- Next manual MVP checks ---")
	fmt.Println("1. Open /app (MVP Test Home)")
	fmt.Println("2. Open /app/dashboard")
	fmt.Println("3. Open /app/alerts and run “Run alerts”")
	fmt.Println("4. Open /app/recommendations and run “Generate recommendations”")
	fmt.Println("5. Open /app/chat and ask real questions (requires OpenAI API key in app config)")
	fmt.Println("6. Open /app/admin and inspect clients, sync/import jobs, logs/traces as applicable")
}
