package performance

import (
	"encoding/json"
	"time"
)

type TypedResponse[T any] struct {
	Raw  []byte
	Data T
	Meta ResponseMeta
}

type Campaign struct {
	CampaignExternalID       int64
	CampaignName             string
	CampaignType             string
	PlacementType            string
	PlacementJSON            json.RawMessage
	Status                   string
	PaymentType              string
	BudgetAmount             string
	BudgetDaily              string
	BudgetWeekly             string
	BudgetRawMicro           int64
	DailyBudgetRawMicro      int64
	WeeklyBudgetRawMicro     int64
	FromDate                 time.Time
	ToDate                   time.Time
	AutostopStatus           string
	IsAutocreated            bool
	IsAutocreatedPresent     bool
	ProductAutopilotStrategy string
	ProductCampaignMode      string
	Autopilot                json.RawMessage
	CreatedAt                time.Time
	UpdatedAt                time.Time
	Raw                      json.RawMessage
}

type CampaignDailyMetric struct {
	CampaignExternalID int64
	CampaignName       string
	MetricDate         time.Time
	Impressions        int64
	ImpressionsPresent bool
	Clicks             int64
	ClicksPresent      bool
	Spend              string
	BonusSpend         string
	OrdersCount        int64
	OrdersCountPresent bool
	Revenue            string
	ProviderDRR        string
	Raw                json.RawMessage
}

type CampaignProductDailyMetric struct {
	CampaignExternalID int64
	SKU                int64
	MetricDate         time.Time
	Impressions        int64
	ImpressionsPresent bool
	Clicks             int64
	ClicksPresent      bool
	Spend              string
	BonusSpend         string
	OrdersCount        int64
	OrdersCountPresent bool
	Revenue            string
	ProviderDRR        string
	AttributionKind    string
	Raw                json.RawMessage
}

type CampaignProductStatisticsResult struct {
	Items []CampaignProductDailyMetric
}

type ExpenseDailyMetric struct {
	MetricDate   time.Time
	PaidSpend    string
	BonusSpend   string
	CurrencyHint string
	Raw          json.RawMessage
}

type ExpenseStatisticsResult struct {
	Items []ExpenseDailyMetric
}

type ReportStatus struct {
	UUID  string
	State string
	Raw   json.RawMessage
}

type ReportSubmitResult struct {
	UUID string
	Raw  json.RawMessage
}

type CampaignPromotedProduct struct {
	CampaignExternalID int64
	OzonProductID      int64
	SKU                int64
	Title              string
	IsActive           bool
	Status             string
	BidRawMicro        int64
	BidAmount          string
	BidUnitAssumption  string
	Raw                json.RawMessage
}

type CampaignObject struct {
	CampaignExternalID int64
	ObjectExternalID   string
	ObjectType         string
	Raw                json.RawMessage
}

type CampaignObjectsResult struct {
	Items []CampaignObject
}

type ListCampaignsRequest struct {
	CampaignIDs   []int64
	AdvObjectType string
	State         string
	Page          int64
	PageSize      int64
}

type ListCampaignsResult struct {
	Items []Campaign
}

type CampaignStatisticsRequest struct {
	CampaignIDs []int64
	DateFrom    string
	DateTo      string
}

type CampaignStatisticsResult struct {
	Items []CampaignDailyMetric
}

type CampaignProductsRequest struct {
	CampaignID int64
	Page       int64
	PageSize   int64
}

type CampaignProductsResult struct {
	Items []CampaignPromotedProduct
}

type CampaignListFilter = ListCampaignsRequest

type SearchPromoProductsRequest struct {
	Page     int64
	PageSize int64
}
