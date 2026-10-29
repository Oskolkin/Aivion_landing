package aidatapack

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service builds AI data packs from a shared AnalysisDataSnapshot loader.
type Service struct {
	loader analysisdata.Loader
}

// NewService creates a Service backed by the shared SQL analysisdata loader.
func NewService(db *pgxpool.Pool) *Service {
	return NewServiceWithLoader(analysisdata.NewSQLLoader(db))
}

// NewServiceWithQueries creates a Service from an existing Queries handle.
func NewServiceWithQueries(q *dbgen.Queries) *Service {
	return NewServiceWithLoader(analysisdata.NewSQLLoaderWithQueries(q))
}

// NewServiceWithLoader creates a Service with an injected analysisdata.Loader.
func NewServiceWithLoader(loader analysisdata.Loader) *Service {
	if loader == nil {
		panic("aidatapack: loader must not be nil")
	}
	return &Service{loader: loader}
}

// Build constructs a normalized SellerAIDataPack for the given seller and metric date.
func (s *Service) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*SellerAIDataPack, error) {
	if s == nil || s.loader == nil {
		return nil, fmt.Errorf("aidatapack: service loader is not configured")
	}
	snapshot, err := s.loader.Load(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	return BuildPack(snapshot)
}
