package ozon

import (
	"context"
	"net/http"
)

type ListOrdersRequest struct {
	Dir    string `json:"dir,omitempty"`
	Filter struct {
		Since string `json:"since,omitempty"`
		To    string `json:"to,omitempty"`
	} `json:"filter"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
	With   struct {
		FinancialData bool `json:"financial_data,omitempty"`
	} `json:"with"`
}

type OrderFinancialProduct struct {
	Price        string `json:"price"`
	Quantity     int32  `json:"quantity"`
	CurrencyCode string `json:"currency_code"`
}

type OrderFinancialData struct {
	Products []OrderFinancialProduct `json:"products"`
}

type OrderItem struct {
	OrderID       int64              `json:"order_id"`
	PostingNumber string             `json:"posting_number"`
	Status        string             `json:"status"`
	CreatedAt     string             `json:"created_at"`
	InProcessAt   string             `json:"in_process_at"`
	FinancialData OrderFinancialData `json:"financial_data"`
}

type ListOrdersResult struct {
	Postings []OrderItem `json:"postings"`
	HasNext  bool        `json:"has_next"`
}

type listOrdersEnvelope struct {
	Result ListOrdersResult `json:"result"`
}

func (c *Client) ListOrders(
	ctx context.Context,
	clientID string,
	apiKey string,
	req ListOrdersRequest,
) (*TypedResponse[ListOrdersResult], error) {
	var parsed listOrdersEnvelope

	rawResp, err := c.doJSON(
		ctx,
		clientID,
		apiKey,
		http.MethodPost,
		"/v3/posting/fbs/list",
		req,
		&parsed,
	)
	if err != nil {
		return nil, err
	}

	return &TypedResponse[ListOrdersResult]{
		Raw:  rawResp.Body,
		Data: parsed.Result,
		Meta: rawResp.Meta,
	}, nil
}
