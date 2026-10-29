package collection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository struct{ queries *dbgen.Queries }

func NewRepository(queries *dbgen.Queries) *Repository { return &Repository{queries: queries} }

func (r *Repository) CreateRun(ctx context.Context, sellerID int64, kind CollectionRunKind, trigger string, metadata []byte, requestedBy *int64) (CollectionRun, error) {
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}
	row, err := r.queries.CreateCollectionRun(ctx, dbgen.CreateCollectionRunParams{SellerAccountID: sellerID, Kind: string(kind), Trigger: nullableText(trigger), Column4: metadata, RequestedBy: nullableInt64(requestedBy)})
	if err != nil {
		return CollectionRun{}, fmt.Errorf("create collection run: %w", err)
	}
	return mapRun(row), nil
}
func (r *Repository) GetRun(ctx context.Context, sellerID, id int64) (CollectionRun, error) {
	row, err := r.queries.GetCollectionRunForSeller(ctx, dbgen.GetCollectionRunForSellerParams{ID: id, SellerAccountID: sellerID})
	if err != nil {
		return CollectionRun{}, err
	}
	return mapRun(row), nil
}
func (r *Repository) TransitionRun(ctx context.Context, sellerID, id int64, expected, target CollectionRunStatus) (CollectionRun, error) {
	row, err := r.queries.TransitionCollectionRunForSeller(ctx, dbgen.TransitionCollectionRunForSellerParams{ID: id, SellerAccountID: sellerID, ExpectedStatus: string(expected), Status: string(target)})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollectionRun{}, &StaleTransitionError{EntityKind: "collection run", EntityID: id, Expected: string(expected), Requested: string(target)}
	}
	if err != nil {
		return CollectionRun{}, err
	}
	return mapRun(row), nil
}
func (r *Repository) CreateDomainRun(ctx context.Context, runID, sellerID int64, domain CollectionDomain, capability *string, requestedFrom, requestedTo *time.Time) (DomainRun, error) {
	var contract *SourceContract
	if capability != nil {
		contract = &SourceContract{CapabilityKey: *capability}
	}
	return r.CreateDomainRunWithContract(ctx, runID, sellerID, domain, contract, requestedFrom, requestedTo)
}
func (r *Repository) CreateDomainRunWithContract(ctx context.Context, runID, sellerID int64, domain CollectionDomain, contract *SourceContract, requestedFrom, requestedTo *time.Time) (DomainRun, error) {
	params := dbgen.CreateCollectionDomainRunParams{CollectionRunID: runID, SellerAccountID: sellerID, Domain: string(domain), RequestedFrom: nullableDate(requestedFrom), RequestedTo: nullableDate(requestedTo)}
	if contract != nil {
		params.CapabilityKey = nullableText(contract.CapabilityKey)
		params.SourceContractKey = nullableText(contract.ContractKey)
		params.SourceContractVersion = nullableText(contract.ContractVersion)
		params.SourceEndpointFamily = nullableText(contract.EndpointFamily)
		params.SemanticMappingVersion = nullableText(contract.SemanticMappingVersion)
		params.SemanticMappingHash = nullableText(contract.SemanticMappingHash)
		// Provider revision is not fabricated from semantic mapping hash.
		params.SourceRevisionHash = pgtype.Text{}
	}
	row, err := r.queries.CreateCollectionDomainRun(ctx, params)
	if err != nil {
		return DomainRun{}, fmt.Errorf("create collection domain run: %w", err)
	}
	return mapDomainRun(row), nil
}
func (r *Repository) GetDomainRun(ctx context.Context, sellerID, id int64) (DomainRun, error) {
	row, err := r.queries.GetCollectionDomainRunForSeller(ctx, dbgen.GetCollectionDomainRunForSellerParams{ID: id, SellerAccountID: sellerID})
	if err != nil {
		return DomainRun{}, err
	}
	return mapDomainRun(row), nil
}
func (r *Repository) ListDomainRuns(ctx context.Context, sellerID, runID int64) ([]DomainRun, error) {
	rows, err := r.queries.ListCollectionDomainRunsForSeller(ctx, dbgen.ListCollectionDomainRunsForSellerParams{CollectionRunID: runID, SellerAccountID: sellerID})
	if err != nil {
		return nil, err
	}
	out := make([]DomainRun, len(rows))
	for i := range rows {
		out[i] = mapDomainRun(rows[i])
	}
	return out, nil
}
func (r *Repository) TransitionDomainRun(ctx context.Context, sellerID, id int64, expected, target DomainRunStatus) (DomainRun, error) {
	row, err := r.queries.TransitionCollectionDomainRunForSeller(ctx, dbgen.TransitionCollectionDomainRunForSellerParams{ID: id, SellerAccountID: sellerID, ExpectedStatus: string(expected), Status: string(target)})
	if errors.Is(err, pgx.ErrNoRows) {
		return DomainRun{}, &StaleTransitionError{EntityKind: "domain run", EntityID: id, Expected: string(expected), Requested: string(target)}
	}
	if err != nil {
		return DomainRun{}, err
	}
	return mapDomainRun(row), nil
}
func (r *Repository) GetCursor(ctx context.Context, sellerID int64, domain CollectionDomain, key string) (Cursor, error) {
	row, err := r.queries.GetCollectionCursorForSeller(ctx, dbgen.GetCollectionCursorForSellerParams{SellerAccountID: sellerID, Domain: string(domain), CursorKey: key})
	if err != nil {
		return Cursor{}, err
	}
	return mapCursor(row), nil
}
func (r *Repository) UpsertCursor(ctx context.Context, sellerID int64, domain CollectionDomain, key string, value []byte) (Cursor, error) {
	if len(value) == 0 {
		value = []byte("null")
	}
	row, err := r.queries.UpsertCollectionCursor(ctx, dbgen.UpsertCollectionCursorParams{SellerAccountID: sellerID, Domain: string(domain), CursorKey: key, Column4: value})
	if err != nil {
		return Cursor{}, err
	}
	return mapCursor(row), nil
}
func (r *Repository) CreatePayload(ctx context.Context, input SourcePayload) (SourcePayload, error) {
	row, err := r.queries.CreateSourcePayload(ctx, dbgen.CreateSourcePayloadParams{SellerAccountID: input.SellerAccountID, CollectionDomainRunID: input.DomainRunID, Provider: string(input.Provider), EndpointKey: input.EndpointKey, RequestFingerprint: input.RequestFingerprint, PageSeq: nullableInt8(input.PageSeq), StorageBucket: input.Bucket, StorageObjectKey: input.ObjectKey, Sha256: input.SHA256, HttpStatus: nullableInt4(input.HTTPStatus), ReceivedAt: pgtype.Timestamptz{Time: input.ReceivedAt, Valid: true}, Column12: input.Metadata, RedactionProfile: input.RedactionProfile})
	if err != nil {
		return SourcePayload{}, err
	}
	return mapPayload(row), nil
}
func (r *Repository) GetPayload(ctx context.Context, sellerID, id int64) (SourcePayload, error) {
	row, err := r.queries.GetSourcePayloadForSeller(ctx, dbgen.GetSourcePayloadForSellerParams{ID: id, SellerAccountID: sellerID})
	if err != nil {
		return SourcePayload{}, err
	}
	return mapPayload(row), nil
}
func (r *Repository) ListPayloads(ctx context.Context, sellerID, domainRunID int64) ([]SourcePayload, error) {
	rows, err := r.queries.ListSourcePayloadsForDomainRunForSeller(ctx, dbgen.ListSourcePayloadsForDomainRunForSellerParams{CollectionDomainRunID: domainRunID, SellerAccountID: sellerID})
	if err != nil {
		return nil, err
	}
	out := make([]SourcePayload, len(rows))
	for i := range rows {
		out[i] = mapPayload(rows[i])
	}
	return out, nil
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}
func nullableString(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return nullableText(*v)
}
func nullableInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
func nullableInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
func nullableInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}
func nullableDate(v *time.Time) pgtype.Date {
	if v == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *v, Valid: true}
}
func mapTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
func mapRun(v dbgen.CollectionRun) CollectionRun {
	return CollectionRun{ID: v.ID, SellerAccountID: v.SellerAccountID, Kind: CollectionRunKind(v.Kind), Status: CollectionRunStatus(v.Status), StartedAt: mapTime(v.StartedAt), FinishedAt: mapTime(v.FinishedAt)}
}
func mapDomainRun(v dbgen.CollectionDomainRun) DomainRun {
	return DomainRun{
		ID: v.ID, CollectionRunID: v.CollectionRunID, SellerAccountID: v.SellerAccountID,
		Domain: CollectionDomain(v.Domain), Status: DomainRunStatus(v.Status),
		CapabilityKey: v.CapabilityKey.String, SourceContractKey: v.SourceContractKey.String,
		SourceContractVersion: v.SourceContractVersion.String, SourceEndpointFamily: v.SourceEndpointFamily.String,
		SemanticMappingVersion: v.SemanticMappingVersion.String, SemanticMappingHash: v.SemanticMappingHash.String,
		SourceRevisionHash: v.SourceRevisionHash.String,
		StartedAt:          mapTime(v.StartedAt), FinishedAt: mapTime(v.FinishedAt),
	}
}
func mapCursor(v dbgen.CollectionCursor) Cursor {
	return Cursor{ID: v.ID, SellerAccountID: v.SellerAccountID, Domain: CollectionDomain(v.Domain), Key: v.CursorKey, Value: v.CursorValue, UpdatedAt: v.UpdatedAt.Time}
}
func mapPayload(v dbgen.SourcePayload) SourcePayload {
	var page *int64
	if v.PageSeq.Valid {
		x := v.PageSeq.Int64
		page = &x
	}
	var status *int32
	if v.HttpStatus.Valid {
		x := v.HttpStatus.Int32
		status = &x
	}
	return SourcePayload{ID: v.ID, SellerAccountID: v.SellerAccountID, DomainRunID: v.CollectionDomainRunID, Provider: Provider(v.Provider), EndpointKey: v.EndpointKey, RequestFingerprint: v.RequestFingerprint, Bucket: v.StorageBucket, ObjectKey: v.StorageObjectKey, SHA256: v.Sha256, ReceivedAt: v.ReceivedAt.Time, Metadata: v.MetadataJson, RedactionProfile: v.RedactionProfile, PageSeq: page, HTTPStatus: status}
}

var _ = pgx.ErrNoRows
