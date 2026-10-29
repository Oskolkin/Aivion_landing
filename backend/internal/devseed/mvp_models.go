package devseed

import "time"

// MVPDefaultSeed is the default deterministic RNG seed (--seed).
const MVPDefaultSeed int64 = 20260514

// MVPSeedOptions configures the dev MVP seed.
type MVPSeedOptions struct {
	SellerAccountID int64

	Email    string
	Password string
	// ResetPassword when true overwrites password_hash for existing demo/admin users found by email (or seller owner when --seller-account-id is set).
	ResetPassword bool
	SellerName    string

	AdminEmail    string
	WithAdminUser bool

	AnchorDate     time.Time
	Days           int
	ProductsTarget int
	Seed           int64
	Reset          bool
	ValidateOnly   bool
	// ValidateAlertGeneration runs the production alerts engine against --seller-account-id (no seed writes).
	ValidateAlertGeneration bool
	// ValidateRecommendationGeneration runs the production recommendation generator (OpenAI) for --seller-account-id.
	ValidateRecommendationGeneration bool
	// ValidateDerived checks DB only: alerts, recommendations, chat, admin artifacts after manual MVP testing (no writes, no OpenAI).
	ValidateDerived bool
	// ValidatePricingLegacy when true enforces deprecated price_risk/overstock pricing scenario counts in validate-only.
	ValidatePricingLegacy bool

	// EncryptionKey must be exactly 32 bytes (same as ENCRYPTION_KEY for API/worker) so Ozon credentials decrypt in UI.
	EncryptionKey string
}

// MVPSeedResult is returned for CLI summary (counts reflect DB after metrics rebuild).
type MVPSeedResult struct {
	DemoUserEmail string
	AdminEmail    string
	// DemoPasswordUpdated is true when this run wrote a bcrypt hash for the demo/seller-owner user (new user or reset).
	DemoPasswordUpdated bool
	// AdminPasswordUpdated is true when admin user was requested and this run wrote a bcrypt hash for that user.
	AdminPasswordUpdated bool
	SellerAccountID      int64

	ProductsCount        int64
	OrdersCount          int64
	SalesCount           int64
	StocksCount          int64
	AdCampaignsCount     int64
	AdMetricRows         int64
	PricingRulesCount    int64
	EffectiveConstraints int64
	SyncJobsCount        int64
	ImportJobsCount      int64
	AccountMetricRows    int64
	SKUMetricRows        int64

	AnchorDate              string
	GeneratedDateStart      string
	GeneratedDateEnd        string
	ExpectedDays            int
	AccountMetricMinDate    string
	AccountMetricMaxDate    string
	SKUMetricDistinctDates  int64
	FutureBusinessRowsFound int64

	ProductAnalyticsRows          int64
	ProductAnalyticsDistinctDates int64
	FunnelAvailableForAnchor      bool
	ProductsWithPrimaryImage      int64
}

// ProductAnalyticsSeedStats is returned from SeedMVPProductAnalytics.
type ProductAnalyticsSeedStats struct {
	AccountRows              int
	DistinctDates            int
	ProductScopeRows         int
	HasAnchorRow             bool
	AnchorSourceStatus       string
	FunnelAvailableForAnchor bool
}

// CommerceSeedStats records how many commerce rows were written (for import_job counters).
type CommerceSeedStats struct {
	Products int
	Orders   int
	Sales    int
	Stocks   int
}

// AdsSeedStats records ad layer inserts.
type AdsSeedStats struct {
	Campaigns  int
	MetricRows int
	SKULinks   int
}

// PricingSeedStats records pricing inserts.
type PricingSeedStats struct {
	Rules     int
	Effective int
}
