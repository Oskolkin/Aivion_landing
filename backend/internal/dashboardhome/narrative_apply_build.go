package dashboardhome

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// buildDataPackNarrativeBuildForApply mirrors NarrativeService.buildNarrativeContextForSeller
// datapack path so freshness checks on GET /dashboard/home match stored ai_dashboard_narratives rows.
func (s *Service) buildDataPackNarrativeBuildForApply(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	dashboardDTO analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	previousSnapshot *accountMetricsSnapshot,
) (dashboardNarrativeBuildResult, error) {
	packSvc := aidatapack.NewService(s.pool)
	pack, err := packSvc.Build(ctx, sellerAccountID, metricDate)
	if err != nil {
		return dashboardNarrativeBuildResult{}, fmt.Errorf("data pack build: %w", err)
	}
	if _, chunkErr := BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions()); chunkErr != nil {
		return dashboardNarrativeBuildResult{}, fmt.Errorf("data pack chunks: %w", chunkErr)
	}
	legacyInput := dataPackNarrativeLegacyInput{
		DTO:          dashboardDTO,
		AlertRows:    alertRows,
		Recs:         recs,
		PreviousSnap: previousSnapshot,
	}
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, resp, legacyInput)
	if err != nil {
		return dashboardNarrativeBuildResult{}, fmt.Errorf("data pack narrative context: %w", err)
	}
	audit := aidatapack.AuditDataPack(pack)
	build.DataPackAuditValid = audit.Valid
	build.ContextSource = ContextSourceSellerAIDataPackChunks
	return build, nil
}

func shouldUseDataPackNarrativeBuild(stored *storedDashboardNarrative) bool {
	return stored != nil &&
		stored.Status == "completed" &&
		stored.PromptVersion == DashboardNarrativePromptVersion
}
