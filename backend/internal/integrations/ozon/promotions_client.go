package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

const (
	ActionsListPath            = "/v1/actions"
	ActionCandidatesPath       = "/v1/actions/candidates"
	ActionProductsPath         = "/v1/actions/products"
	SellerActionsListPath      = "/v1/actions/seller/list"
	SellerActionCandidatesPath = "/v1/actions/seller/candidates"
	SellerActionProductsPath   = "/v1/actions/seller/products"
	PromotionProductsPageLimit = 100
)

// Promotion APIs below are deliberately read-only. No mutating action endpoint
// is represented by this client.
type PromotionAction struct {
	ID                         FlexibleInt64   `json:"id"`
	Title                      string          `json:"title"`
	DateStart                  string          `json:"date_start"`
	DateEnd                    string          `json:"date_end"`
	State                      string          `json:"state"`
	DiscountType               string          `json:"discount_type"`
	DiscountValue              json.Number     `json:"discount_value"`
	CurrencyCode               string          `json:"currency_code"`
	ParticipatingProductsCount int64           `json:"participating_products_count"`
	PotentialProductsCount     int64           `json:"potential_products_count"`
	Raw                        json.RawMessage `json:"-"`
}

func (a *PromotionAction) UnmarshalJSON(data []byte) error {
	type wire PromotionAction
	var v wire
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = PromotionAction(v)
	a.Raw = append(a.Raw[:0], data...)
	return nil
}

type PromotionProductRequest struct {
	ActionID int64 `json:"action_id"`
	Limit    int   `json:"limit"`
	LastID   int64 `json:"last_id,omitempty"`
}

type PromotionProduct struct {
	ID          FlexibleInt64   `json:"id"` // provider ProductID; never canonical SKU identity
	ProductID   FlexibleInt64   `json:"product_id"`
	SKU         FlexibleInt64   `json:"sku"`
	OfferID     string          `json:"offer_id"`
	ActionPrice json.Number     `json:"action_price"`
	Stock       *int64          `json:"stock"`
	Raw         json.RawMessage `json:"-"`
}

func (p *PromotionProduct) UnmarshalJSON(data []byte) error {
	type wire PromotionProduct
	var v wire
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = PromotionProduct(v)
	p.Raw = append(p.Raw[:0], data...)
	if int64(p.ProductID) == 0 {
		p.ProductID = p.ID
	}
	return nil
}

type PromotionProductsResult struct {
	Products []PromotionProduct `json:"products"`
	Items    []PromotionProduct `json:"items"`
	Total    int64              `json:"total"`
	LastID   int64              `json:"last_id"`
}

func (r PromotionProductsResult) Rows() []PromotionProduct {
	if len(r.Products) > 0 {
		return r.Products
	}
	return r.Items
}

type promotionActionsEnvelope struct {
	Result  []PromotionAction `json:"result"`
	Actions []PromotionAction `json:"actions"`
	Items   []PromotionAction `json:"items"`
}

func (c *Client) promotionActionsBearer(ctx context.Context, token, path string) (*TypedResponse[[]PromotionAction], error) {
	var envelope promotionActionsEnvelope
	raw, err := c.doBearerRequest(ctx, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw.Body, &envelope); err != nil {
		return nil, err
	}
	rows := envelope.Result
	if len(rows) == 0 {
		rows = envelope.Actions
	}
	if len(rows) == 0 {
		rows = envelope.Items
	}
	return &TypedResponse[[]PromotionAction]{Raw: raw.Body, Data: rows, Meta: raw.Meta}, nil
}

func (c *Client) promotionProductsBearer(ctx context.Context, token, path string, request PromotionProductRequest) (*TypedResponse[PromotionProductsResult], error) {
	var envelope struct {
		Result PromotionProductsResult `json:"result"`
		PromotionProductsResult
	}
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, path, request, &envelope)
	if err != nil {
		return nil, err
	}
	result := envelope.Result
	if len(result.Rows()) == 0 && result.Total == 0 && result.LastID == 0 {
		result = envelope.PromotionProductsResult
	}
	return &TypedResponse[PromotionProductsResult]{Raw: raw.Body, Data: result, Meta: raw.Meta}, nil
}
