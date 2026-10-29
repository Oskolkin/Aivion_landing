package unifiedanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

type memoryRepository struct {
	mu        sync.Mutex
	nextID    int64
	runs      map[int64]dbgen.AiPipelineRun
	artifacts []dbgen.AiPipelineArtifact
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		nextID: 1,
		runs:   make(map[int64]dbgen.AiPipelineRun),
	}
}

func (m *memoryRepository) CreateRun(ctx context.Context, sellerID int64, metricDate pgtype.Date, cfg Config, startedAt pgtype.Timestamptz, trigger string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	now := time.Now().UTC()
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		trigger = TriggerManual
	}
	row := dbgen.AiPipelineRun{
		ID:                    id,
		SellerAccountID:       sellerID,
		MetricDate:            metricDate,
		Mode:                  ModeUnifiedAnalysis,
		Trigger:               trigger,
		Status:                StatusCreated,
		DataPackSchemaVersion: pgtype.Text{String: AnalysisPackSchemaVersion, Valid: true},
		DataPackHash:          DataPackHashPending,
		PipelineVersion:       pgtype.Text{String: PipelineVersionV1, Valid: true},
		PromptVersion:         pgtype.Text{String: firstNonEmpty(cfg.PromptVersion, promptVersionForConfig(cfg)), Valid: true},
		Provider:              pgtype.Text{String: cfg.Provider, Valid: cfg.Provider != ""},
		ProviderModel:         pgtype.Text{String: cfg.Model, Valid: cfg.Model != ""},
		StartedAt:             startedAt,
		CreatedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:             pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.runs[id] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) GetRun(ctx context.Context, runID int64) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	return RunFromDB(row), nil
}

func (m *memoryRepository) FindActiveRun(ctx context.Context, sellerID int64, metricDate pgtype.Date) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *dbgen.AiPipelineRun
	for id, row := range m.runs {
		if row.SellerAccountID != sellerID || !sameDate(row.MetricDate, metricDate) {
			continue
		}
		if row.Mode != ModeUnifiedAnalysis || !IsActiveUnifiedRunStatus(row.Status) {
			continue
		}
		if latest == nil || id > latest.ID {
			copy := row
			latest = &copy
		}
	}
	if latest == nil {
		return nil, nil
	}
	return RunFromDB(*latest), nil
}

func (m *memoryRepository) TransitionStatus(ctx context.Context, runID int64, from, to string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	if row.Status != from {
		return nil, ErrCASConflict
	}
	row.Status = to
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SavePack(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisPackParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.PackID = params.PackID
	row.PackHash = params.PackHash
	row.PackStorageKey = params.PackStorageKey
	row.PackArchiveSha256 = params.PackArchiveSha256
	row.PackCompressedBytes = params.PackCompressedBytes
	row.PackUncompressedBytes = params.PackUncompressedBytes
	row.DataPackSchemaVersion = params.DataPackSchemaVersion
	row.DataPackHash = textOrEmpty(params.PackHash)
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveContainer(ctx context.Context, runID int64, containerID string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.ProviderContainerID = pgtype.Text{String: containerID, Valid: containerID != ""}
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveContainerFile(ctx context.Context, runID int64, fileID, filePath string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.ProviderContainerFileID = pgtype.Text{String: fileID, Valid: fileID != ""}
	row.ProviderContainerFilePath = pgtype.Text{String: filePath, Valid: filePath != ""}
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveResponseStarted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseStartedParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.ProviderResponseID = params.ProviderResponseID
	row.ProviderStatus = params.ProviderStatus
	row.ProviderStartedAt = params.ProviderStartedAt
	if params.ProviderRequestID.Valid {
		row.ProviderRequestID = params.ProviderRequestID
	}
	row.ProviderAttemptCount++
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveResponseCompleted(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisResponseCompletedParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	// Idempotent completion: only provider_running may transition to completed/validating.
	if row.Status == StatusProviderCompleted || row.Status == StatusOutputValidating ||
		row.Status == StatusPublicationReady || row.Status == StatusPublicationBlocked {
		return RunFromDB(row), nil
	}
	if row.Status != StatusProviderRunning {
		return nil, ErrCASConflict
	}
	row.Status = params.Status
	row.ProviderStatus = params.ProviderStatus
	row.ProviderCompletedAt = params.ProviderCompletedAt
	row.ProviderLatencyMs = params.ProviderLatencyMs
	row.InputTokens = params.InputTokens
	row.OutputTokens = params.OutputTokens
	row.TotalTokens = params.TotalTokens
	row.CodeInterpreterCalls = params.CodeInterpreterCalls
	row.CleanupStatus = params.CleanupStatus
	row.CompletedAt = params.CompletedAt
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) RequestCancellation(ctx context.Context, runID int64) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	now := time.Now().UTC()
	if !row.CancelRequestedAt.Valid {
		row.CancelRequestedAt = pgtype.Timestamptz{Time: now, Valid: true}
	}
	row.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) MarkFailed(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisFailedParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.Status = StatusFailed
	row.ErrorCode = params.ErrorCode
	row.ErrorMessage = params.ErrorMessage
	if params.ProviderStatus.Valid {
		row.ProviderStatus = params.ProviderStatus
	}
	row.CleanupStatus = params.CleanupStatus
	now := time.Now().UTC()
	row.CompletedAt = pgtype.Timestamptz{Time: now, Valid: true}
	row.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveProviderTerminalFailure(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisProviderTerminalFailureParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.Status = StatusFailed
	row.ProviderStatus = params.ProviderStatus
	row.ProviderCompletedAt = params.ProviderCompletedAt
	row.ProviderLatencyMs = params.ProviderLatencyMs
	row.InputTokens = params.InputTokens
	row.OutputTokens = params.OutputTokens
	row.TotalTokens = params.TotalTokens
	row.CodeInterpreterCalls = params.CodeInterpreterCalls
	row.ErrorCode = params.ErrorCode
	row.ErrorMessage = params.ErrorMessage
	if params.CleanupStatus.Valid {
		row.CleanupStatus = params.CleanupStatus
	}
	now := time.Now().UTC()
	row.CompletedAt = pgtype.Timestamptz{Time: now, Valid: true}
	row.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) StartNextProviderAttempt(ctx context.Context, runID int64, params dbgen.StartNextUnifiedAnalysisProviderAttemptParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	if row.Status != StatusProviderRunning {
		return nil, ErrCASConflict
	}
	expected := textOrEmpty(params.ProviderResponseID)
	current := textOrEmpty(row.ProviderResponseID)
	if expected == "" || current != expected {
		return nil, ErrCASConflict
	}
	if row.ProviderAttemptCount >= params.ProviderAttemptCount {
		return nil, ErrCASConflict
	}
	row.ProviderResponseID = params.ProviderResponseID_2
	row.ProviderStatus = params.ProviderStatus
	row.ProviderRequestID = params.ProviderRequestID
	row.ProviderStartedAt = params.ProviderStartedAt
	row.ProviderAttemptCount++
	row.ProviderCompletedAt = pgtype.Timestamptz{}
	row.ProviderLatencyMs = pgtype.Int8{}
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) MarkCancelled(ctx context.Context, runID int64, params dbgen.MarkUnifiedAnalysisCancelledParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.Status = StatusCancelled
	row.ProviderStatus = params.ProviderStatus
	row.CleanupStatus = params.CleanupStatus
	now := time.Now().UTC()
	row.CompletedAt = pgtype.Timestamptz{Time: now, Valid: true}
	row.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) SaveCleanupResult(ctx context.Context, runID int64, params dbgen.SaveUnifiedAnalysisCleanupResultParams) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.CleanupStatus = params.CleanupStatus
	if params.CleanupStartedAt.Valid {
		row.CleanupStartedAt = params.CleanupStartedAt
	}
	if params.CleanupCompletedAt.Valid {
		row.CleanupCompletedAt = params.CleanupCompletedAt
	}
	row.CleanupError = params.CleanupError
	if params.Status.Valid {
		row.Status = params.Status.String
	}
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) UpdateProviderStatus(ctx context.Context, runID int64, providerStatus, status string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	row.ProviderStatus = pgtype.Text{String: providerStatus, Valid: providerStatus != ""}
	if status != "" {
		row.Status = status
	}
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func (m *memoryRepository) ListRecoverableRuns(ctx context.Context, limit int32) ([]*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Run, 0)
	for _, row := range m.runs {
		if row.Mode != ModeUnifiedAnalysis || !isRecoverableStatus(row) {
			continue
		}
		copy := row
		out = append(out, RunFromDB(copy))
	}
	if int32(len(out)) > limit && limit > 0 {
		out = out[:limit]
	}
	return out, nil
}

func (m *memoryRepository) CreateArtifact(ctx context.Context, params dbgen.CreateAIPipelineArtifactParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := int64(len(m.artifacts) + 1)
	m.artifacts = append(m.artifacts, dbgen.AiPipelineArtifact{
		ID:            id,
		PipelineRunID: params.PipelineRunID,
		Kind:          params.Kind,
		Hash:          params.Hash,
		Storage:       params.Storage,
		ObjectKey:     params.ObjectKey,
		PayloadJson:   params.PayloadJson,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	return id, nil
}

func isRecoverableStatus(row dbgen.AiPipelineRun) bool {
	switch row.Status {
	case StatusPackBuilding, StatusPackValidating, StatusPackReady,
		StatusProviderContainerCreating, StatusProviderContainerReady,
		StatusProviderFileUploading, StatusProviderFileReady,
		StatusProviderResponseStarting, StatusProviderRunning,
		StatusCleanupPending, StatusCleanupRunning,
		StatusOutputValidating, StatusProviderCompleted:
		return true
	case StatusPublicationReady, StatusPublishing:
		return true
	case StatusPublicationBlocked:
		cleanup := textOrEmpty(row.CleanupStatus)
		return cleanup == "" || cleanup == CleanupStatusPending || cleanup == CleanupStatusRunning || cleanup == CleanupStatusFailed
	default:
		return false
	}
}

func (m *memoryRepository) ListArtifacts(ctx context.Context, runID int64) ([]dbgen.AiPipelineArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]dbgen.AiPipelineArtifact, 0)
	for _, art := range m.artifacts {
		if art.PipelineRunID == runID {
			out = append(out, art)
		}
	}
	return out, nil
}

func (m *memoryRepository) SavePublicationBlocked(ctx context.Context, runID int64, code, message string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.runs[runID]
	if !ok {
		return nil, ErrRunNotFound
	}
	if row.Status != StatusOutputValidating {
		return nil, ErrCASConflict
	}
	row.Status = StatusPublicationBlocked
	row.ErrorCode = nullableText(code)
	row.ErrorMessage = nullableText(message)
	row.UpdatedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.runs[runID] = row
	return RunFromDB(row), nil
}

func sameDate(a, b pgtype.Date) bool {
	if !a.Valid || !b.Valid {
		return false
	}
	return a.Time.UTC().Format("2006-01-02") == b.Time.UTC().Format("2006-01-02")
}

func textOrEmpty(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

type stubPackBuilder struct {
	result *pack.BuildResult
	key    string
	err    error
	calls  int
}

func (s *stubPackBuilder) BuildAndStore(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*pack.BuildResult, string, error) {
	s.calls++
	if s.err != nil {
		return s.result, "", s.err
	}
	if s.result == nil {
		return nil, "", nil
	}
	out := *s.result
	if out.Snapshot == nil {
		out.Snapshot = minimalTestSnapshot(sellerAccountID, metricDate)
	}
	return &out, s.key, nil
}

func validStubPack(packID string) *pack.BuildResult {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	zipBytes, packHash := mustMinimalValidPackZip(packID, 1, metricDate.Format("2006-01-02"))
	return &pack.BuildResult{
		SchemaVersion:   contract.AnalysisPackSchemaVersionV1,
		PackID:          packID,
		PackHash:        packHash,
		SellerAccountID: 1,
		MetricDate:      metricDate.Format("2006-01-02"),
		ZipBytes:        zipBytes,
		Validation: pack.ValidationReport{
			Valid:             true,
			CompressedBytes:   int64(len(zipBytes)),
			UncompressedBytes: int64(len(zipBytes)),
		},
		Snapshot: minimalTestSnapshot(1, metricDate),
	}
}

// mustMinimalValidPackZip builds a technically valid Analysis Pack ZIP that
// passes pack.ValidateAnalysisPackForGeneration (correct entry order, CSV
// headers, schema_version JSON docs, recomputed pack hash).
func mustMinimalValidPackZip(packID string, sellerID int64, metricDate string) (zipBytes []byte, packHash string) {
	files := make(map[string]pack.GeneratedFile, len(contract.AnalysisPackFilesV1))
	for _, def := range contract.AnalysisPackFilesV1 {
		var content []byte
		switch def.Kind {
		case "csv":
			cols := contract.AnalysisPackCSVColumnsV1[def.Name]
			header := strings.Join(cols, ",") + "\n"
			content = []byte(header)
		case "json":
			switch def.Name {
			case "manifest.json":
				manifest := contract.AnalysisPackManifestV1{
					SchemaVersion:   contract.AnalysisPackSchemaVersionV1,
					PackID:          packID,
					SellerAccountID: sellerID,
					MetricDate:      metricDate,
					Timezone:        "Europe/Moscow",
					Currency:        "RUB",
					DataStatus:      "available",
				}
				raw, err := json.Marshal(manifest)
				if err != nil {
					panic(err)
				}
				content = raw
			case "limitations.json":
				content = []byte(`{"schema_version":"aivion_analysis_pack_v1","items":[]}`)
			case "analysis_policy.json":
				raw, err := json.Marshal(contract.DefaultAnalysisPolicyV1())
				if err != nil {
					panic(err)
				}
				content = raw
			default:
				content = []byte(`{"schema_version":"aivion_analysis_pack_v1"}`)
			}
		}
		sum := sha256.Sum256(content)
		files[def.Name] = pack.GeneratedFile{
			Name:    def.Name,
			Content: content,
			SHA256:  hex.EncodeToString(sum[:]),
			Kind:    def.Kind,
		}
	}
	hash, err := pack.ComputeContentHash(contract.AnalysisPackSchemaVersionV1, files)
	if err != nil {
		panic(err)
	}
	// Rebuild manifest with final pack_id (already set) — pack hash lives outside
	// the ZIP manifest by design; keep packID as provided.
	zipBytes, err = pack.BuildZip(files)
	if err != nil {
		panic(err)
	}
	return zipBytes, hash
}

func minimalTestSnapshot(sellerID int64, metricDate time.Time) *analysisdata.AnalysisDataSnapshot {
	var revenue pgtype.Numeric
	// pgtype.Numeric.Scan expects a string form; a float64 silently fails and
	// leaves the value invalid (0), which now trips sourcerecon's
	// sku_orders_without_revenue check during dashboard facts validation.
	_ = revenue.Scan("100.00")
	// Sku must match contract/testdata/unified_output_v1.no_actions.json sku_focus
	// (801000100001) so BuildDashboardFacts → ProjectDashboard join yields
	// non-empty sku_focus for bundlevalidator.
	skuID := int64(801000100001)
	offerID := "HOME-KITCHEN-001"
	productName := "SKU One"
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		AccountMetrics: []dbgen.DailyAccountMetric{
			{SellerAccountID: sellerID, MetricDate: pgtype.Date{Time: metricDate, Valid: true}, Revenue: revenue, OrdersCount: 2},
		},
		SKUMetrics: []dbgen.DailySkuMetric{
			{
				SellerAccountID: sellerID, MetricDate: pgtype.Date{Time: metricDate, Valid: true},
				OzonProductID: 1001,
				Sku:           pgtype.Int8{Int64: skuID, Valid: true},
				OfferID:       pgtype.Text{String: offerID, Valid: true},
				ProductName:   pgtype.Text{String: productName, Valid: true},
				Revenue:       revenue, OrdersCount: 2, StockAvailable: 1,
			},
		},
	}
}

// buildFixtureDashboardFacts builds a DashboardFactsV1 that joins cleanly with a
// UnifiedAnalysisOutputV1 fixture for publish-only integration tests (facts are
// not re-built by PublishRun). SKU index entries are taken from AI sku_focus so
// bundlevalidator's ProjectDashboard join succeeds without weakening validators.
func buildFixtureDashboardFacts(sellerID int64, metricDate time.Time, packHash string, out *contract.UnifiedAnalysisOutputV1) dashboardfacts.DashboardFactsV1 {
	dash := out.Dashboard
	facts := dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: sellerID,
		MetricDate:      metricDate.Format("2006-01-02"),
		PackID:          out.AnalysisMeta.PackID,
		PackHash:        packHash,
		SKUFactsIndex:   dashboardfacts.SKUFactsIndex{ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{}},
	}
	for _, kpi := range dash.SellerPulse.KPIs {
		facts.SellerPulse.KPIs = append(facts.SellerPulse.KPIs, dashboardfacts.KPICardFact{
			Key: kpi.Key, Label: kpi.Label, Value: kpi.Value, Hint: kpi.Hint,
		})
	}
	for _, p := range dash.SellerPulse.Trend7D {
		facts.SellerPulse.Trend7D = append(facts.SellerPulse.Trend7D, dashboardfacts.TrendDayPoint{
			Date: p.Date, Revenue: p.Revenue, Orders: p.Orders,
		})
	}
	if len(dash.SellerPulse.KPIs) > 0 {
		facts.DataCoverage.AccountMetricsDays = 7
	}
	facts.StockCoverage.Status = dash.StockCoverage.Status
	facts.StockCoverage.TotalSKUs = dash.StockCoverage.TotalSKUs
	for _, s := range dash.StockCoverage.Segments {
		facts.StockCoverage.Segments = append(facts.StockCoverage.Segments, dashboardfacts.StockCoverageSegment{
			Key: s.Key, Label: s.Label, SKUCount: s.SKUCount, Severity: s.Severity,
		})
	}
	if len(dash.StockCoverage.Segments) > 0 {
		facts.DataCoverage.StockSnapshotRows = 10
	}
	facts.AdEfficiency.Status = dash.AdEfficiency.Status
	if dash.AdEfficiency.Summary != nil {
		facts.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
			CurrentDRRPct:          dash.AdEfficiency.Summary.CurrentDRRPct,
			PreviousDRRPct:         dash.AdEfficiency.Summary.PreviousDRRPct,
			AverageDRR7dPct:        dash.AdEfficiency.Summary.AverageDRR7dPct,
			TotalSpendToday:        dash.AdEfficiency.Summary.TotalSpendToday,
			TotalRevenueToday:      dash.AdEfficiency.Summary.TotalRevenueToday,
			TotalOrdersToday:       dash.AdEfficiency.Summary.TotalOrdersToday,
			CampaignsHighDRRCount:  dash.AdEfficiency.Summary.CampaignsHighDRRCount,
			CampaignsNoOrdersCount: dash.AdEfficiency.Summary.CampaignsNoOrdersCount,
			HighDRRSpend:           dash.AdEfficiency.Summary.HighDRRSpend,
			SpendWithoutOrders:     dash.AdEfficiency.Summary.SpendWithoutOrders,
		}
		facts.DataCoverage.AdMetricsDays = 7
	}
	facts.Funnel.Status = dash.SalesFunnel.Status
	for _, s := range dash.SalesFunnel.Stages {
		facts.Funnel.Stages = append(facts.Funnel.Stages, dashboardfacts.FunnelStage{
			Key: s.Key, Label: s.Label, Value: s.Value,
			ConversionFromPrevious: s.ConversionFromPrevious, ConversionFromTop: s.ConversionFromTop,
			PreviousDayValue: s.PreviousDayValue, ChangeVsPreviousDayPct: s.ChangeVsPreviousDayPct,
		})
	}
	if len(dash.SalesFunnel.Stages) > 0 {
		facts.DataCoverage.FunnelAvailable = true
	}
	nextProductID := int64(1)
	addSKU := func(sku *int64, offerID *string, productName string) {
		rec := dashboardfacts.SKUFactRecord{
			OzonProductID:  nextProductID,
			SKU:            sku,
			OfferID:        offerID,
			ProductName:    &productName,
			Revenue:        1000,
			Revenue7d:      1000,
			Orders:         1,
			Orders7d:       1,
			StockAvailable: 10,
		}
		facts.SKUFactsIndex.ByOzonProductID[fmt.Sprintf("%d", nextProductID)] = rec
		nextProductID++
	}
	for _, c := range dash.SKUFocus.OrderDrop {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.StockRisk {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.NoSalesWithStock {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.SlowTurnover {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	for _, c := range dash.SKUFocus.KeySKUs {
		addSKU(c.SKU, c.OfferID, c.ProductName)
	}
	// Cohort membership owns KeySKU projection; seed Revenue7d above so healthy
	// accounts (key_skus only, empty risk tabs) still project sku_focus entries.
	facts.KeySKUCohort = dashboardfacts.BuildKeySKUCohortFacts(facts.SKUFactsIndex)
	if len(facts.SKUFactsIndex.ByOzonProductID) > 0 {
		facts.DataCoverage.SKUMetricsDays = 7
	}
	return facts
}

type memoryPackObjectStore struct {
	objects map[string][]byte
}

func newMemoryPackObjectStore() *memoryPackObjectStore {
	return &memoryPackObjectStore{objects: make(map[string][]byte)}
}

func (s *memoryPackObjectStore) Get(ctx context.Context, objectKey string) ([]byte, error) {
	raw, ok := s.objects[objectKey]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte(nil), raw...), nil
}

func (s *memoryPackObjectStore) Put(objectKey string, data []byte) {
	s.objects[objectKey] = append([]byte(nil), data...)
}

type memoryArtifactStore struct {
	repo *memoryRepository
}

func (m *memoryArtifactStore) SaveProviderRequestSummary(ctx context.Context, runID int64, summary providerRequestSummary) error {
	raw, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedProviderRequestSummary,
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func (m *memoryArtifactStore) SaveProviderUsage(ctx context.Context, runID int64, usage providerUsageArtifact) error {
	raw, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedProviderUsage,
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func (m *memoryArtifactStore) SaveProviderTerminalAttempt(ctx context.Context, runID int64, attempt providerTerminalAttemptArtifact, rawJSON []byte) error {
	store := &ArtifactStore{repo: &Repository{}}
	// Use the same persistence shape via direct CreateArtifact calls so tests
	// can inspect payloads without needing a real Repository.q.
	_ = store
	if len(rawJSON) > 0 {
		var rawPayload any
		if err := json.Unmarshal(rawJSON, &rawPayload); err != nil {
			rawPayload = map[string]string{"raw_text": string(rawJSON)}
		}
		raw, err := json.Marshal(rawPayload)
		if err != nil {
			return err
		}
		if _, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
			PipelineRunID: runID,
			Kind:          ArtifactUnifiedProviderResponseRaw,
			Storage:       "db",
			PayloadJson:   raw,
		}); err != nil {
			return err
		}
	}
	summaryRaw, err := json.Marshal(providerResponseSummary{
		Attempt:               attempt.Attempt,
		ResponseID:            attempt.ResponseID,
		Status:                attempt.Status,
		RequestID:             attempt.RequestID,
		IncompleteReason:      attempt.IncompleteReason,
		AssistantMessageCount: attempt.AssistantMessageCount,
		OutputTextBytes:       attempt.OutputTextBytes,
		OutputItemTypes:       attempt.OutputItemTypes,
		CodeInterpreterCalls:  attempt.CodeInterpreterCalls,
		RawBytes:              len(rawJSON),
		RawStoredInDB:         len(rawJSON) > 0,
	})
	if err != nil {
		return err
	}
	if _, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID, Kind: ArtifactUnifiedProviderResponseSummary, Storage: "db", PayloadJson: summaryRaw,
	}); err != nil {
		return err
	}
	if err := m.SaveProviderUsage(ctx, runID, providerUsageArtifact{
		Attempt: attempt.Attempt, ResponseID: attempt.ResponseID,
		InputTokens: attempt.InputTokens, OutputTokens: attempt.OutputTokens,
		TotalTokens: attempt.TotalTokens, LatencyMs: attempt.LatencyMs,
	}); err != nil {
		return err
	}
	attempt.RawBytes = len(rawJSON)
	attemptRaw, err := json.Marshal(attempt)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID, Kind: ArtifactUnifiedProviderTerminalAttempt, Storage: "db", PayloadJson: attemptRaw,
	})
	return err
}

func (m *memoryArtifactStore) ListArtifactsByKind(ctx context.Context, runID int64, kind string) [][]byte {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return nil
	}
	out := make([][]byte, 0)
	for _, art := range arts {
		if art.Kind == kind {
			out = append(out, append([]byte(nil), art.PayloadJson...))
		}
	}
	return out
}

func (m *memoryArtifactStore) SavePackReference(ctx context.Context, runID int64, ref packReferenceArtifact) error {
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{PipelineRunID: runID, Kind: ArtifactUnifiedAnalysisPackReference, Storage: "db"})
	return err
}

func (m *memoryArtifactStore) SavePackGenerationValidation(ctx context.Context, runID int64, report pack.GenerationValidationReport) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedPackGenerationValidation,
		Storage:       "db",
		PayloadJson:   raw,
		Hash:          nullableText(report.PackHash),
	})
	return err
}

func (m *memoryArtifactStore) SaveDashboardFacts(ctx context.Context, runID int64, facts dashboardfacts.DashboardFactsV1) error {
	raw, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedDashboardFacts,
		Storage:       "db",
		PayloadJson:   raw,
		Hash:          nullableText(facts.FactsHash),
	})
	return err
}

func (m *memoryArtifactStore) LoadDashboardFacts(ctx context.Context, runID int64) (dashboardfacts.DashboardFactsV1, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return dashboardfacts.DashboardFactsV1{}, false
	}
	for i := len(arts) - 1; i >= 0; i-- {
		if arts[i].Kind != ArtifactUnifiedDashboardFacts {
			continue
		}
		var out dashboardfacts.DashboardFactsV1
		if err := json.Unmarshal(arts[i].PayloadJson, &out); err != nil {
			return dashboardfacts.DashboardFactsV1{}, false
		}
		if hash := textOrEmpty(arts[i].Hash); hash != "" {
			out.FactsHash = hash
		}
		return out, true
	}
	return dashboardfacts.DashboardFactsV1{}, false
}

func (m *memoryArtifactStore) SaveSourceReconciliation(ctx context.Context, runID int64, report sourcerecon.Report) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedSourceReconciliation,
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func (m *memoryArtifactStore) LoadSourceReconciliation(ctx context.Context, runID int64) (sourcerecon.Report, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return sourcerecon.Report{}, false
	}
	for i := len(arts) - 1; i >= 0; i-- {
		if arts[i].Kind != ArtifactUnifiedSourceReconciliation || len(arts[i].PayloadJson) == 0 {
			continue
		}
		var out sourcerecon.Report
		if err := json.Unmarshal(arts[i].PayloadJson, &out); err != nil {
			return sourcerecon.Report{}, false
		}
		return out, true
	}
	return sourcerecon.Report{}, false
}

func (m *memoryArtifactStore) SaveDateResolution(ctx context.Context, runID int64, art DateResolutionArtifact) error {
	raw, err := json.Marshal(art)
	if err != nil {
		return err
	}
	_, err = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedAnalysisDateResolution,
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func (m *memoryArtifactStore) LoadDateResolution(ctx context.Context, runID int64) (DateResolutionArtifact, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return DateResolutionArtifact{}, false
	}
	for i := len(arts) - 1; i >= 0; i-- {
		if arts[i].Kind != ArtifactUnifiedAnalysisDateResolution {
			continue
		}
		var out DateResolutionArtifact
		if err := json.Unmarshal(arts[i].PayloadJson, &out); err != nil {
			return DateResolutionArtifact{}, false
		}
		return out, true
	}
	return DateResolutionArtifact{}, false
}

func (m *memoryArtifactStore) SaveCleanup(ctx context.Context, runID int64, cleanup cleanupArtifact) error {
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{PipelineRunID: runID, Kind: ArtifactUnifiedProviderCleanup, Storage: "db"})
	return err
}

func (m *memoryArtifactStore) SaveProviderResponse(ctx context.Context, runID int64, responseID, status, outputText string, rawJSON []byte, ciCalls int) error {
	if len(rawJSON) > 0 {
		_, _ = m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
			PipelineRunID: runID,
			Kind:          ArtifactUnifiedProviderResponseRaw,
			Storage:       "db",
			PayloadJson:   rawJSON,
		})
	}
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{PipelineRunID: runID, Kind: ArtifactUnifiedProviderResponseSummary, Storage: "db"})
	return err
}

func (m *memoryArtifactStore) SaveOutputRaw(ctx context.Context, runID int64, raw []byte) error {
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedAnalysisOutputRaw,
		Storage:       "db",
		PayloadJson:   raw,
		Hash:          nullableText(hashBytes(raw)),
	})
	return err
}

func (m *memoryArtifactStore) SaveOutputValidation(ctx context.Context, runID int64, report outputValidationArtifact) error {
	raw, _ := json.Marshal(report)
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedAnalysisOutputValidation,
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func (m *memoryArtifactStore) SaveOutputValidated(ctx context.Context, runID int64, canonical []byte, outputHash string) error {
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedAnalysisOutputValidated,
		Storage:       "db",
		PayloadJson:   canonical,
		Hash:          nullableText(outputHash),
	})
	return err
}

func (m *memoryArtifactStore) SaveOutputSummary(ctx context.Context, runID int64, summary outputSummaryArtifact) error {
	raw, _ := json.Marshal(summary)
	_, err := m.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          ArtifactUnifiedAnalysisOutputSummary,
		Storage:       "db",
		PayloadJson:   raw,
		Hash:          nullableText(summary.OutputHash),
	})
	return err
}

func (m *memoryArtifactStore) LoadProviderResponseRaw(ctx context.Context, runID int64) ([]byte, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return nil, false
	}
	for i := len(arts) - 1; i >= 0; i-- {
		if arts[i].Kind == ArtifactUnifiedProviderResponseRaw {
			return arts[i].PayloadJson, true
		}
	}
	return nil, false
}

func (m *memoryArtifactStore) LoadValidatedMeta(ctx context.Context, runID int64) (storedValidatedMeta, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return storedValidatedMeta{}, false
	}
	meta := storedValidatedMeta{}
	found := false
	for _, art := range arts {
		if art.Kind == ArtifactUnifiedAnalysisOutputValidated {
			meta.OutputHash = textOrEmpty(art.Hash)
			found = true
		}
		if art.Kind == ArtifactUnifiedAnalysisOutputSummary {
			var summary outputSummaryArtifact
			if json.Unmarshal(art.PayloadJson, &summary) == nil {
				meta.Outcome = summary.Outcome
				meta.ActionCount = summary.ActionCount
				meta.TopActionCount = summary.TopActionCount
				if summary.OutputHash != "" {
					meta.OutputHash = summary.OutputHash
				}
			}
		}
	}
	return meta, found && meta.OutputHash != ""
}

func (m *memoryArtifactStore) LoadOutputSummary(ctx context.Context, runID int64) (outputSummaryArtifact, bool) {
	arts, err := m.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return outputSummaryArtifact{}, false
	}
	for i := len(arts) - 1; i >= 0; i-- {
		if arts[i].Kind != ArtifactUnifiedAnalysisOutputSummary {
			continue
		}
		var summary outputSummaryArtifact
		if json.Unmarshal(arts[i].PayloadJson, &summary) != nil {
			return outputSummaryArtifact{}, false
		}
		return summary, true
	}
	return outputSummaryArtifact{}, false
}

func testConfig() Config {
	return Config{
		Enabled:                 true,
		Provider:                ProviderNameOpenAI,
		Model:                   "fake-model",
		MemoryLimit:             "4g",
		AnalysisMaxOutputTokens: 32768,
		SmokeMaxOutputTokens:    1024,
		MaxOutputTokens:         1024,
		ProviderMaxAttempts:     2,
		CleanupEnabled:          true,
		OutputValidationEnabled: true,
		PromptProfile:           "smoke",
	}
}

func newTestService(repo *memoryRepository, packs PackBuilderService, fake *provider.FakeProvider) *Service {
	return NewServiceWithStore(repo, packs, fake, &memoryArtifactStore{repo: repo}, newMemoryPackObjectStore(), testConfig())
}
