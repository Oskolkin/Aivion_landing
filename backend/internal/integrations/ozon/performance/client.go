package performance

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	transport *Transport
}

func NewClient() *Client {
	return &Client{
		transport: NewTransport(),
	}
}

func NewClientWithConfig(httpClient *http.Client, baseURL string) *Client {
	transport := NewTransport()
	if httpClient != nil {
		transport.httpClient = httpClient
	}
	if baseURL != "" {
		transport.baseURL = baseURL
	}
	return &Client{transport: transport}
}

func (c *Client) ListCampaigns(
	ctx context.Context,
	_ string,
	bearerToken string,
	req ListCampaignsRequest,
) (*TypedResponse[ListCampaignsResult], error) {
	queryParams := map[string]string{}
	if len(req.CampaignIDs) > 0 {
		ids := ""
		for idx, id := range req.CampaignIDs {
			if idx > 0 {
				ids += ","
			}
			ids += strconv.FormatInt(id, 10)
		}
		queryParams["campaignIds"] = ids
	}
	if req.AdvObjectType != "" {
		queryParams["advObjectType"] = req.AdvObjectType
	}
	if req.State != "" {
		queryParams["state"] = req.State
	}
	if req.Page > 0 {
		queryParams["page"] = strconv.FormatInt(req.Page, 10)
	}
	if req.PageSize > 0 {
		queryParams["pageSize"] = strconv.FormatInt(req.PageSize, 10)
	}

	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/campaign"},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}

	result, err := ParseCampaignsResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ListCampaignsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) GetDailyCampaignStatisticsJSON(
	ctx context.Context,
	_ string,
	bearerToken string,
	req CampaignStatisticsRequest,
) (*TypedResponse[CampaignStatisticsResult], error) {
	queryParams := map[string]string{}
	if len(req.CampaignIDs) > 0 {
		ids := ""
		for idx, id := range req.CampaignIDs {
			if idx > 0 {
				ids += ","
			}
			ids += strconv.FormatInt(id, 10)
		}
		queryParams["campaignIds"] = ids
	}
	if req.DateFrom != "" {
		queryParams["dateFrom"] = req.DateFrom
	}
	if req.DateTo != "" {
		queryParams["dateTo"] = req.DateTo
	}

	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/statistics/daily/json"},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign statistics: %w", err)
	}

	result, err := ParseDailyStatisticsResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[CampaignStatisticsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) GetCampaignProductsForSKU(
	ctx context.Context,
	_ string,
	bearerToken string,
	req CampaignProductsRequest,
) (*TypedResponse[CampaignProductsResult], error) {
	queryParams := map[string]string{}
	if req.Page > 0 {
		queryParams["page"] = strconv.FormatInt(req.Page, 10)
	}
	if req.PageSize > 0 {
		queryParams["pageSize"] = strconv.FormatInt(req.PageSize, 10)
	}

	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{fmt.Sprintf("/api/client/campaign/%d/v2/products", req.CampaignID)},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get sku campaign products: %w", err)
	}

	result, err := ParseCampaignProductsResponse(req.CampaignID, rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[CampaignProductsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) GetSearchPromoCampaignProducts(
	ctx context.Context,
	_ string,
	bearerToken string,
	req SearchPromoProductsRequest,
) (*TypedResponse[CampaignProductsResult], error) {
	payload := map[string]any{}
	if req.Page > 0 {
		payload["page"] = req.Page
	}
	if req.PageSize > 0 {
		payload["pageSize"] = req.PageSize
	}

	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodPost,
		[]string{"/api/client/campaign/search_promo/v2/products"},
		nil,
		payload,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get search promo campaign products: %w", err)
	}

	rows := extractRows(decoded)
	result := CampaignProductsResult{Items: mapSearchPromoProducts(rows)}
	return &TypedResponse[CampaignProductsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

func (c *Client) ListPerformanceCampaigns(
	ctx context.Context,
	_ string,
	bearerToken string,
	filter CampaignListFilter,
) (*TypedResponse[ListCampaignsResult], error) {
	return c.ListCampaigns(ctx, "", bearerToken, filter)
}

func (c *Client) GetPerformanceDailyStatisticsJSON(
	ctx context.Context,
	clientID string,
	bearerToken string,
	campaignIDs []int64,
	dateFrom time.Time,
	dateTo time.Time,
) (*TypedResponse[CampaignStatisticsResult], error) {
	return c.GetDailyCampaignStatisticsJSON(ctx, clientID, bearerToken, CampaignStatisticsRequest{
		CampaignIDs: campaignIDs,
		DateFrom:    dateFrom.Format("2006-01-02"),
		DateTo:      dateTo.Format("2006-01-02"),
	})
}

func (c *Client) GetPerformanceCampaignProductsV2(
	ctx context.Context,
	clientID string,
	bearerToken string,
	campaignID int64,
	page int64,
	pageSize int64,
) (*TypedResponse[CampaignProductsResult], error) {
	return c.GetCampaignProductsForSKU(ctx, clientID, bearerToken, CampaignProductsRequest{
		CampaignID: campaignID,
		Page:       page,
		PageSize:   pageSize,
	})
}

func (c *Client) GetPerformanceCampaignObjects(
	ctx context.Context,
	_ string,
	bearerToken string,
	campaignID int64,
	advObjectType string,
) (*TypedResponse[CampaignObjectsResult], error) {
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{fmt.Sprintf("/api/client/campaign/%d/objects", campaignID)},
		nil,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign objects: %w", err)
	}
	rows := extractRows(decoded)
	result := CampaignObjectsResult{Items: mapCampaignObjects(campaignID, advObjectType, rows)}
	return &TypedResponse[CampaignObjectsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

// GetCampaignObjects keeps backward compatibility for adsync fallback.
func (c *Client) GetCampaignObjects(
	ctx context.Context,
	_ string,
	bearerToken string,
	campaignID int64,
) (*TypedResponse[CampaignProductsResult], error) {
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{fmt.Sprintf("/api/client/campaign/%d/objects", campaignID)},
		nil,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign objects: %w", err)
	}
	rows := extractRows(decoded)
	result := CampaignProductsResult{Items: mapCampaignProducts(campaignID, rows)}
	return &TypedResponse[CampaignProductsResult]{
		Raw:  rawResp.Body,
		Data: result,
		Meta: rawResp.Meta,
	}, nil
}

func campaignIDsQuery(ids []int64) string {
	out := ""
	for idx, id := range ids {
		if idx > 0 {
			out += ","
		}
		out += strconv.FormatInt(id, 10)
	}
	return out
}

func (c *Client) GetCampaignProductStatisticsJSON(
	ctx context.Context,
	_ string,
	bearerToken string,
	req CampaignStatisticsRequest,
) (*TypedResponse[CampaignProductStatisticsResult], error) {
	queryParams := map[string]string{}
	if len(req.CampaignIDs) > 0 {
		queryParams["campaignIds"] = campaignIDsQuery(req.CampaignIDs)
	}
	if req.DateFrom != "" {
		queryParams["dateFrom"] = req.DateFrom
	}
	if req.DateTo != "" {
		queryParams["dateTo"] = req.DateTo
	}
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/statistics/campaign/product/json", "/api/client/statistics/campaign/product"},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign product statistics: %w", err)
	}
	result, err := ParseCampaignProductStatisticsResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[CampaignProductStatisticsResult]{Raw: rawResp.Body, Data: result, Meta: rawResp.Meta}, nil
}

func (c *Client) GetExpenseStatisticsJSON(
	ctx context.Context,
	_ string,
	bearerToken string,
	req CampaignStatisticsRequest,
) (*TypedResponse[ExpenseStatisticsResult], error) {
	queryParams := map[string]string{}
	if len(req.CampaignIDs) > 0 {
		queryParams["campaignIds"] = campaignIDsQuery(req.CampaignIDs)
	}
	if req.DateFrom != "" {
		queryParams["dateFrom"] = req.DateFrom
	}
	if req.DateTo != "" {
		queryParams["dateTo"] = req.DateTo
	}
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/statistics/expense/json", "/api/client/statistics/expense"},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get expense statistics: %w", err)
	}
	result, err := ParseExpenseStatisticsResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ExpenseStatisticsResult]{Raw: rawResp.Body, Data: result, Meta: rawResp.Meta}, nil
}

func (c *Client) SubmitCPOOrdersReport(
	ctx context.Context,
	_ string,
	bearerToken string,
	from, to time.Time,
) (*TypedResponse[ReportSubmitResult], error) {
	queryParams := map[string]string{
		"timeBounds.from": from.UTC().Format(time.RFC3339),
		"timeBounds.to":   to.UTC().Format(time.RFC3339),
	}
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/statistics/all_sku_promo/orders/generate"},
		queryParams,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("submit cpo orders report: %w", err)
	}
	result, err := ParseReportSubmitResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ReportSubmitResult]{Raw: rawResp.Body, Data: result, Meta: rawResp.Meta}, nil
}

func (c *Client) GetStatisticsReportStatus(
	ctx context.Context,
	_ string,
	bearerToken string,
	reportUUID string,
) (*TypedResponse[ReportStatus], error) {
	var decoded map[string]any
	rawResp, err := c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{fmt.Sprintf("/api/client/statistics/%s", reportUUID)},
		nil,
		nil,
		&decoded,
	)
	if err != nil {
		return nil, fmt.Errorf("get statistics report status: %w", err)
	}
	result, err := ParseReportStatusResponse(rawResp.Body)
	if err != nil {
		return nil, err
	}
	return &TypedResponse[ReportStatus]{Raw: rawResp.Body, Data: result, Meta: rawResp.Meta}, nil
}

func (c *Client) DownloadStatisticsReport(
	ctx context.Context,
	_ string,
	bearerToken string,
	reportUUID string,
) (*RawResponse, error) {
	var decoded map[string]any
	return c.transport.doJSONWithFallbackPaths(
		ctx,
		bearerToken,
		http.MethodGet,
		[]string{"/api/client/statistics/report", fmt.Sprintf("/api/client/statistics/%s", reportUUID)},
		map[string]string{"UUID": reportUUID},
		nil,
		&decoded,
	)
}
