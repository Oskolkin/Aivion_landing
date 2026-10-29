package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/financeregistry"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

var signedAmountPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(?:\.[0-9]{1,6})?$`)

type API interface {
	ListFinanceAccrualTypes(context.Context, int64) (*ozon.TypedResponse[ozon.FinanceAccrualTypesResult], error)
	ListFinanceAccrualsByDay(context.Context, int64, ozon.FinanceAccrualRequest) (*ozon.TypedResponse[ozon.FinanceAccrualResult], error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type Runner struct {
	API        API
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Now        func() time.Time
	PageSize   int
}

func (r *Runner) Collect(ctx context.Context, seller, run int64, from, to time.Time) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("finance runner is not configured")
	}
	if to.Before(from) {
		return out, fmt.Errorf("finance date range is invalid")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	if r.PageSize <= 0 {
		r.PageSize = 1000
	}
	out.StartedAt = r.Now().UTC()
	typesResponse, err := r.API.ListFinanceAccrualTypes(ctx, seller)
	if err != nil {
		return out, fmt.Errorf("finance accrual types: %w", err)
	}
	typesPayload, err := r.register(ctx, seller, run, TypesEndpoint, map[string]any{}, TypesContract, typesResponse.Raw, typesResponse.Meta.StatusCode, 1)
	if err != nil {
		return out, err
	}
	_ = typesPayload
	for _, item := range typesResponse.Data.Items {
		out.ProviderTypes = append(out.ProviderTypes, financeregistry.Normalize(financeregistry.ProviderType{
			ProviderTypeID: item.ID, Name: item.Name, Description: item.Description,
			DefaultScope: item.DefaultScope, CrossDomainOwner: item.CrossDomainOwner,
			MappingStatus: "UNMAPPED",
		}))
	}
	out.DictionaryHash = financeregistry.DictionaryHash(out.ProviderTypes)

	for day := dateOnly(from); !day.After(dateOnly(to)); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		revision := DayRevision{BusinessDate: day, CompletenessStatus: "COMPLETE"}
		cursor := ""
		for page := 1; ; page++ {
			request := ozon.FinanceAccrualRequest{DateFrom: date, DateTo: date, Page: page, PageSize: r.PageSize, Cursor: cursor}
			response, fetchErr := r.API.ListFinanceAccrualsByDay(ctx, seller, request)
			if fetchErr != nil {
				revision.CompletenessStatus = "PARTIAL"
				break
			}
			payload, registerErr := r.register(ctx, seller, run, ByDayEndpoint, request, ByDayContract, response.Raw, response.Meta.StatusCode, out.PagesReceived+1)
			if registerErr != nil {
				return out, registerErr
			}
			out.PagesReceived++
			if revision.SourcePayloadID == 0 {
				revision.SourcePayloadID = payload.ID
			}
			for _, row := range response.Data.Items {
				record, normalizeErr := normalizeRecord(row, len(revision.Records)+1)
				if normalizeErr != nil {
					return out, normalizeErr
				}
				revision.Records = append(revision.Records, record)
			}
			if !response.Data.HasNext && response.Data.NextCursor == "" {
				break
			}
			if response.Data.NextCursor != "" {
				if response.Data.NextCursor == cursor {
					revision.CompletenessStatus = "PARTIAL"
					break
				}
				cursor = response.Data.NextCursor
			}
		}
		if revision.SourcePayloadID == 0 {
			// A missing first page cannot prove an empty day and has no valid
			// source lineage, so it is not materialized as known zero.
			out.CompletenessStatus = "PARTIAL"
			continue
		}
		revision.StateHash = dayHash(revision)
		out.Days = append(out.Days, revision)
		if revision.CompletenessStatus == "PARTIAL" {
			out.CompletenessStatus = "PARTIAL"
		}
	}
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	out.CompletedAt = r.Now().UTC()
	return out, nil
}

func (r *Runner) register(ctx context.Context, seller, run int64, endpoint string, identity any, contract collection.SourceContract, body []byte, statusCode int, sequence int) (collection.SourcePayload, error) {
	seq, status := int64(sequence), int32(statusCode)
	requestIdentity, ok := identity.(map[string]any)
	if !ok {
		encoded, err := json.Marshal(identity)
		if err != nil {
			return collection.SourcePayload{}, err
		}
		if err = json.Unmarshal(encoded, &requestIdentity); err != nil {
			return collection.SourcePayload{}, err
		}
	}
	return r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{
		SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon,
		EndpointKey: endpoint, RequestIdentity: requestIdentity,
		Metadata: map[string]any{"contract_key": contract.ContractKey, "contract_version": contract.ContractVersion, "semantic_mapping_hash": contract.SemanticMappingHash, "redaction_profile": RedactionProfile},
		Body:     body, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status,
	})
}

func normalizeRecord(row ozon.FinanceAccrualRow, ordinal int) (Record, error) {
	amount, err := exactSigned(string(row.Amount))
	if err != nil {
		return Record{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(row.CurrencyCode))
	if currency == "" {
		currency = "RUB"
	}
	record := Record{RowOrdinal: ordinal, ProviderTypeID: strings.TrimSpace(row.ProviderTypeID), ProviderAccruedCategory: row.AccruedCategory, UnitNumber: row.UnitNumber, TotalAmountSigned: amount, CurrencyCode: currency, SourceFragment: row.Raw}
	if record.ProviderTypeID == "" {
		record.ProviderTypeID = "UNKNOWN"
	}
	for i, component := range row.Components {
		lineAmount, parseErr := exactSigned(string(component.Amount))
		if parseErr != nil {
			return Record{}, parseErr
		}
		path := component.Path
		if path == "" {
			path = fmt.Sprintf("components[%d]", i)
		}
		line := Line{Ordinal: i + 1, SourceComponentPath: path, SignedAmount: lineAmount, MappingStatus: "UNMAPPED", SourceFragment: component.Raw}
		if component.OzonSKU > 0 {
			v := int64(component.OzonSKU)
			line.SourceOzonSKU = &v
		}
		if component.ProductID > 0 {
			v := int64(component.ProductID)
			line.SourceProductID = &v
		}
		record.Lines = append(record.Lines, line)
	}
	return record, nil
}

func exactSigned(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !signedAmountPattern.MatchString(raw) {
		return "", fmt.Errorf("invalid exact finance amount %q", raw)
	}
	return raw, nil
}

// dayHash is semantic only: business day, completeness, parent records and lines in
// deterministic order. Identical provider rows keep distinct multiplicity via row_ordinal.
// Source fragments, payload ids, revision_no, and observed_at are excluded.
func dayHash(day DayRevision) string {
	parts := []string{day.BusinessDate.Format("2006-01-02"), day.CompletenessStatus}
	for _, record := range day.Records {
		parts = append(parts, fmt.Sprintf("%d|%s|%s|%s|%s|%s",
			record.RowOrdinal, record.ProviderTypeID, record.ProviderAccruedCategory,
			record.UnitNumber, record.TotalAmountSigned, record.CurrencyCode))
		for _, line := range record.Lines {
			parts = append(parts, fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s|%s|%s",
				line.Ordinal, line.SourceComponentPath, line.SignedAmount, line.MappingStatus,
				optionalInt64(line.CanonicalSKUID), optionalInt64(line.CanonicalOrderID),
				optionalInt64(line.CanonicalPostingID), optionalInt64(line.SourceOzonSKU),
				optionalInt64(line.SourceProductID)))
		}
	}
	return factualapi.StableHash(parts...)
}

func optionalInt64(v *int64) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprintf("%d", *v)
}

func dateOnly(v time.Time) time.Time {
	y, m, d := v.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AccountNet sums parent totals only. Breakdown lines are explanatory and are
// never added to their parent a second time.
func AccountNet(records []Record) (string, error) {
	total := new(big.Rat)
	for _, record := range records {
		value, ok := new(big.Rat).SetString(record.TotalAmountSigned)
		if !ok {
			return "", fmt.Errorf("invalid parent amount %q", record.TotalAmountSigned)
		}
		total.Add(total, value)
	}
	return strings.TrimRight(strings.TrimRight(total.FloatString(6), "0"), "."), nil
}
