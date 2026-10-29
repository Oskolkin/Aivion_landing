package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

const (
	CanonicalStockAnalyticsPath = "/v1/analytics/stocks"
	CanonicalPriceListPath      = "/v5/product/info/prices"
	StockAnalyticsBatchLimit    = 100
	PriceListPageLimit          = 1000
)

// CanonicalStockRow intentionally preserves provider quantity names. Until
// live semantic acceptance, valid_stock_count is not called sellable stock.
type CanonicalStockRow struct {
	SKU                             FlexibleInt64 `json:"sku"`
	WarehouseID                     FlexibleInt64 `json:"warehouse_id"`
	WarehouseName                   string        `json:"warehouse_name"`
	ClusterID                       FlexibleInt64 `json:"cluster_id"`
	ClusterName                     string        `json:"cluster_name"`
	MacrolocalClusterID             FlexibleInt64 `json:"macrolocal_cluster_id"`
	ValidStockCount                 *int64        `json:"valid_stock_count"`
	AvailableStockCount             *int64        `json:"available_stock_count"`
	RequestedStockCount             *int64        `json:"requested_stock_count"`
	TransitStockCount               *int64        `json:"transit_stock_count"`
	WaitingDocsStockCount           *int64        `json:"waiting_docs_stock_count"`
	ReturnFromCustomerStockCount    *int64        `json:"return_from_customer_stock_count"`
	ReturnToSellerStockCount        *int64        `json:"return_to_seller_stock_count"`
	ExpiringStockCount              *int64        `json:"expiring_stock_count"`
	OtherStockCount                 *int64        `json:"other_stock_count"`
	StockDefectStockCount           *int64        `json:"stock_defect_stock_count"`
	TransitDefectStockCount         *int64        `json:"transit_defect_stock_count"`
	ProviderADS                     json.Number   `json:"ads"`
	ProviderADSCluster              json.Number   `json:"ads_cluster"`
	ProviderIDC                     json.Number   `json:"idc"`
	ProviderIDCCluster              json.Number   `json:"idc_cluster"`
	ProviderTurnoverGrade           string        `json:"turnover_grade"`
	ProviderTurnoverGradeCluster    string        `json:"turnover_grade_cluster"`
	ProviderDaysWithoutSales        *int64        `json:"days_without_sales"`
	ProviderDaysWithoutSalesCluster *int64        `json:"days_without_sales_cluster"`
}

type CanonicalStockRequest struct {
	SKUs []string `json:"skus"`
}

type canonicalStockEnvelope struct {
	Items []CanonicalStockRow `json:"items"`
}

type PriceFilter struct {
	OfferIDs   []string `json:"offer_id,omitempty"`
	ProductIDs []int64  `json:"product_id,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
}

type CanonicalPriceRequest struct {
	Cursor string      `json:"cursor,omitempty"`
	Filter PriceFilter `json:"filter"`
	Limit  int         `json:"limit"`
}

type ProviderPriceIndex struct {
	MinimalPrice         string      `json:"minimal_price"`
	MinimalPriceCurrency string      `json:"minimal_price_currency"`
	PriceIndexValue      json.Number `json:"price_index_value"`
}

type CanonicalPriceRow struct {
	ProductID    FlexibleInt64 `json:"product_id"`
	SKU          FlexibleInt64 `json:"sku"`
	OfferID      string        `json:"offer_id"`
	CurrencyCode string        `json:"currency_code"`
	Price        struct {
		Price                string `json:"price"`
		OldPrice             string `json:"old_price"`
		MarketingSellerPrice string `json:"marketing_seller_price"`
		MinPrice             string `json:"min_price"`
		RetailPrice          string `json:"retail_price"`
		MarketingPrice       string `json:"marketing_price"`
		CurrencyCode         string `json:"currency_code"`
		VAT                  string `json:"vat"`
		AutoActionEnabled    bool   `json:"auto_action_enabled"`
	} `json:"price"`
	PriceIndexes struct {
		Color            string             `json:"color_index"`
		External         ProviderPriceIndex `json:"external_index_data"`
		Ozon             ProviderPriceIndex `json:"ozon_index_data"`
		SelfMarketplaces ProviderPriceIndex `json:"self_marketplaces_index_data"`
	} `json:"price_indexes"`
	Commissions      json.RawMessage `json:"commissions"`
	MarketingActions json.RawMessage `json:"marketing_actions"`
	Raw              json.RawMessage `json:"-"`
}

func (r *CanonicalPriceRow) UnmarshalJSON(data []byte) error {
	type wire CanonicalPriceRow
	var v wire
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = CanonicalPriceRow(v)
	r.Raw = append(r.Raw[:0], data...)
	if r.CurrencyCode == "" {
		r.CurrencyCode = r.Price.CurrencyCode
	}
	return nil
}

type CanonicalPriceResult struct {
	Items  []CanonicalPriceRow `json:"items"`
	Cursor string              `json:"cursor"`
	Total  int64               `json:"total"`
}

func (c *Client) analyticsStocksBearer(ctx context.Context, token string, req CanonicalStockRequest) (*TypedResponse[[]CanonicalStockRow], error) {
	var envelope canonicalStockEnvelope
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, CanonicalStockAnalyticsPath, req, &envelope)
	if err != nil {
		return nil, err
	}
	if len(envelope.Items) == 0 {
		_ = json.Unmarshal(raw.Body, &envelope.Items)
	}
	return &TypedResponse[[]CanonicalStockRow]{Raw: raw.Body, Data: envelope.Items, Meta: raw.Meta}, nil
}

func (c *Client) pricesBearer(ctx context.Context, token string, req CanonicalPriceRequest) (*TypedResponse[CanonicalPriceResult], error) {
	var result CanonicalPriceResult
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, CanonicalPriceListPath, req, &result)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[CanonicalPriceResult]{Raw: raw.Body, Data: result, Meta: raw.Meta}, nil
}
