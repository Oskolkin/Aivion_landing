package aipipeline

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

const defaultSharedChunkMinSuccessPercent = 75

const (
	chunkFamilyAccountFunnel    = "account_funnel"
	chunkFamilySKURollups       = "sku_rollups"
	chunkFamilyStockPositions   = "stock_positions"
	chunkFamilyCampaignRollups  = "campaign_rollups"
	chunkIDAccountFunnelQuality = "account_funnel_quality_constraints"
)

// FamilyCoverage summarizes usable chunk coverage for one critical family.
type FamilyCoverage struct {
	TotalChunks  int  `json:"total_chunks"`
	UsableChunks int  `json:"usable_chunks"`
	Required     int  `json:"required_minimum"`
	Passed       bool `json:"passed"`
}

// SharedChunkCoverageResult is a deterministic coverage assessment before downstream stages.
type SharedChunkCoverageResult struct {
	Passed         bool                      `json:"passed"`
	TotalChunks    int                       `json:"total_chunks"`
	UsableChunks   int                       `json:"usable_chunks"`
	FailedChunks   int                       `json:"failed_chunks"`
	UsablePercent  int                       `json:"usable_percent"`
	FailureReasons []string                  `json:"failure_reasons,omitempty"`
	FamilyCoverage map[string]FamilyCoverage `json:"family_coverage,omitempty"`
}

func sharedChunkMinSuccessPercent() int {
	raw := strings.TrimSpace(os.Getenv("AI_SHARED_CHUNK_MIN_SUCCESS_PERCENT"))
	if raw == "" {
		return defaultSharedChunkMinSuccessPercent
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 || v > 100 {
		return defaultSharedChunkMinSuccessPercent
	}
	return v
}

func minUsableChunksForFamily(total, minPercent int) int {
	if total <= 0 {
		return 0
	}
	if total == 1 {
		return 1
	}
	required := (total*minPercent + 99) / 100
	if required < 1 {
		return 1
	}
	if required > total {
		return total
	}
	return required
}

func chunkCoverageFamily(ch aidatapack.DataPackChunk) string {
	switch {
	case ch.ChunkID == chunkIDAccountFunnelQuality:
		return chunkFamilyAccountFunnel
	case ch.SourceTable == "sku_rollups":
		return chunkFamilySKURollups
	case ch.SourceTable == "stock_positions":
		return chunkFamilyStockPositions
	case ch.SourceTable == "campaign_rollups":
		return chunkFamilyCampaignRollups
	default:
		return ""
	}
}

func usableChunkIDSet(run *dashboardhome.ChunkAnalysisRunResult) map[string]struct{} {
	failed := make(map[string]struct{}, len(run.FailedChunkIDs))
	for _, id := range run.FailedChunkIDs {
		failed[id] = struct{}{}
	}
	return failed
}

func isChunkUsable(chunkID string, failed map[string]struct{}) bool {
	_, failedChunk := failed[chunkID]
	return !failedChunk
}

// EvaluateSharedChunkCoverage checks overall and critical-family coverage after validation.
func EvaluateSharedChunkCoverage(
	chunkSet *aidatapack.ChunkSet,
	run *dashboardhome.ChunkAnalysisRunResult,
) SharedChunkCoverageResult {
	out := SharedChunkCoverageResult{
		FamilyCoverage: map[string]FamilyCoverage{},
	}
	if chunkSet == nil || run == nil {
		out.FailureReasons = []string{"missing chunk set or run result"}
		return out
	}

	minPercent := sharedChunkMinSuccessPercent()
	out.TotalChunks = run.ChunkCount
	out.UsableChunks = run.Completed
	out.FailedChunks = run.Failed
	if out.TotalChunks > 0 {
		out.UsablePercent = out.UsableChunks * 100 / out.TotalChunks
	}

	failed := usableChunkIDSet(run)
	familyChunks := map[string][]aidatapack.DataPackChunk{}
	for _, ch := range chunkSet.Chunks {
		if family := chunkCoverageFamily(ch); family != "" {
			familyChunks[family] = append(familyChunks[family], ch)
		}
	}

	evaluateFamily := func(family string, required bool) {
		chunks := familyChunks[family]
		if len(chunks) == 0 {
			return
		}
		usable := 0
		for _, ch := range chunks {
			if isChunkUsable(ch.ChunkID, failed) {
				usable++
			}
		}
		requiredMin := minUsableChunksForFamily(len(chunks), minPercent)
		if family == chunkFamilyAccountFunnel {
			requiredMin = 1
		}
		passed := usable >= requiredMin
		out.FamilyCoverage[family] = FamilyCoverage{
			TotalChunks:  len(chunks),
			UsableChunks: usable,
			Required:     requiredMin,
			Passed:       passed,
		}
		if required && !passed {
			out.FailureReasons = append(out.FailureReasons,
				fmt.Sprintf("family %s usable=%d/%d required>=%d", family, usable, len(chunks), requiredMin))
		}
	}

	if out.UsablePercent < minPercent {
		out.FailureReasons = append(out.FailureReasons,
			fmt.Sprintf("usable=%d/%d (%d%%); required>=%d%%", out.UsableChunks, out.TotalChunks, out.UsablePercent, minPercent))
	}

	evaluateFamily(chunkFamilyAccountFunnel, true)
	evaluateFamily(chunkFamilySKURollups, true)
	evaluateFamily(chunkFamilyStockPositions, true)
	if len(familyChunks[chunkFamilyCampaignRollups]) > 0 {
		evaluateFamily(chunkFamilyCampaignRollups, true)
	}

	out.Passed = len(out.FailureReasons) == 0
	return out
}

func formatSharedChunkCoverageError(coverage SharedChunkCoverageResult) string {
	missingFamilies := make([]string, 0)
	for family, fc := range coverage.FamilyCoverage {
		if !fc.Passed && fc.TotalChunks > 0 {
			missingFamilies = append(missingFamilies, family)
		}
	}
	msg := fmt.Sprintf("shared chunk coverage insufficient: usable=%d/%d (%d%%); required>=%d%%",
		coverage.UsableChunks, coverage.TotalChunks, coverage.UsablePercent, sharedChunkMinSuccessPercent())
	if len(missingFamilies) > 0 {
		msg += "; missing critical families=" + strings.Join(missingFamilies, ",")
	}
	if len(coverage.FailureReasons) > 0 {
		msg += "; details=" + strings.Join(coverage.FailureReasons, "; ")
	}
	return msg
}
