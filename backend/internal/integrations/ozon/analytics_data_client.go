package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

const analyticsDataPath = "/v1/analytics/data"

// Confirmed Ozon analytics metrics for product funnel (see Ozon Seller API analyticsMetric enum).
const (
	AnalyticsMetricHitsView     = "hits_view"
	AnalyticsMetricHitsViewPDP  = "hits_view_pdp"
	AnalyticsMetricHitsToCart   = "hits_tocart"
	AnalyticsMetricOrderedUnits = "ordered_units"
	// AnalyticsMetricDeliveredUnits — confirmed Ozon analyticsMetric enum: units successfully delivered
	// to customers (Premium / extended analytics). Not the same as buyout rate; label as
	// "Доставленные единицы", not "Выкупы".
	AnalyticsMetricDeliveredUnits = "delivered_units"
	AnalyticsMetricRevenue        = "revenue"
)

// AccountFunnelMetrics is the ordered set used for account-level funnel ingestion.
var AccountFunnelMetrics = []string{
	AnalyticsMetricHitsView,
	AnalyticsMetricHitsViewPDP,
	AnalyticsMetricHitsToCart,
	AnalyticsMetricOrderedUnits,
	AnalyticsMetricDeliveredUnits,
	AnalyticsMetricRevenue,
}

// AnalyticsDataSort describes sort order for analytics/data requests.
type AnalyticsDataSort struct {
	Key   string `json:"key"`
	Order string `json:"order"`
}

// AnalyticsDataRequest is the body for POST /v1/analytics/data.
type AnalyticsDataRequest struct {
	DateFrom  string              `json:"date_from"`
	DateTo    string              `json:"date_to"`
	Metrics   []string            `json:"metrics"`
	Dimension []string            `json:"dimension"`
	Filters   []json.RawMessage   `json:"filters,omitempty"`
	Sort      []AnalyticsDataSort `json:"sort,omitempty"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
}

type analyticsDataDimension struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type analyticsDataRow struct {
	Dimensions []analyticsDataDimension `json:"dimensions"`
	Metrics    []float64                `json:"metrics"`
}

type analyticsDataResult struct {
	Data   []analyticsDataRow `json:"data"`
	Totals []float64          `json:"totals"`
}

type analyticsDataResponse struct {
	Result analyticsDataResult `json:"result"`
}

// AnalyticsDataResponse is the parsed response from POST /v1/analytics/data.
type AnalyticsDataResponse struct {
	Data   []AnalyticsDataRow
	Totals []float64
	Raw    []byte
}

// AnalyticsDataRow is one grouped row from analytics/data.
type AnalyticsDataRow struct {
	Dimensions []AnalyticsDataDimension
	Metrics    []float64
}

// AnalyticsDataDimension is a dimension value in an analytics/data row.
type AnalyticsDataDimension struct {
	ID   string
	Name string
}

// FetchAnalyticsData calls POST /v1/analytics/data and parses the response.
func (c *Client) FetchAnalyticsData(
	ctx context.Context,
	clientID string,
	apiKey string,
	req AnalyticsDataRequest,
) (*AnalyticsDataResponse, error) {
	var parsed analyticsDataResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, analyticsDataPath, req, &parsed)
	if err != nil {
		return nil, err
	}

	out := &AnalyticsDataResponse{
		Totals: parsed.Result.Totals,
		Raw:    rawResp.Body,
	}
	for _, row := range parsed.Result.Data {
		dims := make([]AnalyticsDataDimension, 0, len(row.Dimensions))
		for _, d := range row.Dimensions {
			dims = append(dims, AnalyticsDataDimension{ID: d.ID, Name: d.Name})
		}
		out.Data = append(out.Data, AnalyticsDataRow{
			Dimensions: dims,
			Metrics:    row.Metrics,
		})
	}
	return out, nil
}

// ParseAnalyticsDataResponse parses raw JSON from analytics/data (for tests).
func ParseAnalyticsDataResponse(raw []byte) (*AnalyticsDataResponse, error) {
	var parsed analyticsDataResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := &AnalyticsDataResponse{
		Totals: parsed.Result.Totals,
		Raw:    raw,
	}
	for _, row := range parsed.Result.Data {
		dims := make([]AnalyticsDataDimension, 0, len(row.Dimensions))
		for _, d := range row.Dimensions {
			dims = append(dims, AnalyticsDataDimension{ID: d.ID, Name: d.Name})
		}
		out.Data = append(out.Data, AnalyticsDataRow{
			Dimensions: dims,
			Metrics:    row.Metrics,
		})
	}
	return out, nil
}

// MapAccountFunnelTotals maps totals array to funnel fields using AccountFunnelMetrics order.
func MapAccountFunnelTotals(totals []float64) (impressions, productViews, addToCart, orders, completed *int64, revenue *float64) {
	idx := func(i int) *int64 {
		if i >= len(totals) {
			return nil
		}
		v := int64(totals[i])
		return &v
	}
	if len(totals) > 0 {
		impressions = idx(0)
	}
	if len(totals) > 1 {
		productViews = idx(1)
	}
	if len(totals) > 2 {
		addToCart = idx(2)
	}
	if len(totals) > 3 {
		orders = idx(3)
	}
	if len(totals) > 4 {
		completed = idx(4)
	}
	if len(totals) > 5 {
		r := totals[5]
		revenue = &r
	}
	return
}
