package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CanonicalAnalyticsResponse deliberately preserves exact JSON numbers. The
// legacy float64 adapter remains available only to legacy ingestion.
type CanonicalAnalyticsResponse struct {
	Data []CanonicalAnalyticsRow
	Raw  []byte
	Meta ResponseMeta
}

type CanonicalAnalyticsRow struct {
	Dimensions []AnalyticsDataDimension `json:"dimensions"`
	Metrics    []json.Number            `json:"metrics"`
}

type canonicalAnalyticsEnvelope struct {
	Result struct {
		Data []CanonicalAnalyticsRow `json:"data"`
	} `json:"result"`
}

func ParseCanonicalAnalyticsResponse(raw []byte) (*CanonicalAnalyticsResponse, error) {
	var envelope canonicalAnalyticsEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode canonical analytics response: %w", err)
	}
	return &CanonicalAnalyticsResponse{Data: envelope.Result.Data, Raw: append([]byte(nil), raw...)}, nil
}

func (c *Client) analyticsDataBearer(ctx context.Context, token string, req AnalyticsDataRequest) (*CanonicalAnalyticsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal analytics request: %w", err)
	}
	raw, err := c.doBearerRequest(ctx, token, http.MethodPost, analyticsDataPath, body)
	if err != nil {
		return nil, err
	}
	out, err := ParseCanonicalAnalyticsResponse(raw.Body)
	if err != nil {
		return nil, err
	}
	out.Meta = raw.Meta
	return out, nil
}
