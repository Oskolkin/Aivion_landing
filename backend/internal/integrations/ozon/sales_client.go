package ozon

import (
	"context"
	"encoding/json"
	"net/http"
)

type ListSalesRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type SalesPosting struct {
	PostingNumber string      `json:"posting_number"`
	OrderID       json.Number `json:"order_id"`
}

type SalesItem struct {
	SKU      int64  `json:"sku"`
	Name     string `json:"name"`
	Quantity int32  `json:"quantity"`
}

type SalesOperation struct {
	OperationID   json.Number  `json:"operation_id"`
	OperationType string       `json:"operation_type"`
	OperationDate string       `json:"operation_date"`
	Amount        string       `json:"amount"`
	CurrencyCode  string       `json:"currency_code"`
	Posting       SalesPosting `json:"posting"`
	Items         []SalesItem  `json:"items"`
}

type ListSalesResult struct {
	Operations []SalesOperation `json:"operations"`
}

type listSalesRequestEnvelope struct {
	Filter struct {
		Date struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"date"`
	} `json:"filter"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type listSalesEnvelope struct {
	Result ListSalesResult `json:"result"`
}

func (c *Client) ListSales(
	ctx context.Context,
	clientID string,
	apiKey string,
	req ListSalesRequest,
) (*TypedResponse[ListSalesResult], error) {
	var parsed listSalesEnvelope

	payload := listSalesRequestEnvelope{
		Page:     req.Page,
		PageSize: req.PageSize,
	}
	payload.Filter.Date.From = req.From
	payload.Filter.Date.To = req.To

	rawResp, err := c.doJSON(
		ctx,
		clientID,
		apiKey,
		http.MethodPost,
		"/v3/finance/transaction/list",
		payload,
		&parsed,
	)
	if err != nil {
		return nil, err
	}

	return &TypedResponse[ListSalesResult]{
		Raw:  rawResp.Body,
		Data: parsed.Result,
		Meta: rawResp.Meta,
	}, nil
}
