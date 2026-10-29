package ozon

import (
	"context"
	"net/http"
)

type SellerInfoResponse struct {
	Company      map[string]any `json:"company"`
	Ratings      map[string]any `json:"ratings"`
	Subscription map[string]any `json:"subscription"`
}

type RolesResponse struct {
	ExpiresAt string `json:"expires_at"`
	Roles     []struct {
		Name    string   `json:"name"`
		Methods []string `json:"methods"`
	} `json:"roles"`
}

type SellerLogisticsInfoResponse struct {
	AvailableSchemas     []string `json:"available_schemas"`
	OzonLogisticsEnabled bool     `json:"ozon_logistics_enabled"`
}

func (c *Client) GetSellerInfo(
	ctx context.Context,
	clientID string,
	apiKey string,
) (*TypedResponse[SellerInfoResponse], error) {
	var parsed SellerInfoResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, "/v1/seller/info", map[string]any{}, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[SellerInfoResponse]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) GetRoles(
	ctx context.Context,
	clientID string,
	apiKey string,
) (*TypedResponse[RolesResponse], error) {
	var parsed RolesResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, "/v1/roles", map[string]any{}, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[RolesResponse]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) GetSellerLogisticsInfo(
	ctx context.Context,
	clientID string,
	apiKey string,
) (*TypedResponse[SellerLogisticsInfoResponse], error) {
	var parsed SellerLogisticsInfoResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, "/v1/seller/ozon-logistics/info", map[string]any{}, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[SellerLogisticsInfoResponse]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}
