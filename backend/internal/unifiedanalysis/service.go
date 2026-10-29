package unifiedanalysis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

// PackBuilderService builds and stores analysis packs for orchestration.
type PackBuilderService interface {
	BuildAndStore(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*pack.BuildResult, string, error)
}

// PackObjectStore retrieves stored pack ZIP bytes by object key.
type PackObjectStore interface {
	Get(ctx context.Context, objectKey string) ([]byte, error)
}

type runStore interface {
	CreateRun(ctx context.Context, sellerID int64, metricDate pgtype.Date, cfg Config, startedAt pgtype.Timestamptz, trigger string) (*Run, error)
	GetRun(ctx context.Context, runID int64) (*Run, error)
	FindActiveRun(ctx context.Context, sellerID int64, metricDate pgtype.Date) (*Run, error)
	TransitionStatus(ctx context.Context, runID int64, from, to string) (*Run, error)
	SavePack(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisPackParams) (*Run, error)
	SaveContainer(ctx context.Context, runID int64, containerID string) (*Run, error)
	SaveContainerFile(ctx context.Context, runID int64, fileID, filePath string) (*Run, error)
	SaveResponseStarted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseStartedParams) (*Run, error)
	SaveResponseCompleted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseCompletedParams) (*Run, error)
	RequestCancellation(ctx context.Context, runID int64) (*Run, error)
	MarkFailed(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisFailedParams) (*Run, error)
	SaveProviderTerminalFailure(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisProviderTerminalFailureParams) (*Run, error)
	StartNextProviderAttempt(ctx context.Context, runID int64, params dbgen.StartNextUnifiedAnalysisProviderAttemptParams) (*Run, error)
	MarkCancelled(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisCancelledParams) (*Run, error)
	SaveCleanupResult(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisCleanupResultParams) (*Run, error)
	UpdateProviderStatus(ctx context.Context, runID int64, providerStatus, status string) (*Run, error)
	ListRecoverableRuns(ctx context.Context, limit int32) ([]*Run, error)
}

type artifactWriter interface {
	SaveProviderRequestSummary(ctx context.Context, runID int64, summary providerRequestSummary) error
	SaveProviderResponse(ctx context.Context, runID int64, responseID, status, outputText string, rawJSON []byte, ciCalls int) error
	SaveProviderUsage(ctx context.Context, runID int64, usage providerUsageArtifact) error
	SaveProviderTerminalAttempt(ctx context.Context, runID int64, attempt providerTerminalAttemptArtifact, rawJSON []byte) error
	SavePackReference(ctx context.Context, runID int64, ref packReferenceArtifact) error
	SavePackGenerationValidation(ctx context.Context, runID int64, report pack.GenerationValidationReport) error
	SaveDashboardFacts(ctx context.Context, runID int64, facts dashboardfacts.DashboardFactsV1) error
	LoadDashboardFacts(ctx context.Context, runID int64) (dashboardfacts.DashboardFactsV1, bool)
	SaveSourceReconciliation(ctx context.Context, runID int64, report sourcerecon.Report) error
	LoadSourceReconciliation(ctx context.Context, runID int64) (sourcerecon.Report, bool)
	SaveDateResolution(ctx context.Context, runID int64, art DateResolutionArtifact) error
	LoadDateResolution(ctx context.Context, runID int64) (DateResolutionArtifact, bool)
	SaveCleanup(ctx context.Context, runID int64, cleanup cleanupArtifact) error
	SaveOutputRaw(ctx context.Context, runID int64, raw []byte) error
	SaveOutputValidation(ctx context.Context, runID int64, report outputValidationArtifact) error
	SaveOutputValidated(ctx context.Context, runID int64, canonical []byte, outputHash string) error
	SaveOutputSummary(ctx context.Context, runID int64, summary outputSummaryArtifact) error
	LoadProviderResponseRaw(ctx context.Context, runID int64) ([]byte, bool)
	LoadValidatedMeta(ctx context.Context, runID int64) (storedValidatedMeta, bool)
	LoadOutputSummary(ctx context.Context, runID int64) (outputSummaryArtifact, bool)
	ListArtifactsByKind(ctx context.Context, runID int64, kind string) [][]byte
}

// Service orchestrates unified analysis provider smoke runs.
type Service struct {
	repo      runStore
	packs     PackBuilderService
	provider  provider.Provider
	artifacts artifactWriter
	packStore PackObjectStore
	cfg       Config
	clock     Clock

	packZipMu sync.Mutex
	packZip   map[int64][]byte
}

// NewService creates a unified analysis orchestration service.
func NewService(
	repo *Repository,
	packs PackBuilderService,
	prov provider.Provider,
	artifacts *ArtifactStore,
	packStore PackObjectStore,
	cfg Config,
) *Service {
	var store runStore = repo
	var writer artifactWriter
	if artifacts != nil {
		writer = artifacts
	}
	return &Service{
		repo:      store,
		packs:     packs,
		provider:  prov,
		artifacts: writer,
		packStore: packStore,
		cfg:       cfg,
		clock:     RealClock,
		packZip:   make(map[int64][]byte),
	}
}

// NewServiceWithStore wires a custom run store (unit tests).
func NewServiceWithStore(
	store runStore,
	packs PackBuilderService,
	prov provider.Provider,
	artifacts artifactWriter,
	packStore PackObjectStore,
	cfg Config,
) *Service {
	return &Service{
		repo:      store,
		packs:     packs,
		provider:  prov,
		artifacts: artifacts,
		packStore: packStore,
		cfg:       cfg,
		clock:     RealClock,
		packZip:   make(map[int64][]byte),
	}
}

// GetRun returns the current unified analysis run by id.
func (s *Service) GetRun(ctx context.Context, runID int64) (*Run, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("unified analysis service is not configured")
	}
	return s.repo.GetRun(ctx, runID)
}

// PollResult is returned by PollRun.
type PollResult struct {
	Run           *Run
	Completed     bool
	Failed        bool
	Cancelled     bool
	InProgress    bool
	NextPollAfter time.Duration
}
