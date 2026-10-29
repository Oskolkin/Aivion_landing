package ozon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	CanonicalFBOListPath   = "/v3/posting/fbo/list"
	CanonicalFBODetailPath = "/v2/posting/fbo/get"
	CanonicalFBSListPath   = "/v4/posting/fbs/list"
	CanonicalFBSDetailPath = "/v3/posting/fbs/get"
	CanonicalReturnsPath   = "/v1/returns/list"
)

type Scalar string

func (s *Scalar) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*s = ""
		return nil
	}
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		*s = Scalar(text)
		return nil
	}
	*s = Scalar(string(raw))
	return nil
}

type FlexibleInt64 int64

func (v *FlexibleInt64) UnmarshalJSON(raw []byte) error {
	text := strings.Trim(string(raw), `"`)
	if text == "" || text == "null" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*v = FlexibleInt64(parsed)
	return nil
}

type PostingListRequest struct {
	Dir    string `json:"dir,omitempty"`
	Filter struct {
		Since                 string `json:"since"`
		To                    string `json:"to"`
		LastChangedStatusDate *struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"last_changed_status_date,omitempty"`
	} `json:"filter"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor,omitempty"`
	With   struct {
		AnalyticsData bool `json:"analytics_data"`
		FinancialData bool `json:"financial_data"`
		Barcodes      bool `json:"barcodes"`
	} `json:"with"`
}

type PostingDetailRequest struct {
	PostingNumber string `json:"posting_number"`
	With          struct {
		AnalyticsData bool `json:"analytics_data"`
		FinancialData bool `json:"financial_data"`
		Barcodes      bool `json:"barcodes"`
	} `json:"with"`
}
type PostingProduct struct {
	SKU          FlexibleInt64 `json:"sku"`
	ProductID    FlexibleInt64 `json:"product_id"`
	OfferID      string        `json:"offer_id"`
	Name         string        `json:"name"`
	Quantity     Scalar        `json:"quantity"`
	Price        Scalar        `json:"price"`
	CurrencyCode string        `json:"currency_code"`
}
type PostingCancellation struct {
	CancelReasonID        FlexibleInt64 `json:"cancel_reason_id"`
	CancelReason          string        `json:"cancel_reason"`
	CancellationType      string        `json:"cancellation_type"`
	CancellationInitiator string        `json:"cancellation_initiator"`
	CancelledAfterShip    bool          `json:"cancelled_after_ship"`
}
type PostingDeliveryMethod struct {
	Schema      string        `json:"schema"`
	ID          FlexibleInt64 `json:"id"`
	WarehouseID FlexibleInt64 `json:"warehouse_id"`
}
type CanonicalPosting struct {
	OrderID             FlexibleInt64         `json:"order_id"`
	OrderNumber         string                `json:"order_number"`
	PostingNumber       string                `json:"posting_number"`
	Status              string                `json:"status"`
	Substatus           string                `json:"substatus"`
	CreatedAt           string                `json:"created_at"`
	InProcessAt         string                `json:"in_process_at"`
	DeliveringDate      string                `json:"delivering_date"`
	ShipmentDate        string                `json:"shipment_date"`
	ParentPostingNumber string                `json:"parent_posting_number"`
	Products            []PostingProduct      `json:"products"`
	Cancellation        PostingCancellation   `json:"cancellation"`
	DeliveryMethod      PostingDeliveryMethod `json:"delivery_method"`
	WarehouseID         FlexibleInt64         `json:"warehouse_id"`
}
type PostingListResult struct {
	Postings []CanonicalPosting `json:"postings"`
	HasNext  bool               `json:"has_next"`
	Cursor   string             `json:"cursor"`
}

type ReturnWindow struct {
	TimeFrom string `json:"time_from"`
	TimeTo   string `json:"time_to"`
}
type ReturnsListRequest struct {
	Filter struct {
		VisualStatusChangeMoment *ReturnWindow `json:"visual_status_change_moment,omitempty"`
	} `json:"filter"`
	Limit  int   `json:"limit"`
	LastID int64 `json:"last_id"`
}
type CanonicalReturn struct {
	ID                 FlexibleInt64 `json:"id"`
	SourceID           FlexibleInt64 `json:"source_id"`
	OrderID            FlexibleInt64 `json:"order_id"`
	PostingNumber      string        `json:"posting_number"`
	SKU                FlexibleInt64 `json:"sku"`
	OfferID            string        `json:"offer_id"`
	ProductName        string        `json:"product_name"`
	Quantity           Scalar        `json:"quantity"`
	Price              Scalar        `json:"price"`
	CurrencyCode       string        `json:"currency_code"`
	Type               string        `json:"type"`
	Schema             string        `json:"schema"`
	ReturnSchema       string        `json:"return_schema"`
	ReturnReasonName   string        `json:"return_reason_name"`
	CompensationStatus string        `json:"compensation_status"`
	Logistic           struct {
		ReturnDate                      string `json:"return_date"`
		TechnicalReturnMoment           string `json:"technical_return_moment"`
		FinalMoment                     string `json:"final_moment"`
		CancelledWithCompensationMoment string `json:"cancelled_with_compensation_moment"`
	} `json:"logistic"`
	Visual struct {
		Status        string `json:"status"`
		StatusSysName string `json:"status_sys_name"`
		ChangeMoment  string `json:"change_moment"`
	} `json:"visual"`
}
type ReturnsListResult struct {
	Returns []CanonicalReturn `json:"returns"`
	HasNext bool              `json:"has_next"`
	LastID  FlexibleInt64     `json:"last_id"`
}

func ParsePostingListResponse(raw []byte) (PostingListResult, error) {
	var envelope struct {
		Result   json.RawMessage    `json:"result"`
		Postings []CanonicalPosting `json:"postings"`
		HasNext  bool               `json:"has_next"`
		Cursor   string             `json:"cursor"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return PostingListResult{}, err
	}
	if len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		var result PostingListResult
		if err := json.Unmarshal(envelope.Result, &result); err != nil {
			return result, err
		}
		return result, nil
	}
	return PostingListResult{Postings: envelope.Postings, HasNext: envelope.HasNext, Cursor: envelope.Cursor}, nil
}
func ParsePostingDetailResponse(raw []byte) (CanonicalPosting, error) {
	var e struct {
		Result CanonicalPosting `json:"result"`
	}
	err := json.Unmarshal(raw, &e)
	return e.Result, err
}
func ParseReturnsListResponse(raw []byte) (ReturnsListResult, error) {
	var direct ReturnsListResult
	if err := json.Unmarshal(raw, &direct); err != nil {
		return direct, err
	}
	if direct.Returns != nil {
		return direct, nil
	}
	var e struct {
		Result ReturnsListResult `json:"result"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return direct, err
	}
	return e.Result, nil
}

func (c *Client) postingListBearer(ctx context.Context, token, path string, req PostingListRequest) (*TypedResponse[PostingListResult], error) {
	var rawEnvelope json.RawMessage
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, path, req, &rawEnvelope)
	if err != nil {
		return nil, err
	}
	result, err := ParsePostingListResponse(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("decode posting list: %w", err)
	}
	return &TypedResponse[PostingListResult]{Raw: raw.Body, Data: result, Meta: raw.Meta}, nil
}
func (c *Client) postingDetailBearer(ctx context.Context, token, path string, req PostingDetailRequest) (*TypedResponse[CanonicalPosting], error) {
	var rawEnvelope json.RawMessage
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, path, req, &rawEnvelope)
	if err != nil {
		return nil, err
	}
	result, err := ParsePostingDetailResponse(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("decode posting detail: %w", err)
	}
	return &TypedResponse[CanonicalPosting]{Raw: raw.Body, Data: result, Meta: raw.Meta}, nil
}
func (c *Client) returnsListBearer(ctx context.Context, token string, req ReturnsListRequest) (*TypedResponse[ReturnsListResult], error) {
	var rawEnvelope json.RawMessage
	raw, err := c.doJSONBearer(ctx, token, http.MethodPost, CanonicalReturnsPath, req, &rawEnvelope)
	if err != nil {
		return nil, err
	}
	result, err := ParseReturnsListResponse(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("decode returns list: %w", err)
	}
	return &TypedResponse[ReturnsListResult]{Raw: raw.Body, Data: result, Meta: raw.Meta}, nil
}
