package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ProductQueriesPath      = "/v1/analytics/product-queries"
	ProductQueryDetailsPath = "/v1/analytics/product-queries/details"
	SearchQueriesTopPath    = "/v1/search-queries/top"
	SearchQueriesTextPath   = "/v1/search-queries/text"

	// Verified Seller API limits (product-queries / details).
	ProductQueriesSKULimit      = 1000
	ProductQueryDetailsSKULimit = 1000
	ProductQueryDetailsLimitMax = 15
	ProductQueryDetailsPageMax  = 100
)

// ProductQueriesRequest is POST /v1/analytics/product-queries.
// Verified contract requires skus + date_from + page_size; sort_* are optional provider fields.
type ProductQueriesRequest struct {
	DateFrom string  `json:"date_from"`
	DateTo   string  `json:"date_to,omitempty"`
	SKUs     []int64 `json:"skus"`
	Page     int     `json:"page,omitempty"`
	PageSize int     `json:"page_size,omitempty"`
	SortBy   string  `json:"sort_by,omitempty"`
	SortDir  string  `json:"sort_dir,omitempty"`
}

// ProductQueryDetailsRequest is POST /v1/analytics/product-queries/details.
// Verified contract: skus (up to 1000), limit_by_sku (<=15 = max query rows PER SKU),
// page_size (max 100), date_from required; sort_by/sort_dir optional.
// JSON field is "skus" (not legacy "sku").
type ProductQueryDetailsRequest struct {
	DateFrom   string  `json:"date_from"`
	DateTo     string  `json:"date_to,omitempty"`
	SKUs       []int64 `json:"skus"`
	LimitBySKU int     `json:"limit_by_sku"`
	Page       int     `json:"page,omitempty"`
	PageSize   int     `json:"page_size,omitempty"`
	SortBy     string  `json:"sort_by,omitempty"`
	SortDir    string  `json:"sort_dir,omitempty"`
}

type MarketSearchRequest struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Query    string `json:"query,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// SearchQueryRow covers both SKU-summary rows (product-queries) and SKU×query
// detail rows (product-queries/details). Summary responses typically omit query.
type SearchQueryRow struct {
	Query             string
	SKU               FlexibleInt64
	ProviderSearchGMV ExactNumber
	Orders            ExactNumber
	UniqueSearchUsers ExactNumber
	UniqueViewUsers   ExactNumber
	Position          ExactNumber
	ViewConversion    ExactNumber
	UniqueUsers       ExactNumber // legacy alias when provider emits unique_users only
	Rank              int
	CategoryID        FlexibleInt64
	Raw               json.RawMessage
}

func (r *SearchQueryRow) UnmarshalJSON(data []byte) error {
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return err
	}
	r.Raw = append([]byte(nil), data...)
	r.Query = firstScalar(obj, "query", "query_text", "search_query", "text")
	r.ProviderSearchGMV = ExactNumber(firstScalar(obj, "provider_search_gmv", "gmv", "currency_gmv", "revenue"))
	r.Orders = ExactNumber(firstScalar(obj, "orders", "orders_count", "order_count"))
	r.UniqueSearchUsers = ExactNumber(firstScalar(obj, "unique_search_users", "unique_users", "users"))
	r.UniqueViewUsers = ExactNumber(firstScalar(obj, "unique_view_users", "view_users"))
	r.Position = ExactNumber(firstScalar(obj, "position", "avg_position"))
	r.ViewConversion = ExactNumber(firstScalar(obj, "view_conversion", "conversion", "conversion_view"))
	r.UniqueUsers = r.UniqueSearchUsers
	r.SKU = FlexibleInt64(parsePositive(firstScalar(obj, "sku", "ozon_sku")))
	r.CategoryID = FlexibleInt64(parsePositive(firstScalar(obj, "category_id")))
	fmt.Sscan(firstScalar(obj, "rank", "position"), &r.Rank)
	return nil
}

type SearchQueryResult struct {
	Items   []SearchQueryRow
	HasNext bool
}

func (r *SearchQueryResult) UnmarshalJSON(data []byte) error {
	var env struct {
		Items   []SearchQueryRow `json:"items"`
		Queries []SearchQueryRow `json:"queries"`
		Result  []SearchQueryRow `json:"result"`
		HasNext bool             `json:"has_next"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&env); err != nil {
		return err
	}
	r.Items, r.HasNext = env.Items, env.HasNext
	if len(r.Items) == 0 {
		r.Items = env.Queries
	}
	if len(r.Items) == 0 {
		r.Items = env.Result
	}
	return nil
}

func firstScalar(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := scalarString(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func parsePositive(raw string) int64 {
	var value int64
	fmt.Sscan(raw, &value)
	if value < 0 {
		return 0
	}
	return value
}

func (c *Client) searchBearer(ctx context.Context, token, path string, request any) (*TypedResponse[SearchQueryResult], error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal search request: %w", err)
	}
	raw, err := c.doBearerRequest(ctx, token, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var result SearchQueryResult
	decoder := json.NewDecoder(bytes.NewReader(raw.Body))
	decoder.UseNumber()
	if err = decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return &TypedResponse[SearchQueryResult]{Data: result, Raw: raw.Body, Meta: raw.Meta}, nil
}

func (c *Client) productQueriesBearer(ctx context.Context, token string, request ProductQueriesRequest) (*TypedResponse[SearchQueryResult], error) {
	return c.searchBearer(ctx, token, ProductQueriesPath, request)
}
func (c *Client) productQueryDetailsBearer(ctx context.Context, token string, request ProductQueryDetailsRequest) (*TypedResponse[SearchQueryResult], error) {
	return c.searchBearer(ctx, token, ProductQueryDetailsPath, request)
}
func (c *Client) marketSearchBearer(ctx context.Context, token, path string, request MarketSearchRequest) (*TypedResponse[SearchQueryResult], error) {
	return c.searchBearer(ctx, token, path, request)
}
