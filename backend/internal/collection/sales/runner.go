package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
)

const PageSize = 1000

var ErrSourceContractMismatch = errors.New("SOURCE_CONTRACT_MISMATCH")
var ErrResumeArtifactIntegrity = errors.New("RESUME_ARTIFACT_INTEGRITY_FAILURE")

func ErrorCode(err error) string {
	switch {
	case IsResumableInterruption(err):
		return "SALES_COLLECTION_INTERRUPTED"
	case errors.Is(err, ErrInvalidSalesAmount):
		return "INVALID_SALES_AMOUNT"
	case errors.Is(err, ErrInvalidOrderedUnits):
		return "INVALID_ORDERED_UNITS"
	case errors.Is(err, ErrResumeArtifactIntegrity):
		return "RESUME_ARTIFACT_INTEGRITY_FAILURE"
	case errors.Is(err, ErrSourceContractMismatch):
		return "SOURCE_CONTRACT_MISMATCH"
	default:
		return "SALES_COLLECTION_FAILED"
	}
}

func IsResumableInterruption(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ozon.ErrNetworkError) {
		return true
	}
	var providerErr *ozon.ProviderError
	return errors.As(err, &providerErr) && (providerErr.StatusCode == 429 || providerErr.StatusCode >= 500)
}

type AnalyticsAPI interface {
	FetchCanonicalAnalyticsData(context.Context, int64, ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error)
}
type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}
type CursorWriter interface {
	UpsertCursor(context.Context, int64, collection.CollectionDomain, string, []byte) (collection.Cursor, error)
}
type ReplayRepository interface {
	GetCursor(context.Context, int64, collection.CollectionDomain, string) (collection.Cursor, error)
	ListPayloads(context.Context, int64, int64) ([]collection.SourcePayload, error)
}

type Runner struct {
	API        AnalyticsAPI
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Cursors    CursorWriter
	Recovery   ReplayRepository
	Now        func() time.Time
	Market     string
}
type candidate struct {
	SourceSKU  int64
	Date       time.Time
	Amount     Decimal
	Units      int64
	PayloadID  int64
	ObservedAt time.Time
}
type accountCandidate struct {
	Date       time.Time
	Amount     Decimal
	Units      int64
	PayloadID  int64
	ObservedAt time.Time
}
type Result struct {
	RequestedFrom, RequestedTo                                                                          time.Time
	RecordsReceived, RecordsWritten, Pages                                                              int
	SKURecords, AccountRecords, RevisionsWritten, ReusedFacts, UnresolvedSKUs                           int
	ReconciliationPASS, ReconciliationExplained, ReconciliationUnexplained, ReconciliationNotComparable int
	Currency                                                                                            string
	sku                                                                                                 []candidate
	account                                                                                             []accountCandidate
}

type streamSpec struct {
	name, endpoint string
	dimensions     []string
	contract       collection.SourceContract
	sku            bool
}
type streamCheckpoint struct {
	DomainRunID   int64  `json:"collection_domain_run_id"`
	Stream        string `json:"stream"`
	ContractKey   string `json:"logical_contract_key"`
	Version       string `json:"logical_contract_version"`
	RequestedFrom string `json:"requested_from"`
	RequestedTo   string `json:"requested_to"`
	PageSize      int    `json:"page_size"`
	PagesReceived int    `json:"pages_received"`
	NextOffset    int    `json:"next_offset"`
	Terminal      bool   `json:"terminal"`
}
type pageMetadata struct {
	LogicalContractKey     string `json:"logical_contract_key"`
	LogicalContractVersion string `json:"logical_contract_version"`
	SourceContractKey      string `json:"source_contract_key"`
	SourceContractVersion  string `json:"source_contract_version"`
	SemanticMappingHash    string `json:"semantic_mapping_hash"`
	RedactionProfile       string `json:"redaction_profile"`
	Stream                 string `json:"stream"`
	RequestedFrom          string `json:"requested_from"`
	RequestedTo            string `json:"requested_to"`
	Offset                 int    `json:"offset"`
	Limit                  int    `json:"limit"`
	ReturnedRows           int    `json:"returned_rows"`
}

func Initial90DayRange(now time.Time) (time.Time, time.Time) {
	moscow, _ := time.LoadLocation("Europe/Moscow")
	local := now.In(moscow).AddDate(0, 0, -1)
	to := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return to.AddDate(0, 0, -89), to
}

func (r *Runner) Collect(ctx context.Context, sellerID, domainRunID int64, from, to time.Time) (Result, error) {
	out := Result{RequestedFrom: day(from), RequestedTo: day(to)}
	if sellerID <= 0 || domainRunID <= 0 || out.RequestedTo.Before(out.RequestedFrom) {
		return out, fmt.Errorf("invalid sales collection scope")
	}
	if r.API == nil || r.Collection == nil || (r.Recovery != nil && r.Payloads == nil) {
		return out, errors.New("sales runner requires API/collection and a payload store for replay")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	market := r.Market
	if market == "" {
		market = "RU"
	}
	currency, err := ResolveCurrency("OZON", market)
	if err != nil {
		return out, err
	}
	out.Currency = currency
	seenSKU, seenAccount := map[string]struct{}{}, map[string]struct{}{}
	streams := []streamSpec{{"sku_daily", "ozon.analytics.sales.sku_daily", []string{"sku", "day"}, SKUDailySourceContract, true}, {"account_daily", "ozon.analytics.sales.account_daily", []string{"day"}, AccountDailySourceContract, false}}
	for _, spec := range streams {
		if err := r.collectStream(ctx, sellerID, domainRunID, &out, spec, seenSKU, seenAccount); err != nil {
			return out, err
		}
	}
	out.SKURecords, out.AccountRecords = len(out.sku), len(out.account)
	out.RecordsReceived = out.SKURecords + out.AccountRecords
	return out, nil
}

func (r *Runner) collectStream(ctx context.Context, sellerID, domainRunID int64, out *Result, spec streamSpec, seenSKU, seenAccount map[string]struct{}) error {
	offset, page, terminal, err := r.replayStream(ctx, sellerID, domainRunID, out, spec, seenSKU, seenAccount)
	if err != nil {
		return err
	}
	if terminal {
		return nil
	}
	for ; ; offset, page = offset+PageSize, page+1 {
		if err := ctx.Err(); err != nil {
			return err
		}
		req := r.request(out, spec, offset)
		resp, err := r.API.FetchCanonicalAnalyticsData(ctx, sellerID, req)
		if err != nil {
			return fmt.Errorf("%s offset %d: %w", spec.endpoint, offset, err)
		}
		pageSeq, status := int64(page), int32(resp.Meta.StatusCode)
		metadata := pageMetadata{spec.contract.ContractKey, spec.contract.ContractVersion, DomainSourceContract.ContractKey, DomainSourceContract.ContractVersion, DomainSourceContract.SemanticMappingHash, "OZON_SALES_DEMAND_REDACTION_V1", spec.name, req.DateFrom, req.DateTo, offset, PageSize, len(resp.Data)}
		payload, err := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: sellerID, DomainRunID: domainRunID, Provider: collection.ProviderOzon, EndpointKey: spec.endpoint, RequestIdentity: requestIdentity(req), Metadata: pageMetadataMap(metadata), Body: resp.Raw, ContentType: "application/json", PageSeq: &pageSeq, HTTPStatus: &status})
		if err != nil {
			return err
		}
		if err := r.acceptRows(resp, payload.ID, payload.ReceivedAt, out, spec, seenSKU, seenAccount); err != nil {
			return err
		}
		updateResultCounts(out)
		out.Pages++
		terminal = len(resp.Data) < PageSize
		if r.Cursors != nil {
			checkpoint := streamCheckpoint{domainRunID, spec.name, spec.contract.ContractKey, spec.contract.ContractVersion, req.DateFrom, req.DateTo, PageSize, page, offset + len(resp.Data), terminal}
			value, _ := json.Marshal(checkpoint)
			if _, err = r.Cursors.UpsertCursor(ctx, sellerID, collection.DomainSalesDemand, cursorKey(domainRunID, spec), value); err != nil {
				return fmt.Errorf("checkpoint %s offset %d: %w", spec.name, offset, err)
			}
		}
		if terminal {
			return nil
		}
	}
}

func (r *Runner) replayStream(ctx context.Context, sellerID, domainRunID int64, out *Result, spec streamSpec, seenSKU, seenAccount map[string]struct{}) (int, int, bool, error) {
	if r.Recovery == nil {
		return 0, 1, false, nil
	}
	payloads, err := r.Recovery.ListPayloads(ctx, sellerID, domainRunID)
	if err != nil {
		return 0, 0, false, fmt.Errorf("list replay payloads: %w", err)
	}
	streamPayloads := make([]collection.SourcePayload, 0)
	for _, payload := range payloads {
		if payload.EndpointKey == spec.endpoint {
			streamPayloads = append(streamPayloads, payload)
		}
	}
	sort.Slice(streamPayloads, func(i, j int) bool {
		if streamPayloads[i].PageSeq == nil {
			return false
		}
		if streamPayloads[j].PageSeq == nil {
			return true
		}
		if *streamPayloads[i].PageSeq == *streamPayloads[j].PageSeq {
			return streamPayloads[i].ID < streamPayloads[j].ID
		}
		return *streamPayloads[i].PageSeq < *streamPayloads[j].PageSeq
	})
	cursor, err := r.Recovery.GetCursor(ctx, sellerID, collection.DomainSalesDemand, cursorKey(domainRunID, spec))
	if errors.Is(err, pgx.ErrNoRows) {
		if len(streamPayloads) != 0 {
			return 0, 0, false, integrityError(spec, "payloads exist without checkpoint")
		}
		return 0, 1, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("get replay cursor: %w", err)
	}
	var checkpoint streamCheckpoint
	if err := json.Unmarshal(cursor.Value, &checkpoint); err != nil {
		return 0, 0, false, integrityError(spec, "invalid checkpoint JSON: %v", err)
	}
	if checkpoint.DomainRunID != domainRunID || checkpoint.Stream != spec.name || checkpoint.ContractKey != spec.contract.ContractKey || checkpoint.Version != spec.contract.ContractVersion || checkpoint.RequestedFrom != out.RequestedFrom.Format("2006-01-02") || checkpoint.RequestedTo != out.RequestedTo.Format("2006-01-02") || checkpoint.PageSize != PageSize || checkpoint.PagesReceived < 1 {
		return 0, 0, false, integrityError(spec, "checkpoint scope/contract mismatch")
	}
	if len(streamPayloads) != checkpoint.PagesReceived {
		return 0, 0, false, integrityError(spec, "checkpoint pages=%d artifacts=%d", checkpoint.PagesReceived, len(streamPayloads))
	}
	expectedOffset := 0
	for index, payload := range streamPayloads {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		expectedPage := int64(index + 1)
		if payload.SellerAccountID != sellerID || payload.DomainRunID != domainRunID || payload.PageSeq == nil || *payload.PageSeq != expectedPage || payload.RedactionProfile != "OZON_SALES_DEMAND_REDACTION_V1" {
			return 0, 0, false, integrityError(spec, "artifact page %d scope/sequence/redaction mismatch", expectedPage)
		}
		var metadata pageMetadata
		if err := json.Unmarshal(payload.Metadata, &metadata); err != nil {
			return 0, 0, false, integrityError(spec, "artifact page %d metadata: %v", expectedPage, err)
		}
		if !validPageMetadata(metadata, spec, out, expectedOffset) {
			return 0, 0, false, integrityError(spec, "artifact page %d contract/offset mismatch", expectedPage)
		}
		req := r.request(out, spec, expectedOffset)
		fingerprint, err := collection.RequestFingerprint(collection.ProviderOzon, spec.endpoint, withDomain(requestIdentity(req)))
		if err != nil || fingerprint != payload.RequestFingerprint {
			return 0, 0, false, integrityError(spec, "artifact page %d request fingerprint mismatch", expectedPage)
		}
		valid, err := r.Payloads.Verify(ctx, payload)
		if err != nil || !valid {
			return 0, 0, false, integrityError(spec, "artifact page %d hash/read failure: %v", expectedPage, err)
		}
		body, err := r.Payloads.Get(ctx, payload.Bucket, payload.ObjectKey)
		if err != nil {
			return 0, 0, false, integrityError(spec, "artifact page %d read failure: %v", expectedPage, err)
		}
		resp, err := ozon.ParseCanonicalAnalyticsResponse(body)
		if err != nil || len(resp.Data) != metadata.ReturnedRows {
			return 0, 0, false, integrityError(spec, "artifact page %d decode/row-count mismatch: %v", expectedPage, err)
		}
		if err := r.acceptRows(resp, payload.ID, payload.ReceivedAt, out, spec, seenSKU, seenAccount); err != nil {
			return 0, 0, false, integrityError(spec, "artifact page %d normalization: %v", expectedPage, err)
		}
		updateResultCounts(out)
		out.Pages++
		expectedOffset += metadata.ReturnedRows
		if index < len(streamPayloads)-1 && metadata.ReturnedRows != PageSize {
			return 0, 0, false, integrityError(spec, "non-final page %d is short", expectedPage)
		}
		if index == len(streamPayloads)-1 && checkpoint.Terminal != (metadata.ReturnedRows < PageSize) {
			return 0, 0, false, integrityError(spec, "terminal marker mismatch")
		}
	}
	if expectedOffset != checkpoint.NextOffset {
		return 0, 0, false, integrityError(spec, "next offset mismatch: replay=%d cursor=%d", expectedOffset, checkpoint.NextOffset)
	}
	return checkpoint.NextOffset, checkpoint.PagesReceived + 1, checkpoint.Terminal, nil
}

func (r *Runner) acceptRows(resp *ozon.CanonicalAnalyticsResponse, payloadID int64, receivedAt time.Time, out *Result, spec streamSpec, seenSKU, seenAccount map[string]struct{}) error {
	if receivedAt.IsZero() {
		receivedAt = r.Now()
	}
	for i, row := range resp.Data {
		if len(row.Dimensions) != len(spec.dimensions) || len(row.Metrics) != 2 {
			return fmt.Errorf("%w: %s row %d expected dimensions=%d metrics=2", ErrSourceContractMismatch, spec.endpoint, i, len(spec.dimensions))
		}
		amount, err := ParseAmount(row.Metrics[0])
		if err != nil {
			return fmt.Errorf("%w: ordered_amount: %w", ErrSourceContractMismatch, err)
		}
		units, err := ParseUnits(row.Metrics[1])
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSourceContractMismatch, err)
		}
		if spec.sku {
			id, err := strconv.ParseInt(row.Dimensions[0].ID, 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("%w: invalid sku dimension", ErrSourceContractMismatch)
			}
			d, err := parseProviderDay(row.Dimensions[1].ID, out.RequestedFrom, out.RequestedTo)
			if err != nil {
				return err
			}
			key := fmt.Sprintf("%d/%s", id, d.Format("2006-01-02"))
			if _, ok := seenSKU[key]; ok {
				return fmt.Errorf("%w: duplicated SKU grain %s", ErrSourceContractMismatch, key)
			}
			seenSKU[key] = struct{}{}
			out.sku = append(out.sku, candidate{id, d, amount, units, payloadID, receivedAt})
		} else {
			d, err := parseProviderDay(row.Dimensions[0].ID, out.RequestedFrom, out.RequestedTo)
			if err != nil {
				return err
			}
			key := d.Format("2006-01-02")
			if _, ok := seenAccount[key]; ok {
				return fmt.Errorf("%w: duplicated account grain %s", ErrSourceContractMismatch, key)
			}
			seenAccount[key] = struct{}{}
			out.account = append(out.account, accountCandidate{d, amount, units, payloadID, receivedAt})
		}
	}
	return nil
}

func updateResultCounts(out *Result) {
	out.SKURecords, out.AccountRecords = len(out.sku), len(out.account)
	out.RecordsReceived = out.SKURecords + out.AccountRecords
}

func (r *Runner) request(out *Result, spec streamSpec, offset int) ozon.AnalyticsDataRequest {
	return ozon.AnalyticsDataRequest{DateFrom: out.RequestedFrom.Format("2006-01-02"), DateTo: out.RequestedTo.Format("2006-01-02"), Metrics: []string{ozon.AnalyticsMetricRevenue, ozon.AnalyticsMetricOrderedUnits}, Dimension: spec.dimensions, Limit: PageSize, Offset: offset}
}
func requestIdentity(req ozon.AnalyticsDataRequest) map[string]any {
	return map[string]any{"date_from": req.DateFrom, "date_to": req.DateTo, "dimensions": req.Dimension, "metrics": req.Metrics, "limit": req.Limit, "offset": req.Offset}
}
func withDomain(identity map[string]any) map[string]any {
	copy := make(map[string]any, len(identity)+1)
	for k, v := range identity {
		copy[k] = v
	}
	copy["collection_domain"] = string(collection.DomainSalesDemand)
	return copy
}
func pageMetadataMap(m pageMetadata) map[string]any {
	return map[string]any{"logical_contract_key": m.LogicalContractKey, "logical_contract_version": m.LogicalContractVersion, "source_contract_key": m.SourceContractKey, "source_contract_version": m.SourceContractVersion, "semantic_mapping_hash": m.SemanticMappingHash, "redaction_profile": m.RedactionProfile, "stream": m.Stream, "requested_from": m.RequestedFrom, "requested_to": m.RequestedTo, "offset": m.Offset, "limit": m.Limit, "returned_rows": m.ReturnedRows}
}
func validPageMetadata(m pageMetadata, spec streamSpec, out *Result, offset int) bool {
	return m.LogicalContractKey == spec.contract.ContractKey && m.LogicalContractVersion == spec.contract.ContractVersion && m.SourceContractKey == DomainSourceContract.ContractKey && m.SourceContractVersion == DomainSourceContract.ContractVersion && m.SemanticMappingHash == DomainSourceContract.SemanticMappingHash && m.RedactionProfile == "OZON_SALES_DEMAND_REDACTION_V1" && m.Stream == spec.name && m.RequestedFrom == out.RequestedFrom.Format("2006-01-02") && m.RequestedTo == out.RequestedTo.Format("2006-01-02") && m.Offset == offset && m.Limit == PageSize && m.ReturnedRows >= 0 && m.ReturnedRows <= PageSize
}
func cursorKey(domainRunID int64, spec streamSpec) string {
	return fmt.Sprintf("run:%d:%s", domainRunID, spec.name)
}
func integrityError(spec streamSpec, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrResumeArtifactIntegrity, spec.name, fmt.Sprintf(format, args...))
}
func parseProviderDay(raw string, from, to time.Time) (time.Time, error) {
	d, e := time.Parse("2006-01-02", raw)
	if e != nil || d.Before(from) || d.After(to) {
		return time.Time{}, fmt.Errorf("%w: invalid/out-of-range provider day %q", ErrSourceContractMismatch, raw)
	}
	return d, nil
}
func day(v time.Time) time.Time { return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC) }
