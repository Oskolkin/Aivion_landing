package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

type ProductListFilter struct {
	OfferID    []string `json:"offer_id,omitempty"`
	ProductID  []int64  `json:"product_id,omitempty"`
	SKUs       []int64  `json:"skus,omitempty"`
	Visibility string   `json:"visibility,omitempty"`
}

type ListProductsRequest struct {
	Filter ProductListFilter `json:"filter"`
	LastID string            `json:"last_id"`
	Limit  int               `json:"limit"`
}

type ProductItem struct {
	ProductID  int64  `json:"product_id"`
	ID         int64  `json:"-"`
	Name       string `json:"name"`
	OfferID    string `json:"offer_id"`
	SKU        int64  `json:"sku"`
	Status     string `json:"status"`
	State      string `json:"state"`
	IsArchived bool   `json:"is_archived"`
	Archived   bool   `json:"archived"`
	UpdatedAt  string `json:"updated_at"`
}

func (item *ProductItem) UnmarshalJSON(data []byte) error {
	type productItemWire ProductItem
	var decoded productItemWire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*item = ProductItem(decoded)
	item.ID = item.ProductID
	return nil
}

type ListProductsResult struct {
	Items  []ProductItem `json:"items"`
	LastID string        `json:"last_id"`
	Total  int64         `json:"total"`
}

type listProductsEnvelope struct {
	Result ListProductsResult `json:"result"`
}

type ProductInfoListRequest struct {
	OfferID   []string `json:"offer_id,omitempty"`
	ProductID []int64  `json:"product_id,omitempty"`
	SKU       []int64  `json:"sku,omitempty"`
}

func (c *Client) ListProducts(
	ctx context.Context,
	clientID string,
	apiKey string,
	req ListProductsRequest,
) (*TypedResponse[ListProductsResult], error) {
	var parsed listProductsEnvelope

	rawResp, err := c.doJSON(
		ctx,
		clientID,
		apiKey,
		http.MethodPost,
		"/v3/product/list",
		req,
		&parsed,
	)
	if err != nil {
		return nil, err
	}

	return &TypedResponse[ListProductsResult]{
		Raw:  rawResp.Body,
		Data: parsed.Result,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) ListProductsInfo(
	ctx context.Context,
	clientID string,
	apiKey string,
	req ProductInfoListRequest,
) (*TypedResponse[ProductInfoListResult], error) {
	var parsed ProductInfoListResult
	rawResp, err := c.doJSON(
		ctx,
		clientID,
		apiKey,
		http.MethodPost,
		"/v3/product/info/list",
		req,
		&parsed,
	)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ProductInfoListResult]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) listProductsBearer(ctx context.Context, accessToken string, req ListProductsRequest) (*TypedResponse[ListProductsResult], error) {
	var parsed listProductsEnvelope
	rawResp, err := c.doJSONBearer(ctx, accessToken, http.MethodPost, "/v3/product/list", req, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ListProductsResult]{Raw: rawResp.Body, Data: parsed.Result, Meta: rawResp.Meta}, nil
}

func (c *Client) listProductsInfoBearer(ctx context.Context, accessToken string, req ProductInfoListRequest) (*TypedResponse[ProductInfoListResult], error) {
	var parsed ProductInfoListResult
	rawResp, err := c.doJSONBearer(ctx, accessToken, http.MethodPost, "/v3/product/info/list", req, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ProductInfoListResult]{Raw: rawResp.Body, Data: parsed, Meta: rawResp.Meta}, nil
}
