package funnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/funnelregistry"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

const PageSize = 1000

var ErrSourceContractMismatch = errors.New("SOURCE_CONTRACT_MISMATCH")

type AnalyticsAPI interface {
	FetchCanonicalAnalyticsData(context.Context, int64, ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error)
}
type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}
type CursorWriter interface {
	UpsertCursor(context.Context, int64, collection.CollectionDomain, string, []byte) (collection.Cursor, error)
}

type Runner struct {
	API        AnalyticsAPI
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Cursors    CursorWriter
	Recovery   collection.ReplayRepository
	Now        func() time.Time
}

type streamSpec struct {
	Name       string
	Scope      string
	Dimensions []string
	Metrics    []funnelregistry.Entry
	From, To   time.Time
}

type streamCheckpoint struct {
	DomainRunID     int64  `json:"collection_domain_run_id"`
	Stream          string `json:"stream"`
	RegistryVersion string `json:"registry_version"`
	From            string `json:"from"`
	To              string `json:"to"`
	NextOffset      int    `json:"next_offset"`
	Pages           int    `json:"pages"`
	Terminal        bool   `json:"terminal"`
}

func (r *Runner) Collect(ctx context.Context, sellerID, domainRunID int64, from, to time.Time) (Result, error) {
	out := Result{RequestedFrom: dateOnly(from), RequestedTo: dateOnly(to)}
	if sellerID <= 0 || domainRunID <= 0 || out.RequestedTo.Before(out.RequestedFrom) {
		return out, fmt.Errorf("invalid funnel collection scope")
	}
	if r == nil || r.API == nil || r.Collection == nil {
		return out, errors.New("funnel runner requires API and collection boundary")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}

	additive := funnelregistry.AdditiveDailyMetrics()
	direct := funnelregistry.DirectPeriodMetrics()
	currentFrom := out.RequestedTo.AddDate(0, 0, -6)
	previousFrom, previousTo := out.RequestedTo.AddDate(0, 0, -13), out.RequestedTo.AddDate(0, 0, -7)
	streams := []streamSpec{
		{"sku_daily_additive", "SKU", []string{"sku", "day"}, additive, out.RequestedFrom, out.RequestedTo},
		{"account_daily_additive", "ACCOUNT", []string{"day"}, additive, out.RequestedFrom, out.RequestedTo},
		{"sku_current_7d_direct", "SKU", []string{"sku"}, direct, currentFrom, out.RequestedTo},
		{"account_current_7d_direct", "ACCOUNT", []string{"day"}, direct, currentFrom, out.RequestedTo},
		{"sku_previous_7d_direct", "SKU", []string{"sku"}, direct, previousFrom, previousTo},
		{"account_previous_7d_direct", "ACCOUNT", []string{"day"}, direct, previousFrom, previousTo},
	}
	pageSeq := int64(0)
	seen := map[string]struct{}{}
	for _, spec := range streams {
		if len(spec.Metrics) == 0 {
			continue
		}
		offset := 0
		for {
			req := request(spec, offset)
			resp, err := r.API.FetchCanonicalAnalyticsData(ctx, sellerID, req)
			if err != nil {
				return out, fmt.Errorf("%s offset %d: %w", spec.Name, offset, err)
			}
			pageSeq++
			status := int32(resp.Meta.StatusCode)
			identity := requestIdentity(req)
			payload, err := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{
				SellerAccountID: sellerID, DomainRunID: domainRunID, Provider: collection.ProviderOzon,
				EndpointKey: Endpoint, RequestIdentity: identity,
				Metadata: map[string]any{"stream": spec.Name, "registry_version": funnelregistry.Version(), "semantic_mapping_hash": DomainSourceContract.SemanticMappingHash, "redaction_profile": RedactionProfile, "returned_rows": len(resp.Data)},
				Body:     resp.Raw, ContentType: "application/json", PageSeq: &pageSeq, HTTPStatus: &status,
			})
			if err != nil {
				return out, err
			}
			if err = r.acceptRows(resp, payload, spec, &out, seen); err != nil {
				return out, err
			}
			out.Pages++
			terminal := len(resp.Data) < PageSize
			if r.Cursors != nil {
				cp, _ := json.Marshal(streamCheckpoint{domainRunID, spec.Name, funnelregistry.Version(), req.DateFrom, req.DateTo, offset + len(resp.Data), out.Pages, terminal})
				if _, err = r.Cursors.UpsertCursor(ctx, sellerID, collection.DomainFunnel, fmt.Sprintf("run:%d:%s", domainRunID, spec.Name), cp); err != nil {
					return out, fmt.Errorf("checkpoint %s: %w", spec.Name, err)
				}
			}
			if terminal {
				break
			}
			offset += PageSize
		}
	}
	out.RecordsReceived = len(out.Observations)
	return out, nil
}

func (r *Runner) acceptRows(resp *ozon.CanonicalAnalyticsResponse, payload collection.SourcePayload, spec streamSpec, out *Result, seen map[string]struct{}) error {
	observed := payload.ReceivedAt
	if observed.IsZero() {
		observed = r.Now()
	}
	for rowIndex, row := range resp.Data {
		if len(row.Dimensions) != len(spec.Dimensions) || len(row.Metrics) != len(spec.Metrics) {
			return fmt.Errorf("%w: %s row %d dimensions/metrics mismatch", ErrSourceContractMismatch, spec.Name, rowIndex)
		}
		var sourceSKU *int64
		periodStart, periodEnd := spec.From, spec.To
		if spec.Scope == "SKU" {
			n, err := strconv.ParseInt(row.Dimensions[0].ID, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("%w: invalid ozon sku", ErrSourceContractMismatch)
			}
			sourceSKU = &n
			if len(spec.Dimensions) == 2 {
				periodStart, err = parseDay(row.Dimensions[1].ID, spec.From, spec.To)
				if err != nil {
					return err
				}
				periodEnd = periodStart
			}
		} else if len(spec.Dimensions) == 1 && spec.Metrics[0].TemporalSemantics == funnelregistry.AdditiveFlow {
			var err error
			periodStart, err = parseDay(row.Dimensions[0].ID, spec.From, spec.To)
			if err != nil {
				return err
			}
			periodEnd = periodStart
		}
		for i, metric := range spec.Metrics {
			if metric.ProviderMetric == "ordered_units" || metric.ProviderMetric == "revenue" {
				return fmt.Errorf("%w: sales metric in funnel stream", ErrSourceContractMismatch)
			}
			value := row.Metrics[i].String()
			rat, ok := new(big.Rat).SetString(value)
			if !ok || rat.Sign() < 0 {
				return fmt.Errorf("%w: invalid metric number %q", ErrSourceContractMismatch, value)
			}
			entity := "account"
			if sourceSKU != nil {
				entity = strconv.FormatInt(*sourceSKU, 10)
			}
			key := fmt.Sprintf("%s/%s/%s/%s", spec.Scope, entity, metric.CanonicalKey, periodStart.Format("2006-01-02")+".."+periodEnd.Format("2006-01-02"))
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: duplicate funnel grain %s", ErrSourceContractMismatch, key)
			}
			seen[key] = struct{}{}
			resolution := "NOT_APPLICABLE"
			if sourceSKU != nil {
				resolution = "UNRESOLVED"
			}
			out.Observations = append(out.Observations, Observation{
				ScopeKind: spec.Scope, SourceSKU: sourceSKU, SKUResolutionStatus: resolution,
				MetricKey: metric.CanonicalKey, PeriodStart: periodStart, PeriodEnd: periodEnd,
				ValueNumeric: value, Unit: metric.Unit, TemporalSemantics: string(metric.TemporalSemantics),
				EntityAggregationSemantics: string(metric.EntityAggregationSemantics), Surface: string(metric.Surface),
				ProviderMetricName: metric.ProviderMetric, RegistryVersion: funnelregistry.Version(),
				SourceContractHash: DomainSourceContract.SemanticMappingHash, SourcePayloadID: payload.ID, ObservedAt: observed,
			})
		}
	}
	return nil
}

func request(spec streamSpec, offset int) ozon.AnalyticsDataRequest {
	metrics := make([]string, len(spec.Metrics))
	for i, metric := range spec.Metrics {
		metrics[i] = metric.ProviderMetric
	}
	return ozon.AnalyticsDataRequest{DateFrom: spec.From.Format("2006-01-02"), DateTo: spec.To.Format("2006-01-02"), Metrics: metrics, Dimension: spec.Dimensions, Limit: PageSize, Offset: offset}
}
func requestIdentity(req ozon.AnalyticsDataRequest) map[string]any {
	return map[string]any{"date_from": req.DateFrom, "date_to": req.DateTo, "dimensions": req.Dimension, "metrics": req.Metrics, "limit": req.Limit, "offset": req.Offset}
}
func parseDay(raw string, from, to time.Time) (time.Time, error) {
	d, err := time.Parse("2006-01-02", raw)
	if err != nil || d.Before(from) || d.After(to) {
		return time.Time{}, fmt.Errorf("%w: invalid provider day %q", ErrSourceContractMismatch, raw)
	}
	return d, nil
}
func dateOnly(v time.Time) time.Time {
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
