package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type StockAPI interface {
	ListCanonicalStocks(context.Context, int64, ozon.CanonicalStockRequest) (*ozon.TypedResponse[[]ozon.CanonicalStockRow], error)
}

type UniverseResolver interface {
	ResolveStockUniverse(context.Context, int64, time.Time) ([]FrozenSKU, error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type Runner struct {
	API        StockAPI
	Universe   UniverseResolver
	Collection PayloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Recovery   collection.ReplayRepository
	Now        func() time.Time
}

func (r *Runner) Collect(ctx context.Context, seller, run int64) (Result, error) {
	var out Result
	if r == nil || r.API == nil || r.Universe == nil || r.Collection == nil || r.Payloads == nil {
		return out, fmt.Errorf("stock runner is not configured")
	}
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	out.StartedAt = r.Now().UTC()
	universe, err := r.Universe.ResolveStockUniverse(ctx, seller, out.StartedAt)
	if err != nil {
		return out, err
	}
	sort.Slice(universe, func(i, j int) bool { return universe[i].OzonSKU < universe[j].OzonSKU })
	hashParts := make([]string, 0, len(universe))
	bySKU := make(map[int64]int64, len(universe))
	for _, sku := range universe {
		hashParts = append(hashParts, fmt.Sprintf("%d:%d", sku.CanonicalSKUID, sku.OzonSKU))
		bySKU[sku.OzonSKU] = sku.CanonicalSKUID
	}
	out.UniverseHash = factualapi.StableHash(hashParts...)
	out.ExpectedSKUCount, out.RequestedSKUCount = len(universe), len(universe)
	limit := ozon.StockAnalyticsBatchLimit
	if len(universe) > 0 {
		out.BatchesExpected = (len(universe) + limit - 1) / limit
	}
	replay, err := collection.LoadReplayArtifacts(ctx, r.Recovery, r.Payloads, seller, run, PrimaryEndpoint, RedactionProfile)
	if err != nil {
		return out, err
	}
	if len(replay) > out.BatchesExpected {
		return out, fmt.Errorf("stock resume has more artifacts than frozen batches")
	}
	seen := map[int64]struct{}{}
	failedBatchSKUs := map[int64]struct{}{}
	completedBatchSKUs := map[int64]struct{}{}
	for start, batchIndex := 0, 0; start < len(universe); start, batchIndex = start+limit, batchIndex+1 {
		end := start + limit
		if end > len(universe) {
			end = len(universe)
		}
		batch := universe[start:end]
		skus := make([]string, 0, len(batch))
		for _, x := range batch {
			skus = append(skus, strconv.FormatInt(x.OzonSKU, 10))
		}
		identity := map[string]any{"skus": skus, "limit": limit}
		var resp *ozon.TypedResponse[[]ozon.CanonicalStockRow]
		var payload collection.SourcePayload
		if batchIndex < len(replay) {
			artifact := replay[batchIndex]
			if err = validateStockReplay(artifact, identity, batchIndex+1); err != nil {
				return out, err
			}
			rows, decodeErr := decodeStockRows(artifact.Body)
			if decodeErr != nil {
				return out, decodeErr
			}
			resp = &ozon.TypedResponse[[]ozon.CanonicalStockRow]{Data: rows, Raw: artifact.Body}
			payload = artifact.Payload
		} else {
			var fetchErr error
			resp, fetchErr = r.API.ListCanonicalStocks(ctx, seller, ozon.CanonicalStockRequest{SKUs: skus})
			if fetchErr != nil {
				out.BatchesFailed++
				for _, x := range batch {
					failedBatchSKUs[x.OzonSKU] = struct{}{}
				}
				out.CompletenessStatus = "PARTIAL"
				continue
			}
			seq := int64(batchIndex + 1)
			status := int32(resp.Meta.StatusCode)
			payload, err = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{
				SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon,
				EndpointKey: PrimaryEndpoint, RequestIdentity: identity,
				Metadata: map[string]any{"contract_key": PrimaryContract.ContractKey, "contract_version": PrimaryContract.ContractVersion, "semantic_mapping_hash": PrimaryContract.SemanticMappingHash, "redaction_profile": RedactionProfile, "batch_index": batchIndex + 1, "returned_rows": len(resp.Data)},
				Body:     resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status,
			})
			if err != nil {
				return out, err
			}
		}
		out.BatchesCompleted++
		for _, x := range batch {
			completedBatchSKUs[x.OzonSKU] = struct{}{}
		}
		for _, row := range resp.Data {
			sku := int64(row.SKU)
			if sku <= 0 {
				continue
			}
			seen[sku] = struct{}{}
			resolution := "UNRESOLVED"
			var canonical *int64
			if id, ok := bySKU[sku]; ok {
				resolution = "RESOLVED"
				v := id
				canonical = &v
			}
			providerFields, _ := json.Marshal(row)
			locationKey := fmt.Sprintf("WAREHOUSE:%d", int64(row.WarehouseID))
			poolKey := fmt.Sprintf("FBO:WAREHOUSE:%d", int64(row.WarehouseID))
			payloadID := payload.ID
			out.Positions = append(out.Positions, Position{
				CanonicalSKUID: canonical, SourceSKU: sku, SKUResolutionStatus: resolution,
				ProviderPoolKey: poolKey, FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED",
				ProviderLocationKey: locationKey, LocationType: "WAREHOUSE", LocationName: row.WarehouseName,
				ProviderValidStockCount: row.ValidStockCount, QuantitySemanticStatus: "PENDING_ACCEPTANCE", ProviderFields: providerFields,
				ProviderADS: row.ProviderADS.String(), ProviderADSCluster: row.ProviderADSCluster.String(), ProviderIDC: row.ProviderIDC.String(), ProviderIDCCluster: row.ProviderIDCCluster.String(),
				ProviderTurnoverGrade: row.ProviderTurnoverGrade, ProviderTurnoverGradeCluster: row.ProviderTurnoverGradeCluster,
				ProviderDaysWithoutSales: row.ProviderDaysWithoutSales, ProviderDaysWithoutSalesCluster: row.ProviderDaysWithoutSalesCluster,
				SourcePayloadID: &payloadID, SourceReceivedAt: payload.ReceivedAt,
			})
		}
	}
	out.ReceivedSKUCount = len(seen)
	for _, sku := range universe {
		if _, ok := seen[sku.OzonSKU]; ok {
			continue
		}
		canonical := sku.CanonicalSKUID
		reason := "MISSING_PROVIDER_RESPONSE_ROW"
		if _, failed := failedBatchSKUs[sku.OzonSKU]; failed {
			reason = "SOURCE_BATCH_UNAVAILABLE"
			out.BatchUnavailableSKUCount++
		} else if _, completed := completedBatchSKUs[sku.OzonSKU]; completed {
			out.MissingResponseSKUCount++
		} else {
			reason = "SOURCE_BATCH_UNAVAILABLE"
			out.BatchUnavailableSKUCount++
		}
		fields, _ := json.Marshal(map[string]any{"absence_reason": reason})
		out.Positions = append(out.Positions, Position{
			CanonicalSKUID: &canonical, SourceSKU: sku.OzonSKU, SKUResolutionStatus: "RESOLVED",
			ProviderPoolKey: "UNKNOWN:ABSENCE", FulfillmentType: "UNKNOWN", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED",
			QuantitySemanticStatus: "PENDING_ACCEPTANCE", AbsenceReason: reason, ProviderFields: fields,
		})
	}
	out.CompletedAt = r.Now().UTC()
	if out.CompletenessStatus == "" {
		out.CompletenessStatus = "COMPLETE"
	}
	if out.BatchesFailed > 0 || out.ReceivedSKUCount < out.ExpectedSKUCount {
		out.CompletenessStatus = "PARTIAL"
	}
	return out, nil
}

func decodeStockRows(body []byte) ([]ozon.CanonicalStockRow, error) {
	var envelope struct {
		Items []ozon.CanonicalStockRow `json:"items"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode stock replay: %w", err)
	}
	if envelope.Items == nil {
		if err := json.Unmarshal(body, &envelope.Items); err != nil {
			return nil, fmt.Errorf("decode stock replay array: %w", err)
		}
	}
	return envelope.Items, nil
}
func validateStockReplay(a collection.ReplayArtifact, identity map[string]any, page int) error {
	if a.Payload.PageSeq == nil || *a.Payload.PageSeq != int64(page) {
		return fmt.Errorf("stock replay sequence mismatch")
	}
	request := map[string]any{"collection_domain": string(collection.DomainStock)}
	for k, v := range identity {
		request[k] = v
	}
	fingerprint, err := collection.RequestFingerprint(collection.ProviderOzon, PrimaryEndpoint, request)
	if err != nil || fingerprint != a.Payload.RequestFingerprint {
		return fmt.Errorf("stock replay request fingerprint mismatch")
	}
	var metadata map[string]any
	if json.Unmarshal(a.Payload.Metadata, &metadata) != nil || metadata["semantic_mapping_hash"] != PrimaryContract.SemanticMappingHash {
		return fmt.Errorf("stock replay source contract mismatch")
	}
	return nil
}
