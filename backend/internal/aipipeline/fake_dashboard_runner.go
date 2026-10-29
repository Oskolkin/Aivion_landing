package aipipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// FakeDashboardNarrativeStore persists offline fake dashboard narratives.
type FakeDashboardNarrativeStore interface {
	PersistFakeDashboardNarrative(ctx context.Context, params PersistFakeDashboardParams) (string, error)
}

// FakeDashboardRunner generates deterministic offline dashboard narratives for devtools/tests.
type FakeDashboardRunner struct {
	store FakeDashboardNarrativeStore
}

func NewFakeDashboardRunner(store FakeDashboardNarrativeStore) *FakeDashboardRunner {
	return &FakeDashboardRunner{store: store}
}

func (r *FakeDashboardRunner) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
) (*DashboardRunOutcome, error) {
	if r == nil || r.store == nil {
		return nil, fmt.Errorf("fake dashboard runner is not configured")
	}
	if opts.DataPackOverride == nil {
		return nil, fmt.Errorf("fake dashboard requires DataPackOverride")
	}
	if opts.GlobalContext == nil {
		return nil, fmt.Errorf("fake dashboard requires GlobalContext")
	}

	debugInput := buildDashboardArtifactInput(sellerAccountID, metricDate, opts, "fake")
	payload := buildFakeDashboardPayload(opts.GlobalContext, metricDate.Format("2006-01-02"))

	runID, err := r.store.PersistFakeDashboardNarrative(ctx, PersistFakeDashboardParams{
		SellerAccountID:   sellerAccountID,
		MetricDate:        metricDate,
		ForceRegenerate:   opts.ForceRegenerate,
		DataPackHash:      opts.DataPackHash,
		GlobalContextHash: opts.GlobalContextHash,
		PipelineRunID:     opts.PipelineRunID,
		Payload:           payload,
		HealthScore:       fakeDashboardHealthScore(opts.GlobalContext),
		Severity:          fakeDashboardSeverity(opts.GlobalContext),
	})
	if err != nil {
		return nil, err
	}

	result := &dashboardhome.NarrativeGenerationResult{
		Status:                         "completed",
		RunID:                          runID,
		DashboardSynthesisInputTokens:  400,
		DashboardSynthesisOutputTokens: 40,
		DashboardSynthesisRequestID:    "req_dash_123",
		DashboardSynthesisLatencyMs:    0,
		InputTokens:                    400,
		OutputTokens:                   40,
		DashboardChunkUsageIncluded:    false,
	}
	debugOutput := map[string]any{
		"model":   fakeDashboardModel,
		"mode":    "fake",
		"payload": payload,
	}
	return &DashboardRunOutcome{
		Result:      result,
		DebugInput:  debugInput,
		DebugOutput: debugOutput,
	}, nil
}
