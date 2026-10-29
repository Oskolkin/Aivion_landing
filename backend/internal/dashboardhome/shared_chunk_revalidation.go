package dashboardhome

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// RevalidateSharedChunkFromRawResponses rebuilds ChunkAnalysisRunResult from saved raw bodies without AI calls.
func RevalidateSharedChunkFromRawResponses(
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	rawCalls []ChunkRawCallInput,
) (ChunkAnalysisRunResult, ChunkCandidateRejectionReport, error) {
	if pack == nil || chunkSet == nil {
		return ChunkAnalysisRunResult{}, ChunkCandidateRejectionReport{}, fmt.Errorf("pack or chunk set is nil")
	}
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		return ChunkAnalysisRunResult{}, ChunkCandidateRejectionReport{}, err
	}
	chunkByID := map[string]aidatapack.DataPackChunk{}
	for _, ch := range chunkSet.Chunks {
		chunkByID[ch.ChunkID] = ch
	}
	result := ChunkAnalysisRunResult{
		SellerAccountID: pack.SellerContext.SellerAccountID,
		MetricDate:      pack.SellerContext.MetricDate,
		ChunkCount:      len(chunkSet.Chunks),
		AnalysisStatus:  ChunkAnalysisStatusCompleted,
	}
	aggregateReport := ChunkCandidateRejectionReport{
		RejectionsByCode:  map[string]int{},
		RejectionsByChunk: map[string]int{},
	}
	for _, call := range rawCalls {
		ch, ok := chunkByID[call.ChunkID]
		if !ok {
			continue
		}
		result.ChunkHashes = append(result.ChunkHashes, ch.ChunkHash)
		callUsage := ChunkCallUsage{
			ChunkID:     call.ChunkID,
			ChunkType:   ch.ChunkType,
			RawBody:     call.RawBody,
			Status:      ChunkCallStatusFailed,
			UsageSource: "revalidation",
		}
		if strings.TrimSpace(call.RawBody) == "" {
			callUsage.Error = "missing_raw_body"
			result.Failed++
			result.FailedChunkIDs = append(result.FailedChunkIDs, call.ChunkID)
			result.ChunkCalls = append(result.ChunkCalls, callUsage)
			continue
		}
		analysis, err := parseDashboardChunkAnalysis(call.RawBody)
		if err != nil {
			repairedJSON, _, repairErr := repairRawDashboardNarrativeJSON(call.RawBody)
			if repairErr == nil {
				analysis, err = parseDashboardChunkAnalysis(repairedJSON)
			}
		}
		if err != nil {
			callUsage.Error = err.Error()
			callUsage.ParseStatus = "invalid_json"
			result.Failed++
			result.ParseErrors++
			result.FailedChunkIDs = append(result.FailedChunkIDs, call.ChunkID)
			result.ChunkCalls = append(result.ChunkCalls, callUsage)
			continue
		}
		if analysis.ChunkID == "" {
			analysis.ChunkID = ch.ChunkID
		}
		if analysis.SchemaVersion == DashboardChunkAnalysisSchemaVersionV3 {
			mapping := BuildChunkRefMapping(ch)
			ResolveChunkAnalysisRefs(analysis, mapping)
		}
		analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(ch.ChunkID, analysis.ActionCandidates)
		v := ValidateDashboardChunkAnalysis(ch, *analysis, pack)
		chunkReport := buildChunkCandidateRejectionReportFromValidation(ch.ChunkID, analysis.ActionCandidates, v)
		mergeCandidateRejectionReport(&aggregateReport, chunkReport)

		if v.Valid {
			callUsage.Status = ChunkCallStatusCompleted
			result.Completed++
			if analysis.AnalysisStatus == "insufficient_data" && len(v.Signals) == 0 {
				result.InsufficientDataCount++
			}
			result.ValidatedSignals = append(result.ValidatedSignals, v.Signals...)
			result.ValidatedSignalCount += len(v.Signals)
			result.ValidatedActionCandidates = append(result.ValidatedActionCandidates, v.ActionCandidates...)
			result.ValidatedCandidateCount += len(v.ActionCandidates)
		} else {
			callUsage.Status = ChunkCallStatusValidationFailed
			callUsage.Error = strings.Join(v.HardErrors, ", ")
			result.Failed++
			result.ValidationFailedCount++
			result.FailedChunkIDs = append(result.FailedChunkIDs, call.ChunkID)
		}
		result.RejectedCandidateCount += v.RejectedCandidateCount
		result.RejectedSignalCount += v.RejectedSignalCount
		result.InventedFactIDsRejected += v.InventedFactRejected
		result.ChunkCalls = append(result.ChunkCalls, callUsage)
	}
	if result.Failed > 0 && result.Completed > 0 {
		result.AnalysisStatus = ChunkAnalysisStatusPartial
	} else if result.Failed > 0 {
		result.AnalysisStatus = ChunkAnalysisStatusFailed
	}
	aggregateReport.CandidateTotal = aggregateReport.Accepted + aggregateReport.Rejected
	return result, aggregateReport, nil
}

// ChunkRawCallInput is one saved chunk provider response for offline revalidation.
type ChunkRawCallInput struct {
	ChunkID string
	RawBody string
}

func buildChunkCandidateRejectionReportFromValidation(
	chunkID string,
	raw []actions.AIFirstActionCandidate,
	v ChunkValidationResult,
) ChunkCandidateRejectionReport {
	report := ChunkCandidateRejectionReport{
		Accepted:          len(v.ActionCandidates),
		Rejected:          v.RejectedCandidateCount,
		CandidateTotal:    len(raw),
		RejectionsByCode:  map[string]int{},
		RejectionsByChunk: map[string]int{chunkID: v.RejectedCandidateCount},
	}
	for _, msg := range v.RepairableErrors {
		if !strings.Contains(msg, "candidate_") {
			continue
		}
		parts := strings.Split(msg, ":")
		if len(parts) < 2 {
			continue
		}
		code := parts[len(parts)-1]
		if strings.HasPrefix(code, "unknown_") || strings.HasSuffix(code, "_removed") {
			report.Repaired++
		}
		report.RejectionsByCode[code]++
	}
	return report
}

func mergeCandidateRejectionReport(dst *ChunkCandidateRejectionReport, src ChunkCandidateRejectionReport) {
	if dst == nil {
		return
	}
	dst.CandidateTotal += src.CandidateTotal
	dst.Accepted += src.Accepted
	dst.Rejected += src.Rejected
	dst.Repaired += src.Repaired
	for k, v := range src.RejectionsByCode {
		dst.RejectionsByCode[k] += v
	}
	for k, v := range src.RejectionsByChunk {
		dst.RejectionsByChunk[k] += v
	}
	dst.Rejections = append(dst.Rejections, src.Rejections...)
}
