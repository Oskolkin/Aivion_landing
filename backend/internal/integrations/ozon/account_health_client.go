package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	RatingSummaryPath        = "/v1/rating/summary"
	RatingHistoryPath        = "/v1/rating/history"
	FBSIndexInfoPath         = "/v1/rating/index/fbs/info"
	FBSPostingErrorListPath  = "/v1/rating/index/fbs/posting/list"
	FBSPostingErrorPageLimit = 100
)

type AccountHealthRequest struct {
	DateFrom string `json:"date_from,omitempty"`
	DateTo   string `json:"date_to,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// AccountHealthDocument intentionally retains the provider document shape.
// Rating schemas contain provider-defined keys, so normalization belongs in
// the versioned canonical mapping rather than a lossy transport DTO.
type AccountHealthDocument map[string]any

func (c *Client) accountHealthBearer(ctx context.Context, token, path string, request AccountHealthRequest) (*TypedResponse[AccountHealthDocument], error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal account health request: %w", err)
	}
	raw, err := c.doBearerRequest(ctx, token, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var document AccountHealthDocument
	decoder := json.NewDecoder(bytes.NewReader(raw.Body))
	decoder.UseNumber()
	if err = decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode account health response: %w", err)
	}
	return &TypedResponse[AccountHealthDocument]{Raw: raw.Body, Data: document, Meta: raw.Meta}, nil
}
