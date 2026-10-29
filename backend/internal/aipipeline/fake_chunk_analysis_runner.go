package aipipeline

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// DeterministicChunkAnalysisRunner produces stable chunk analysis output for fake runtime and tests.
type DeterministicChunkAnalysisRunner struct{}

var _ ChunkAnalysisRunner = (*DeterministicChunkAnalysisRunner)(nil)

func NewDeterministicChunkAnalysisRunner() *DeterministicChunkAnalysisRunner {
	return &DeterministicChunkAnalysisRunner{}
}

// fakeChunkSourceTableActionType maps chunk source tables to action types for deterministic fake/test data only.
var fakeChunkSourceTableActionType = map[string]actions.ActionType{
	"stock_positions":        actions.ActionTypeStockReplenishment,
	"sku_rollups":            actions.ActionTypeSKUOrdersDrop,
	"campaign_rollups":       actions.ActionTypeAdHighDRRReview,
	"category_rollups":       actions.ActionTypeCategoryUnderperform,
	"account_funnel_finance": actions.ActionTypeFunnelWeakTransition,
}

func (r *DeterministicChunkAnalysisRunner) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	_ = opts
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pack == nil {
		return nil, errors.New("seller AI data pack is nil")
	}
	if chunkSet == nil {
		return nil, errors.New("chunk set is nil")
	}
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		return nil, err
	}

	result := dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID: pack.SellerContext.SellerAccountID,
		MetricDate:      pack.SellerContext.MetricDate,
		ChunkCount:      len(chunkSet.Chunks),
		AnalysisStatus:  dashboardhome.ChunkAnalysisStatusCompleted,
	}

	for i, chunk := range chunkSet.Chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.ChunkHashes = append(result.ChunkHashes, chunk.ChunkHash)

		inTok := 100 * (i + 1)
		outTok := 10 * (i + 1)

		analysis := buildFakeChunkAnalysis(chunk)
		result.Analyses = append(result.Analyses, analysis)
		result.InputTokens += inTok
		result.OutputTokens += outTok

		callUsage := dashboardhome.ChunkCallUsage{
			ChunkID:      chunk.ChunkID,
			ChunkType:    chunk.ChunkType,
			Model:        "fake-chunk-analysis-deterministic",
			InputTokens:  inTok,
			OutputTokens: outTok,
			LatencyMs:    0,
			Status:       dashboardhome.ChunkCallStatusCompleted,
			UsageSource:  "fake",
		}

		if analysis.AnalysisStatus == "insufficient_data" {
			result.Completed++
			result.InsufficientDataCount++
			result.ChunkCalls = append(result.ChunkCalls, callUsage)
			continue
		}

		validation := dashboardhome.ValidateDashboardChunkAnalysis(chunk, analysis, pack)
		result.InventedFactIDsRejected += validation.InventedFactRejected
		result.RejectedSignalCount += validation.RejectedSignalCount
		result.RejectedCandidateCount += validation.RejectedCandidateCount

		if len(validation.Signals) > 0 {
			result.ValidatedSignals = append(result.ValidatedSignals, validation.Signals...)
			result.ValidatedSignalCount += len(validation.Signals)
		}
		if len(validation.ActionCandidates) > 0 {
			result.ValidatedActionCandidates = append(result.ValidatedActionCandidates, validation.ActionCandidates...)
			result.ValidatedCandidateCount += len(validation.ActionCandidates)
		}
		result.Completed++
		result.ChunkCalls = append(result.ChunkCalls, callUsage)
	}

	if result.Completed+result.Failed != result.ChunkCount {
		result.Completed = result.ChunkCount - result.Failed
	}
	if result.Failed > 0 && result.Completed > 0 {
		result.AnalysisStatus = dashboardhome.ChunkAnalysisStatusPartial
	} else if result.Failed > 0 {
		result.AnalysisStatus = dashboardhome.ChunkAnalysisStatusFailed
	}

	return &result, nil
}

func buildFakeChunkAnalysis(chunk aidatapack.DataPackChunk) dashboardhome.DashboardChunkAnalysis {
	analysis := dashboardhome.DashboardChunkAnalysis{
		SchemaVersion:    dashboardhome.DashboardChunkAnalysisSchemaVersionV2,
		ChunkID:          chunk.ChunkID,
		ChunkType:        chunk.ChunkType,
		AnalysisStatus:   "insufficient_data",
		Signals:          nil,
		ActionCandidates: []actions.AIFirstActionCandidate{},
	}

	actionType, hasAction := fakeChunkSourceTableActionType[chunk.SourceTable]
	entityIDs := sortedEntityIDsWithFacts(chunk, 3)
	factIDs := sortedFactIDsForEntities(chunk, entityIDs, 12)
	if len(entityIDs) == 0 || len(factIDs) == 0 {
		return analysis
	}

	signalID := chunk.ChunkID + ":fake-signal:01"
	analysis.AnalysisStatus = "completed"
	analysis.Signals = []dashboardhome.DashboardChunkSignal{{
		SignalID:       signalID,
		SignalType:     dashboardhome.ChunkSignalRiskCandidate,
		EntityIDs:      append([]string(nil), entityIDs...),
		FactIDs:        append([]string(nil), factIDs...),
		Observation:    fakeSignalObservation(chunk.SourceTable),
		WhyItMayMatter: "Детерминированный fake-сигнал для тестов.",
	}}

	if !hasAction {
		return analysis
	}
	category, ok := actions.ActionCategoryForType(actionType)
	if !ok {
		return analysis
	}

	affected := make([]actions.AIFirstAffectedEntityRef, 0, len(entityIDs))
	for _, eid := range entityIDs {
		ref := actions.AIFirstAffectedEntityRef{EntityID: eid}
		if ent, ok := chunk.LocalEntityCatalog[eid]; ok {
			ref.EntityType = ent.EntityType
			if strings.TrimSpace(ent.Name) != "" {
				ref.Name = ent.Name
			}
		}
		affected = append(affected, ref)
	}

	analysis.ActionCandidates = []actions.AIFirstActionCandidate{{
		CandidateID:       chunk.ChunkID + ":candidate:01",
		ChunkID:           chunk.ChunkID,
		Type:              actionType,
		Category:          category,
		LocalPriority:     actions.ActionPriorityImportant,
		Problem:           fakeCandidateProblem(chunk.SourceTable),
		RecommendedAction: fakeCandidateRecommendation(chunk.SourceTable),
		WhyActionable:     "Детерминированный fake-кандидат для тестов.",
		AffectedEntities:  affected,
		EvidenceFactIDs:   append([]string(nil), factIDs...),
		SourceSignalIDs:   []string{signalID},
	}}

	return analysis
}

func sortedEntityIDsWithFacts(chunk aidatapack.DataPackChunk, max int) []string {
	if len(chunk.LocalEntityCatalog) == 0 || len(chunk.LocalFactCatalog) == 0 {
		return nil
	}
	entityWithFact := map[string]struct{}{}
	for _, fact := range chunk.LocalFactCatalog {
		eid := strings.TrimSpace(fact.EntityID)
		if eid != "" {
			entityWithFact[eid] = struct{}{}
		}
	}
	ids := make([]string, 0, len(entityWithFact))
	for eid := range entityWithFact {
		if _, ok := chunk.LocalEntityCatalog[eid]; ok {
			ids = append(ids, eid)
		}
	}
	sort.Strings(ids)
	if max > 0 && len(ids) > max {
		ids = ids[:max]
	}
	return ids
}

func sortedFactIDsForEntities(chunk aidatapack.DataPackChunk, entityIDs []string, max int) []string {
	if len(chunk.LocalFactCatalog) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, eid := range entityIDs {
		allowed[eid] = struct{}{}
	}
	ids := make([]string, 0, len(chunk.LocalFactCatalog))
	for fid, fact := range chunk.LocalFactCatalog {
		eid := strings.TrimSpace(fact.EntityID)
		if len(allowed) > 0 {
			if _, ok := allowed[eid]; !ok {
				continue
			}
		}
		ids = append(ids, fid)
	}
	sort.Strings(ids)
	if max > 0 && len(ids) > max {
		ids = ids[:max]
	}
	return ids
}

func fakeSignalObservation(sourceTable string) string {
	switch sourceTable {
	case "stock_positions":
		return "У нескольких SKU покрытие запасом ниже порога."
	case "campaign_rollups":
		return "У нескольких кампаний повышенный расход при слабой отдаче."
	case "sku_rollups":
		return "У нескольких SKU снижение заказов за окно."
	case "category_rollups":
		return "У нескольких категорий слабая динамика продаж."
	case "account_funnel_quality_constraints":
		return "На одном из переходов воронки наблюдается просадка."
	default:
		return "Детерминированное наблюдение по локальным данным чанка."
	}
}

func fakeCandidateProblem(sourceTable string) string {
	switch sourceTable {
	case "stock_positions":
		return "У нескольких SKU с подтверждённым спросом запас ниже безопасного уровня."
	case "campaign_rollups":
		return "У нескольких кампаний расход растёт быстрее заказов."
	default:
		return "Детерминированная проблема по локальным данным чанка."
	}
}

func fakeCandidateRecommendation(sourceTable string) string {
	switch sourceTable {
	case "stock_positions":
		return "Проверить поставки и пополнить запас по выбранным SKU."
	case "campaign_rollups":
		return "Пересмотреть ставки и бюджет выбранных кампаний."
	default:
		return "Проверить выбранные объекты и скорректировать операционные действия."
	}
}
