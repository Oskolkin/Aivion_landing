package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	FinanceAccrualTypesPath    = "/v1/finance/accrual/types"
	FinanceAccrualByDayPath    = "/v1/finance/accrual/by-day"
	FinanceAccrualPostingsPath = "/v1/finance/accrual/postings"
	FinanceBalancePath         = "/v1/finance/balance"
)

// ExactNumber retains the provider decimal token verbatim. It intentionally
// has no float conversion.
type ExactNumber string

func (n *ExactNumber) UnmarshalJSON(data []byte) error {
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		*n = ExactNumber(number.String())
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("finance number: %w", err)
	}
	*n = ExactNumber(text)
	return nil
}

type FinanceAccrualType struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	AccruedCategory  string `json:"accrued_category"`
	EconomicRole     string `json:"economic_role"`
	DefaultScope     string `json:"default_scope"`
	CrossDomainOwner string `json:"cross_domain_owner"`
}

type FinanceAccrualTypesResult struct {
	Items []FinanceAccrualType `json:"items"`
}

func (t *FinanceAccrualType) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	_ = json.Unmarshal(raw["name"], &t.Name)
	_ = json.Unmarshal(raw["description"], &t.Description)
	_ = json.Unmarshal(raw["accrued_category"], &t.AccruedCategory)
	_ = json.Unmarshal(raw["economic_role"], &t.EconomicRole)
	_ = json.Unmarshal(raw["default_scope"], &t.DefaultScope)
	_ = json.Unmarshal(raw["cross_domain_owner"], &t.CrossDomainOwner)
	var id any
	decoder := json.NewDecoder(bytes.NewReader(raw["id"]))
	decoder.UseNumber()
	if decoder.Decode(&id) == nil {
		t.ID = scalarString(id)
	}
	return nil
}

func (r *FinanceAccrualTypesResult) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Items        []FinanceAccrualType `json:"items"`
		AccrualTypes []FinanceAccrualType `json:"accrual_types"`
		Types        []FinanceAccrualType `json:"types"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		var direct []FinanceAccrualType
		if directErr := json.Unmarshal(data, &direct); directErr != nil {
			return err
		}
		r.Items = direct
		return nil
	}
	r.Items = envelope.Items
	if len(r.Items) == 0 {
		r.Items = envelope.AccrualTypes
	}
	if len(r.Items) == 0 {
		r.Items = envelope.Types
	}
	return nil
}

type FinanceAccrualRequest struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

func (r FinanceAccrualRequest) MarshalJSON() ([]byte, error) {
	if r.DateTo != "" && r.DateTo != r.DateFrom {
		return json.Marshal(map[string]any{"date": map[string]string{"from": r.DateFrom, "to": r.DateTo}, "last_id": r.Cursor})
	}
	return json.Marshal(map[string]any{"date": r.DateFrom, "last_id": r.Cursor})
}

type FinanceAccrualComponent struct {
	Path      string          `json:"path"`
	Amount    ExactNumber     `json:"amount"`
	TypeID    string          `json:"type_id"`
	OzonSKU   FlexibleInt64   `json:"sku"`
	ProductID FlexibleInt64   `json:"product_id"`
	OrderID   FlexibleInt64   `json:"order_id"`
	PostingID FlexibleInt64   `json:"posting_id"`
	Raw       json.RawMessage `json:"-"`
}

func (c *FinanceAccrualComponent) UnmarshalJSON(data []byte) error {
	type wire FinanceAccrualComponent
	var v wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&v); err != nil {
		return err
	}
	*c = FinanceAccrualComponent(v)
	c.Raw = append([]byte(nil), data...)
	return nil
}

type FinanceAccrualRow struct {
	BusinessDate    string                    `json:"date"`
	ProviderTypeID  string                    `json:"type_id"`
	AccruedCategory string                    `json:"accrued_category"`
	UnitNumber      string                    `json:"unit_number"`
	Amount          ExactNumber               `json:"amount"`
	CurrencyCode    string                    `json:"currency"`
	Components      []FinanceAccrualComponent `json:"components"`
	Raw             json.RawMessage           `json:"-"`
}

func (r *FinanceAccrualRow) UnmarshalJSON(data []byte) error {
	type wire FinanceAccrualRow
	var v wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&v); err != nil {
		return err
	}
	*r = FinanceAccrualRow(v)
	r.Raw = append([]byte(nil), data...)
	var object map[string]any
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return err
	}
	if r.ProviderTypeID == "" {
		r.ProviderTypeID = scalarString(object["type_id"])
	}
	if total, ok := object["total_amount"].(map[string]any); ok {
		if r.Amount == "" {
			r.Amount = ExactNumber(scalarString(total["amount"]))
		}
		if r.CurrencyCode == "" {
			r.CurrencyCode = scalarString(total["currency"])
		}
	}
	if len(r.Components) == 0 {
		walkFinanceComponents(object, "", 0, &r.Components)
	}
	return nil
}

type FinanceAccrualResult struct {
	Items      []FinanceAccrualRow `json:"items"`
	HasNext    bool                `json:"has_next"`
	NextCursor string              `json:"cursor"`
}

func (r *FinanceAccrualResult) UnmarshalJSON(data []byte) error {
	type wire FinanceAccrualResult
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*r = FinanceAccrualResult(value)
	var envelope struct {
		Accruals []FinanceAccrualRow `json:"accruals"`
		LastID   string              `json:"last_id"`
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return err
	}
	if len(r.Items) == 0 {
		r.Items = envelope.Accruals
	}
	if r.NextCursor == "" {
		r.NextCursor = envelope.LastID
	}
	r.HasNext = r.HasNext || r.NextCursor != ""
	return nil
}

type FinanceBalanceResult struct {
	Amount       ExactNumber `json:"amount"`
	CurrencyCode string      `json:"currency"`
}

func (c *Client) financeBearer(ctx context.Context, token, path string, request, result any) (*RawResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal finance request: %w", err)
	}
	raw, err := c.doBearerRequest(ctx, token, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw.Body))
	decoder.UseNumber()
	if err = decoder.Decode(result); err != nil {
		return nil, fmt.Errorf("decode finance response: %w", err)
	}
	return raw, nil
}

func (c *Client) financeAccrualTypesBearer(ctx context.Context, token string) (*TypedResponse[FinanceAccrualTypesResult], error) {
	var result FinanceAccrualTypesResult
	raw, err := c.financeBearer(ctx, token, FinanceAccrualTypesPath, struct{}{}, &result)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[FinanceAccrualTypesResult]{Data: result, Raw: raw.Body, Meta: raw.Meta}, nil
}

func (c *Client) financeAccrualBearer(ctx context.Context, token, path string, req FinanceAccrualRequest) (*TypedResponse[FinanceAccrualResult], error) {
	var result FinanceAccrualResult
	raw, err := c.financeBearer(ctx, token, path, req, &result)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[FinanceAccrualResult]{Data: result, Raw: raw.Body, Meta: raw.Meta}, nil
}

func (c *Client) financeBalanceBearer(ctx context.Context, token string) (*TypedResponse[FinanceBalanceResult], error) {
	var result FinanceBalanceResult
	raw, err := c.financeBearer(ctx, token, FinanceBalancePath, struct{}{}, &result)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[FinanceBalanceResult]{Data: result, Raw: raw.Body, Meta: raw.Meta}, nil
}

func scalarString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func walkFinanceComponents(value any, path string, inheritedSKU int64, out *[]FinanceAccrualComponent) {
	switch typed := value.(type) {
	case map[string]any:
		sku := inheritedSKU
		handledAccrued := false
		if raw, ok := typed["sku"]; ok {
			if parsed, err := strconv.ParseInt(scalarString(raw), 10, 64); err == nil {
				sku = parsed
			}
		}
		if path != "total_amount" {
			if amount, ok := typed["amount"]; ok {
				if amountText := scalarString(amount); amountText != "" {
					raw, _ := json.Marshal(typed)
					component := FinanceAccrualComponent{Path: path, Amount: ExactNumber(amountText), TypeID: scalarString(typed["type_id"]), Raw: raw}
					if sku > 0 {
						component.OzonSKU = FlexibleInt64(sku)
					}
					*out = append(*out, component)
				}
			} else if accrued, ok := typed["accrued"].(map[string]any); ok {
				if amountText := scalarString(accrued["amount"]); amountText != "" {
					raw, _ := json.Marshal(typed)
					component := FinanceAccrualComponent{Path: path + ".accrued", Amount: ExactNumber(amountText), TypeID: scalarString(typed["type_id"]), Raw: raw}
					if sku > 0 {
						component.OzonSKU = FlexibleInt64(sku)
					}
					*out = append(*out, component)
				}
				handledAccrued = true
			}
		}
		for key, child := range typed {
			if handledAccrued && key == "accrued" {
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			walkFinanceComponents(child, childPath, sku, out)
		}
	case []any:
		for i, child := range typed {
			walkFinanceComponents(child, fmt.Sprintf("%s[%d]", path, i), inheritedSKU, out)
		}
	}
}
