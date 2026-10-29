package pack

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Service builds and (optionally) stores Unified Analysis Packs, using the shared
// analysisdata.OptionsLoader for the 30-day pack history window.
type Service struct {
	loader    analysisdata.OptionsLoader
	builder   *PackBuilder
	validator *Validator
	storage   PackStorage
}

// NewService creates a Service. storage may be nil; BuildAndStore will then fail
// with ErrStorageNotConfigured while Build continues to work.
func NewService(loader analysisdata.OptionsLoader, builder *PackBuilder, validator *Validator, storage PackStorage) *Service {
	if loader == nil {
		panic("unifiedanalysis/pack: loader must not be nil")
	}
	if builder == nil {
		builder = NewPackBuilder(DefaultBuildOptions())
	}
	if validator == nil {
		validator = NewValidator(DefaultBuildOptions())
	}
	return &Service{loader: loader, builder: builder, validator: validator, storage: storage}
}

// Build loads a 30-day AnalysisDataSnapshot for sellerAccountID/metricDate, builds the pack,
// and runs technical validation. The result is returned even when validation fails
// (Result.Validation.Valid=false); ErrValidationFailed is returned alongside it.
func (s *Service) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*BuildResult, error) {
	if s == nil || s.loader == nil || s.builder == nil {
		return nil, ErrServiceNotConfigured
	}
	if sellerAccountID <= 0 {
		return nil, ErrInvalidSellerAccountID
	}

	snapshot, err := s.loader.LoadWithOptions(ctx, analysisdata.LoadOptions{
		SellerAccountID: sellerAccountID,
		MetricDate:      metricDate,
		HistoryDays:     contract.AnalysisPackDailyHistoryDaysV1,
	})
	if err != nil {
		return nil, err
	}

	result, err := s.builder.Build(snapshot)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot

	report, err := s.validator.ValidateZip(result.ZipBytes)
	if err != nil {
		return nil, err
	}
	result.Validation = report
	if !report.Valid {
		return result, ErrValidationFailed
	}
	if result.BusinessConsistency.HasErrors() {
		return result, ErrBusinessConsistencyFailed
	}
	return result, nil
}

// BuildAndStore builds a pack and uploads it via the configured PackStorage,
// returning the build result and the object key it was stored under.
func (s *Service) BuildAndStore(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*BuildResult, string, error) {
	if s == nil || s.storage == nil {
		return nil, "", ErrStorageNotConfigured
	}
	result, err := s.Build(ctx, sellerAccountID, metricDate)
	if err != nil {
		return result, "", err
	}
	key, err := s.storage.Put(ctx, result)
	if err != nil {
		return result, "", fmt.Errorf("unifiedanalysis/pack: store pack: %w", err)
	}
	return result, key, nil
}
