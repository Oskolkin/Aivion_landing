package ozon

import (
	"context"
	"net/http"
)

type ListStocksRequest struct {
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	WarehouseID int64  `json:"warehouse_id,omitempty"`
}

type StockItem struct {
	ProductID   int64  `json:"product_id"`
	OfferID     string `json:"offer_id"`
	SKU         int64  `json:"sku"`
	WarehouseID int64  `json:"warehouse_id"`
	Present     int32  `json:"present"`
	Reserved    int32  `json:"reserved"`
	FreeStock   int32  `json:"free_stock"`
	UpdatedAt   string `json:"updated_at"`
}

type ListStocksResult struct {
	Stocks  []StockItem `json:"stocks"`
	HasNext bool        `json:"has_next"`
	Cursor  string      `json:"cursor"`
}

func (c *Client) ListStocks(
	ctx context.Context,
	clientID string,
	apiKey string,
	req ListStocksRequest,
) (*TypedResponse[ListStocksResult], error) {
	var parsed ListStocksResult

	rawResp, err := c.doJSON(
		ctx,
		clientID,
		apiKey,
		http.MethodPost,
		"/v1/product/info/warehouse/stocks",
		req,
		&parsed,
	)
	if err != nil {
		return nil, err
	}

	return &TypedResponse[ListStocksResult]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}
