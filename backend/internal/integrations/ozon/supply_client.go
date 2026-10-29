package ozon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	SupplyOrderListPath    = "/v3/supply-order/list"
	SupplyOrderGetPath     = "/v3/supply-order/get"
	SupplyOrderDetailsPath = "/v1/supply-order/details"
	SupplyOrderBundlePath  = "/v1/supply-order/bundle"
	SupplyActSummaryPath   = "/v1/supply-order/act/summary/get"
	SupplyActProductPath   = "/v1/supply-order/act/product/get"
	SupplyBundlePageLimit  = 100
)

// ProviderID preserves provider identities whether the wire uses JSON strings
// or numbers. It is never converted to a canonical order or supply identity.
type ProviderID string

func (v *ProviderID) UnmarshalJSON(data []byte) error {
	var s string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*v = ProviderID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*v = ProviderID(n.String())
	return nil
}

type SupplyOrderListRequest struct {
	Filter map[string]any `json:"filter,omitempty"`
	LastID ProviderID     `json:"last_id,omitempty"`
	Limit  int            `json:"limit,omitempty"`
}
type SupplyOrderRequest struct {
	OrderID ProviderID `json:"order_id"`
}
type SupplyBundleRequest struct {
	SupplyID ProviderID `json:"supply_id"`
	LastID   ProviderID `json:"last_id,omitempty"`
	Limit    int        `json:"limit,omitempty"`
}
type SupplyActRequest struct {
	SupplyID ProviderID `json:"supply_id"`
	ActID    ProviderID `json:"act_id,omitempty"`
	LastID   ProviderID `json:"last_id,omitempty"`
	Limit    int        `json:"limit,omitempty"`
}

type SupplyOrder struct {
	OrderID            ProviderID      `json:"order_id"`
	State              string          `json:"state"`
	DropoffWarehouseID FlexibleInt64   `json:"dropoff_warehouse_id"`
	TimeslotFrom       string          `json:"timeslot_from"`
	TimeslotTo         string          `json:"timeslot_to"`
	Supplies           []Supply        `json:"supplies"`
	Raw                json.RawMessage `json:"-"`
}

func (v *SupplyOrder) UnmarshalJSON(data []byte) error {
	type wire SupplyOrder
	var x wire
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	*v = SupplyOrder(x)
	v.Raw = append(v.Raw[:0], data...)
	return nil
}

type Supply struct {
	SupplyID           ProviderID      `json:"supply_id"`
	OrderID            ProviderID      `json:"order_id"`
	State              string          `json:"state"`
	StorageWarehouseID FlexibleInt64   `json:"storage_warehouse_id"`
	Raw                json.RawMessage `json:"-"`
}

func (v *Supply) UnmarshalJSON(data []byte) error {
	type wire Supply
	var x wire
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	*v = Supply(x)
	v.Raw = append(v.Raw[:0], data...)
	return nil
}

type SupplyBundleItem struct {
	ID        ProviderID      `json:"id"`
	SKU       FlexibleInt64   `json:"sku"`
	ProductID FlexibleInt64   `json:"product_id"`
	OfferID   string          `json:"offer_id"`
	Quantity  *int64          `json:"quantity"`
	Raw       json.RawMessage `json:"-"`
}

func (v *SupplyBundleItem) UnmarshalJSON(data []byte) error {
	type wire SupplyBundleItem
	var x wire
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	*v = SupplyBundleItem(x)
	v.Raw = append(v.Raw[:0], data...)
	return nil
}

type SupplyBundleResult struct {
	BundleID ProviderID         `json:"bundle_id"`
	Items    []SupplyBundleItem `json:"items"`
	HasNext  bool               `json:"has_next"`
	LastID   ProviderID         `json:"last_id"`
	Total    int64              `json:"total"`
}
type SupplyOrderListResult struct {
	Orders     []SupplyOrder `json:"orders"`
	OrderIDs   []ProviderID  `json:"order_ids"`
	HasNext    bool          `json:"has_next"`
	LastID     ProviderID    `json:"last_id"`
	Total      int64         `json:"total"`
	TotalCount int64         `json:"total_count"`
}

func (r SupplyOrderListResult) Rows() []SupplyOrder {
	if len(r.Orders) > 0 {
		return r.Orders
	}
	rows := make([]SupplyOrder, 0, len(r.OrderIDs))
	for _, id := range r.OrderIDs {
		rows = append(rows, SupplyOrder{OrderID: id})
	}
	return rows
}

type SupplyActSummary struct {
	ActID        ProviderID      `json:"act_id"`
	SupplyID     ProviderID      `json:"supply_id"`
	State        string          `json:"state"`
	TotalAmount  json.Number     `json:"total_amount"`
	CurrencyCode string          `json:"currency_code"`
	Raw          json.RawMessage `json:"-"`
}

func (v *SupplyActSummary) UnmarshalJSON(data []byte) error {
	type wire SupplyActSummary
	var x wire
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	*v = SupplyActSummary(x)
	v.Raw = append(v.Raw[:0], data...)
	return nil
}

// SupplyActItem retains independent provider facts. Accepted/shortage/surplus are
// never derived from each other; ProductID is never treated as SKU.
type SupplyActItem struct {
	SKU          FlexibleInt64
	OfferID      string
	Accepted     *int64
	Shortage     *int64
	Surplus      *int64
	Amount       json.Number
	CurrencyCode string
	Raw          json.RawMessage
}

func decodeOptionalInt64(raw json.RawMessage) (*int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var n FlexibleInt64
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, false
	}
	v := int64(n)
	return &v, true
}

func (v *SupplyActItem) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	v.Raw = append(v.Raw[:0], data...)
	if raw, ok := obj["sku"]; ok {
		_ = json.Unmarshal(raw, &v.SKU)
	} else if raw, ok := obj["ozon_sku"]; ok {
		_ = json.Unmarshal(raw, &v.SKU)
	}
	if raw, ok := obj["offer_id"]; ok {
		_ = json.Unmarshal(raw, &v.OfferID)
	}
	acceptedSet := false
	for _, key := range []string{"accepted", "accepted_units", "accepted_quantity", "quantity_accepted"} {
		if raw, ok := obj[key]; ok {
			if n, ok := decodeOptionalInt64(raw); ok {
				v.Accepted = n
			}
			acceptedSet = true
			break
		}
	}
	if !acceptedSet {
		if raw, ok := obj["quantity"]; ok {
			if n, ok := decodeOptionalInt64(raw); ok {
				v.Accepted = n
			}
		}
	}
	for _, key := range []string{"shortage", "shortage_units", "shortage_qty", "shortage_quantity"} {
		if raw, ok := obj[key]; ok {
			if n, ok := decodeOptionalInt64(raw); ok {
				v.Shortage = n
			}
			break
		}
	}
	for _, key := range []string{"surplus", "surplus_units", "surplus_qty", "surplus_quantity"} {
		if raw, ok := obj[key]; ok {
			if n, ok := decodeOptionalInt64(raw); ok {
				v.Surplus = n
			}
			break
		}
	}
	if raw, ok := obj["amount"]; ok {
		_ = json.Unmarshal(raw, &v.Amount)
	} else if raw, ok := obj["total_amount"]; ok {
		_ = json.Unmarshal(raw, &v.Amount)
	}
	if raw, ok := obj["currency_code"]; ok {
		_ = json.Unmarshal(raw, &v.CurrencyCode)
	}
	return nil
}

type SupplyActProductResult struct {
	Items   []SupplyActItem `json:"items"`
	HasNext bool            `json:"has_next"`
	LastID  ProviderID      `json:"last_id"`
	Total   int64           `json:"total"`
}

func (c *Client) supplyPost(ctx context.Context, token, path string, request any, out any) (*RawResponse, error) {
	return c.doJSONBearer(ctx, token, http.MethodPost, path, request, out)
}

func decodeSupplyResult[T any](raw *RawResponse, target *T) (*TypedResponse[T], error) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw.Body, &envelope); err != nil {
		return nil, err
	}
	body := raw.Body
	if len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		body = envelope.Result
	}
	if err := json.Unmarshal(body, target); err != nil {
		return nil, err
	}
	return &TypedResponse[T]{Raw: raw.Body, Data: *target, Meta: raw.Meta}, nil
}

func (c *Client) supplyOrderListBearer(ctx context.Context, token string, request SupplyOrderListRequest) (*TypedResponse[SupplyOrderListResult], error) {
	var wire any
	raw, err := c.supplyPost(ctx, token, SupplyOrderListPath, request, &wire)
	if err != nil {
		return nil, err
	}
	var out SupplyOrderListResult
	response, err := decodeSupplyResult(raw, &out)
	if err == nil && len(out.Rows()) == 0 {
		var rows []SupplyOrder
		if listErr := json.Unmarshal(raw.Body, &rows); listErr == nil {
			response.Data.Orders = rows
		}
	}
	return response, err
}
func (c *Client) supplyOrderBearer(ctx context.Context, token, path string, request SupplyOrderRequest) (*TypedResponse[SupplyOrder], error) {
	var wire any
	raw, err := c.supplyPost(ctx, token, path, request, &wire)
	if err != nil {
		return nil, err
	}
	var out SupplyOrder
	return decodeSupplyResult(raw, &out)
}
func (c *Client) supplyBundleBearer(ctx context.Context, token string, request SupplyBundleRequest) (*TypedResponse[SupplyBundleResult], error) {
	var wire any
	raw, err := c.supplyPost(ctx, token, SupplyOrderBundlePath, request, &wire)
	if err != nil {
		return nil, err
	}
	var out SupplyBundleResult
	return decodeSupplyResult(raw, &out)
}

func (c *Client) supplyActSummaryBearer(ctx context.Context, token string, request SupplyActRequest) (*TypedResponse[SupplyActSummary], error) {
	if err := requireProviderID(request.SupplyID, "supply_id"); err != nil {
		return nil, err
	}
	var wire any
	raw, err := c.supplyPost(ctx, token, SupplyActSummaryPath, request, &wire)
	if err != nil {
		return nil, err
	}
	var out SupplyActSummary
	response, err := decodeSupplyResult(raw, &out)
	if err != nil {
		return nil, err
	}
	if out.SupplyID == "" {
		out.SupplyID = request.SupplyID
		response.Data.SupplyID = request.SupplyID
	}
	return response, nil
}

func (c *Client) supplyActProductBearer(ctx context.Context, token string, request SupplyActRequest) (*TypedResponse[SupplyActProductResult], error) {
	if err := requireProviderID(request.SupplyID, "supply_id"); err != nil {
		return nil, err
	}
	var wire any
	raw, err := c.supplyPost(ctx, token, SupplyActProductPath, request, &wire)
	if err != nil {
		return nil, err
	}
	var out SupplyActProductResult
	response, err := decodeSupplyResult(raw, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		var alt struct {
			Products []SupplyActItem `json:"products"`
			Items    []SupplyActItem `json:"items"`
			HasNext  bool            `json:"has_next"`
			LastID   ProviderID      `json:"last_id"`
			Total    int64           `json:"total"`
		}
		if decodeErr := json.Unmarshal(response.Raw, &alt); decodeErr == nil {
			if len(alt.Items) > 0 {
				response.Data.Items = alt.Items
			} else if len(alt.Products) > 0 {
				response.Data.Items = alt.Products
			}
			response.Data.HasNext = alt.HasNext
			response.Data.LastID = alt.LastID
			response.Data.Total = alt.Total
		}
	}
	return response, nil
}

func providerID(v int64) ProviderID {
	if v <= 0 {
		return ""
	}
	return ProviderID(strconv.FormatInt(v, 10))
}
func requireProviderID(v ProviderID, kind string) error {
	if v == "" || v == "0" {
		return fmt.Errorf("%s is required", kind)
	}
	return nil
}
