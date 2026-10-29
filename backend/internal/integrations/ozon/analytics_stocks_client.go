package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

const analyticsStocksPath = "/v1/analytics/stocks"

// AnalyticsStocksRequest is the body for POST /v1/analytics/stocks.
type AnalyticsStocksRequest struct {
	SKUs                 []string `json:"skus"`
	WarehouseIDs         []int64  `json:"warehouse_ids,omitempty"`
	ClusterIDs           []int64  `json:"cluster_ids,omitempty"`
	MacrolocalClusterIDs []int64  `json:"macrolocal_cluster_ids,omitempty"`
	TurnoverGrades       []string `json:"turnover_grades,omitempty"`
	ItemTags             []string `json:"item_tags,omitempty"`
}

// AnalyticsStockItem is one warehouse-level row from Ozon stock analytics.
type AnalyticsStockItem struct {
	SKU                          int64    `json:"sku"`
	OfferID                      string   `json:"offer_id"`
	Name                         string   `json:"name"`
	WarehouseID                  int64    `json:"warehouse_id"`
	WarehouseName                string   `json:"warehouse_name"`
	ClusterID                    int64    `json:"cluster_id"`
	ClusterName                  string   `json:"cluster_name"`
	MacrolocalClusterID          int64    `json:"macrolocal_cluster_id"`
	AvailableStockCount          int32    `json:"available_stock_count"`
	ExcessStockCount             int32    `json:"excess_stock_count"`
	ADS                          float64  `json:"ads"`
	ADSCluster                   float64  `json:"ads_cluster"`
	IDC                          float64  `json:"idc"`
	IDCCluster                   float64  `json:"idc_cluster"`
	TurnoverGrade                string   `json:"turnover_grade"`
	TurnoverGradeCluster         string   `json:"turnover_grade_cluster"`
	DaysWithoutSales             int32    `json:"days_without_sales"`
	DaysWithoutSalesCluster      int32    `json:"days_without_sales_cluster"`
	RequestedStockCount          int32    `json:"requested_stock_count"`
	TransitStockCount            int32    `json:"transit_stock_count"`
	ValidStockCount              int32    `json:"valid_stock_count"`
	WaitingDocsStockCount        int32    `json:"waiting_docs_stock_count"`
	ReturnFromCustomerStockCount int32    `json:"return_from_customer_stock_count"`
	ReturnToSellerStockCount     int32    `json:"return_to_seller_stock_count"`
	ExpiringStockCount           int32    `json:"expiring_stock_count"`
	OtherStockCount              int32    `json:"other_stock_count"`
	StockDefectStockCount        int32    `json:"stock_defect_stock_count"`
	TransitDefectStockCount      int32    `json:"transit_defect_stock_count"`
	ItemTags                     []string `json:"item_tags"`
}

type analyticsStocksResponse struct {
	Items []AnalyticsStockItem `json:"items"`
}

// ListAnalyticsStocks fetches FBO stock analytics for up to 100 SKUs per request.
func (c *Client) ListAnalyticsStocks(
	ctx context.Context,
	clientID string,
	apiKey string,
	req AnalyticsStocksRequest,
) (*TypedResponse[[]AnalyticsStockItem], error) {
	var parsed analyticsStocksResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, analyticsStocksPath, req, &parsed)
	if err != nil {
		return nil, err
	}
	if len(parsed.Items) == 0 {
		var direct []AnalyticsStockItem
		if err := json.Unmarshal(rawResp.Body, &direct); err == nil && len(direct) > 0 {
			parsed.Items = direct
		}
	}
	return &TypedResponse[[]AnalyticsStockItem]{
		Raw:  rawResp.Body,
		Data: parsed.Items,
		Meta: rawResp.Meta,
	}, nil
}

// GetAnalyticsStocks is an alias for ListAnalyticsStocks.
func (c *Client) GetAnalyticsStocks(
	ctx context.Context,
	clientID string,
	apiKey string,
	req AnalyticsStocksRequest,
) (*TypedResponse[[]AnalyticsStockItem], error) {
	return c.ListAnalyticsStocks(ctx, clientID, apiKey, req)
}
