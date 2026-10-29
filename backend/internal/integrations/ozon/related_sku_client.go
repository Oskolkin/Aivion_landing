package ozon

import (
	"context"
	"net/http"
)

type RelatedSKURequest struct {
	SKU []int64 `json:"sku"`
}

type RelatedSKUItem struct {
	SKU            int64  `json:"sku"`
	ProductID      int64  `json:"product_id"`
	DeliverySchema string `json:"delivery_schema"`
	Availability   string `json:"availability"`
	DeletedAt      string `json:"deleted_at"`
	RequestedSKU   int64  `json:"requested_sku"`
}

type RelatedSKUErrorItem struct {
	SKU     int64  `json:"sku"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type RelatedSKUsResponse struct {
	Items  []RelatedSKUItem      `json:"items"`
	Errors []RelatedSKUErrorItem `json:"errors"`
}

// GetRelatedSKUs fetches FBO/FBS/legacy SKU relations for up to 200 SKUs per request.
func (c *Client) GetRelatedSKUs(
	ctx context.Context,
	clientID string,
	apiKey string,
	skus []int64,
) (*TypedResponse[RelatedSKUsResponse], error) {
	var parsed RelatedSKUsResponse
	rawResp, err := c.doJSON(ctx, clientID, apiKey, http.MethodPost, "/v1/product/related-sku/get", RelatedSKURequest{
		SKU: skus,
	}, &parsed)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[RelatedSKUsResponse]{
		Raw:  rawResp.Body,
		Data: parsed,
		Meta: rawResp.Meta,
	}, nil
}

// GetSellerAPIRoles is the canonical name for POST /v1/roles.
func (c *Client) GetSellerAPIRoles(
	ctx context.Context,
	clientID string,
	apiKey string,
) (*TypedResponse[RolesResponse], error) {
	return c.GetRoles(ctx, clientID, apiKey)
}
