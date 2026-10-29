package accounthealth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type AccountHealthAPI interface {
	GetAccountRatingSummary(context.Context, int64, ozon.AccountHealthRequest) (*ozon.TypedResponse[ozon.AccountHealthDocument], error)
	GetAccountRatingHistory(context.Context, int64, ozon.AccountHealthRequest) (*ozon.TypedResponse[ozon.AccountHealthDocument], error)
	GetFBSRatingIndex(context.Context, int64, ozon.AccountHealthRequest) (*ozon.TypedResponse[ozon.AccountHealthDocument], error)
	ListFBSPostingErrors(context.Context, int64, ozon.AccountHealthRequest) (*ozon.TypedResponse[ozon.AccountHealthDocument], error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type PostingResolver interface {
	ResolvePosting(context.Context, int64, string, time.Time) (*int64, string, error)
}

type Runner struct {
	API        AccountHealthAPI
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Postings   PostingResolver
	Now        func() time.Time
}

func (r *Runner) Collect(ctx context.Context, seller, run int64) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("account health runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	out.StartedAt = r.Now().UTC()

	summary, err := r.API.GetAccountRatingSummary(ctx, seller, ozon.AccountHealthRequest{})
	if err != nil {
		return out, err
	}
	summaryPayload, err := r.register(ctx, seller, run, SummaryEndpoint, SummaryContract, map[string]any{"scope": "current"}, summary, 1)
	if err != nil {
		return out, err
	}
	out.SummaryPayloadID = summaryPayload.ID
	out.SummaryRequests = 1
	out.Metrics, out.Thresholds, out.Penalties = normalizeSummary(summary.Data, summaryPayload.ID)

	historyFrom := out.StartedAt.AddDate(0, 0, -89).Format("2006-01-02")
	historyTo := out.StartedAt.Format("2006-01-02")
	history, err := r.API.GetAccountRatingHistory(ctx, seller, ozon.AccountHealthRequest{DateFrom: historyFrom, DateTo: historyTo})
	if err != nil {
		return out, err
	}
	historyPayload, err := r.register(ctx, seller, run, HistoryEndpoint, HistoryContract, map[string]any{"scope": "provider_periods", "date_from": historyFrom, "date_to": historyTo}, history, 1)
	if err != nil {
		return out, err
	}
	out.HistoryPayloadID = historyPayload.ID
	out.HistoryRequests = 1
	out.History = normalizeHistory(history.Data, historyPayload.ID)

	index, err := r.API.GetFBSRatingIndex(ctx, seller, ozon.AccountHealthRequest{})
	if err != nil {
		return out, err
	}
	indexPayload, err := r.register(ctx, seller, run, FBSIndexEndpoint, FBSIndexContract, map[string]any{"scope": "current"}, index, 1)
	if err != nil {
		return out, err
	}
	out.FBSIndexPayloadID = indexPayload.ID
	out.IndexRequests = 1
	out.FBSIndex = normalizeFBSIndex(index.Data, indexPayload.ID)

	for offset, page := 0, int64(1); ; offset, page = offset+ozon.FBSPostingErrorPageLimit, page+1 {
		response, callErr := r.API.ListFBSPostingErrors(ctx, seller, ozon.AccountHealthRequest{Limit: ozon.FBSPostingErrorPageLimit, Offset: offset})
		if callErr != nil {
			out.CompletenessStatus = "PARTIAL"
			break
		}
		payload, storeErr := r.register(ctx, seller, run, FBSErrorsEndpoint, FBSErrorsContract, map[string]any{"limit": ozon.FBSPostingErrorPageLimit, "offset": offset}, response, page)
		if storeErr != nil {
			return out, storeErr
		}
		out.FBSErrorsLastPayloadID = payload.ID
		out.ErrorPages++
		rows := objectRows(response.Data, "postings", "errors", "items")
		for _, row := range rows {
			event := normalizeFBSError(row, payload.ID)
			if r.Postings != nil && event.SourcePostingNumber != "" {
				event.CanonicalPostingID, event.ResolutionStatus, err = r.Postings.ResolvePosting(ctx, seller, event.SourcePostingNumber, out.StartedAt)
				if err != nil {
					return out, err
				}
			}
			out.FBSErrors = append(out.FBSErrors, event)
		}
		if len(rows) < ozon.FBSPostingErrorPageLimit {
			break
		}
	}
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	out.CompletedAt = r.Now().UTC()
	return out, nil
}

func (r *Runner) register(ctx context.Context, seller, run int64, endpoint string, contract collection.SourceContract, identity map[string]any, response *ozon.TypedResponse[ozon.AccountHealthDocument], page int64) (collection.SourcePayload, error) {
	status := int32(response.Meta.StatusCode)
	meta := map[string]any{"contract_key": contract.ContractKey, "contract_version": contract.ContractVersion, "semantic_mapping_hash": contract.SemanticMappingHash, "redaction_profile": RedactionProfile}
	return r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: endpoint, RequestIdentity: identity, Metadata: meta, Body: response.Raw, ContentType: "application/json", PageSeq: &page, HTTPStatus: &status})
}

func normalizeSummary(document ozon.AccountHealthDocument, payload int64) ([]MetricObservation, []ThresholdObservation, []PenaltyEvent) {
	rows := summaryRows(document)
	var metrics []MetricObservation
	var thresholds []ThresholdObservation
	for _, row := range rows {
		key := ratingIdentity(row)
		canonical, status := mapRatingKey(key)
		raw := rawJSON(row)
		metric := MetricObservation{
			ProviderRatingKey: key, CanonicalMetricKey: canonical, MappingStatus: status,
			GroupName: scalar(row, "group_name", "group"), DisplayName: scalar(row, "name", "title"),
			Unit: scalar(row, "unit", "value_unit"), ValueType: strings.ToUpper(scalar(row, "value_type", "type_value")),
			RatingDirection: strings.ToUpper(scalar(row, "rating_direction", "direction")),
			CanonicalFamily: "UNMAPPED", KnownCalculationWindow: scalar(row, "calculation_window", "window"),
			HistorySupported:        boolPointer(row, "history_supported", "has_history"),
			Value:                   scalar(row, "value", "current_value"),
			ProviderPreviousValue:   scalar(row, "past_value", "previous_value"),
			ProviderStatus:          strings.ToUpper(scalar(row, "status", "rating_status")),
			ProviderChangeDirection: strings.ToUpper(scalar(row, "change_direction", "value_change_direction")),
			ProviderChangeMeaning:   strings.ToUpper(scalar(row, "change_meaning")),
			PenaltyAmount:           scalar(row, "penalty_amount", "penalty"), PenaltyCurrency: strings.ToUpper(scalar(row, "currency", "currency_code")),
			ProviderFields: raw, SourcePayloadID: payload,
		}
		metrics = append(metrics, metric)
		for _, threshold := range nestedObjects(row, "thresholds", "statuses", "levels") {
			thresholds = append(thresholds, ThresholdObservation{
				ProviderRatingKey: key, ProviderStatus: strings.ToUpper(scalar(threshold, "status", "name")),
				From: scalar(threshold, "from", "min", "threshold_from"), To: scalar(threshold, "to", "max", "threshold_to"),
				ProviderFields: rawJSON(threshold), SourcePayloadID: payload,
			})
		}
	}
	var penalties []PenaltyEvent
	for _, row := range objectRows(document, "penalties", "penalty_events") {
		raw := rawJSON(row)
		penalties = append(penalties, PenaltyEvent{
			ProviderEventKey: stableProviderKey(row, "id", "event_id"), ProviderRatingKey: scalar(row, "rating_key", "key"),
			Amount: scalar(row, "amount", "penalty_amount"), Currency: strings.ToUpper(scalar(row, "currency", "currency_code")),
			Description: scalar(row, "description", "reason"), OccurredAt: parseTime(scalar(row, "occurred_at", "date", "created_at")),
			ProviderFields: raw, SourcePayloadID: payload,
		})
	}
	return metrics, thresholds, penalties
}

func summaryRows(document map[string]any) []map[string]any {
	if rows := objectRows(document, "ratings", "items"); len(rows) > 0 {
		return rows
	}
	groups := objectRows(document, "groups")
	var out []map[string]any
	for _, group := range groups {
		groupName := scalar(group, "group_name", "name", "title")
		for _, row := range nestedObjects(group, "ratings", "items") {
			if scalar(row, "group_name") == "" && groupName != "" {
				row["group_name"] = groupName
			}
			out = append(out, row)
		}
	}
	return out
}

func normalizeHistory(document ozon.AccountHealthDocument, payload int64) []PeriodFact {
	var out []PeriodFact
	for _, row := range objectRows(document, "history", "ratings", "items", "periods") {
		key := ratingIdentity(row)
		periods := []map[string]any{row}
		if nested := nestedObjects(row, "periods", "history", "values", "items"); len(nested) > 0 {
			periods = nested
		}
		for _, period := range periods {
			from := parseDate(scalar(period, "date_from", "period_from", "from"))
			to := parseDate(scalar(period, "date_to", "period_to", "to"))
			if from == nil || to == nil {
				continue
			}
			periodKey := scalar(period, "rating_key", "rating_id", "key", "type")
			if periodKey == "" {
				periodKey = key
			}
			out = append(out, PeriodFact{ProviderRatingKey: periodKey, Value: scalar(period, "value", "rating"), ProviderStatus: strings.ToUpper(scalar(period, "status", "rating_status")), PeriodFrom: *from, PeriodTo: *to, ProviderFields: rawJSON(period), SourcePayloadID: payload})
		}
	}
	return out
}

func normalizeFBSIndex(document ozon.AccountHealthDocument, payload int64) *FBSIndexFact {
	row := map[string]any(document)
	if result, ok := document["result"].(map[string]any); ok {
		row = result
	}
	from := parseDate(scalar(row, "date_from", "period_from", "from"))
	to := parseDate(scalar(row, "date_to", "period_to", "to"))
	if from == nil || to == nil {
		from, to = nil, nil
	}
	return &FBSIndexFact{
		Value: scalar(row, "value", "index", "error_index"), ProviderStatus: strings.ToUpper(scalar(row, "status")),
		PeriodFrom: from, PeriodTo: to,
		ProviderFields: rawJSON(row), SourcePayloadID: payload,
	}
}

func normalizeFBSError(row map[string]any, payload int64) FBSErrorEvent {
	return FBSErrorEvent{
		ProviderEventKey: stableProviderKey(row, "id", "event_id"), SourcePostingNumber: scalar(row, "posting_number", "posting"),
		ResolutionStatus: "UNRESOLVED", ErrorCode: scalar(row, "error_code", "code"), ErrorMessage: scalar(row, "error_message", "message", "reason"),
		OccurredAt: parseTime(scalar(row, "occurred_at", "created_at", "date")), ProviderFields: rawJSON(row), SourcePayloadID: payload,
	}
}

func ratingIdentity(row map[string]any) string {
	if key := scalar(row, "rating_key", "rating_id", "key", "type"); key != "" {
		return key
	}
	sum := sha256.Sum256(rawJSON(row))
	return "UNMAPPED:" + hex.EncodeToString(sum[:12])
}

// Provider keys are deliberately opt-in. An unknown key remains ingestible and
// queryable as UNMAPPED; display/group names never become identity.
var canonicalRatingKeys = map[string]string{}

func mapRatingKey(key string) (string, string) {
	if canonical := canonicalRatingKeys[strings.ToLower(strings.TrimSpace(key))]; canonical != "" {
		return canonical, "MAPPED"
	}
	return "UNMAPPED", "UNMAPPED"
}

func objectRows(document map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		if rows := nestedObjects(document, key); len(rows) > 0 {
			return rows
		}
	}
	for _, wrapper := range []string{"result", "data"} {
		if child, ok := document[wrapper].(map[string]any); ok {
			if rows := objectRows(child, keys...); len(rows) > 0 {
				return rows
			}
		}
		if values, ok := document[wrapper].([]any); ok {
			var direct []map[string]any
			for _, value := range values {
				if child, ok := value.(map[string]any); ok {
					if rows := objectRows(child, keys...); len(rows) > 0 {
						return rows
					}
					direct = append(direct, child)
				}
			}
			if len(direct) > 0 {
				return direct
			}
		}
	}
	return nil
}

func nestedObjects(row map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		values, ok := row[key].([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if object, ok := value.(map[string]any); ok {
				out = append(out, object)
			}
		}
		return out
	}
	return nil
}

func scalar(row map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := row[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case json.Number:
			return value.String()
		case bool:
			return strconv.FormatBool(value)
		}
	}
	return ""
}

func boolPointer(row map[string]any, keys ...string) *bool {
	for _, key := range keys {
		switch value := row[key].(type) {
		case bool:
			return &value
		case string:
			if parsed, err := strconv.ParseBool(value); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func stableProviderKey(row map[string]any, keys ...string) string {
	if value := scalar(row, keys...); value != "" {
		return value
	}
	sum := sha256.Sum256(rawJSON(row))
	return "ROW:" + hex.EncodeToString(sum[:])
}

func rawJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func parseDate(raw string) *time.Time {
	if value, err := time.Parse("2006-01-02", raw); err == nil {
		return &value
	}
	return nil
}

func parseTime(raw string) *time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if value, err := time.Parse(layout, raw); err == nil {
			value = value.UTC()
			return &value
		}
	}
	return nil
}
