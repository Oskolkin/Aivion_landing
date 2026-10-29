package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

func decodeResponseMap(body []byte) (map[string]any, error) {
	var decoded map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func ParseCampaignsResponse(body []byte) (ListCampaignsResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return ListCampaignsResult{}, fmt.Errorf("decode campaigns response: %w", err)
	}
	return ListCampaignsResult{Items: mapCampaigns(extractRows(decoded))}, nil
}
func ParseDailyStatisticsResponse(body []byte) (CampaignStatisticsResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return CampaignStatisticsResult{}, fmt.Errorf("decode daily statistics response: %w", err)
	}
	return CampaignStatisticsResult{Items: mapCampaignStatistics(extractRows(decoded))}, nil
}
func ParseCampaignProductsResponse(campaignID int64, body []byte) (CampaignProductsResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return CampaignProductsResult{}, fmt.Errorf("decode campaign products response: %w", err)
	}
	return CampaignProductsResult{Items: mapCampaignProducts(campaignID, extractRows(decoded))}, nil
}
func ParseCampaignProductStatisticsResponse(body []byte) (CampaignProductStatisticsResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return CampaignProductStatisticsResult{}, fmt.Errorf("decode campaign product statistics response: %w", err)
	}
	rows := extractRows(decoded)
	if len(rows) == 0 {
		if nested, ok := decoded["rows"].([]any); ok {
			rows = toMapSlice(nested)
		}
	}
	return CampaignProductStatisticsResult{Items: mapCampaignProductStatistics(rows)}, nil
}
func ParseExpenseStatisticsResponse(body []byte) (ExpenseStatisticsResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return ExpenseStatisticsResult{}, fmt.Errorf("decode expense statistics response: %w", err)
	}
	return ExpenseStatisticsResult{Items: mapExpenseStatistics(extractRows(decoded))}, nil
}
func ParseReportSubmitResponse(body []byte) (ReportSubmitResult, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return ReportSubmitResult{}, fmt.Errorf("decode report submit response: %w", err)
	}
	uuid := firstString(decoded, "UUID", "uuid", "reportId", "report_id")
	raw, _ := json.Marshal(decoded)
	return ReportSubmitResult{UUID: uuid, Raw: raw}, nil
}
func ParseReportStatusResponse(body []byte) (ReportStatus, error) {
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return ReportStatus{}, fmt.Errorf("decode report status response: %w", err)
	}
	raw, _ := json.Marshal(decoded)
	return ReportStatus{
		UUID:  firstString(decoded, "UUID", "uuid"),
		State: strings.ToUpper(firstString(decoded, "state", "status")),
		Raw:   raw,
	}, nil
}
func ParseCPOOrdersReport(body []byte) ([]map[string]any, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var rows []map[string]any
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&rows); err != nil {
			return nil, fmt.Errorf("decode cpo report array: %w", err)
		}
		return rows, nil
	}
	decoded, err := decodeResponseMap(body)
	if err != nil {
		return nil, fmt.Errorf("decode cpo report: %w", err)
	}
	rows := extractRows(decoded)
	if len(rows) == 0 {
		if nested, ok := decoded["orders"].([]any); ok {
			rows = toMapSlice(nested)
		}
		if len(rows) == 0 {
			if nested, ok := decoded["rows"].([]any); ok {
				rows = toMapSlice(nested)
			}
		}
	}
	return rows, nil
}
