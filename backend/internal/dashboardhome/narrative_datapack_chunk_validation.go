package dashboardhome

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// ValidateDashboardChunkAnalysis validates and repairs chunk analysis signal-by-signal.
func ValidateDashboardChunkAnalysis(chunk aidatapack.DataPackChunk, analysis DashboardChunkAnalysis, fullPack *aidatapack.SellerAIDataPack) ChunkValidationResult {
	result := ChunkValidationResult{Valid: true}
	if fullPack == nil {
		return invalidChunkResult(&result, "nil_data_pack")
	}
	if !IsSupportedDashboardChunkAnalysisSchemaVersion(analysis.SchemaVersion) {
		result.HardErrors = append(result.HardErrors, "invalid_schema_version")
	}
	if analysis.ChunkID != chunk.ChunkID {
		result.HardErrors = append(result.HardErrors, "chunk_id_mismatch")
	}
	if _, ok := allowedChunkAnalysisStatuses[analysis.AnalysisStatus]; !ok {
		result.HardErrors = append(result.HardErrors, "invalid_analysis_status")
	}
	if len(result.HardErrors) > 0 {
		result.Valid = false
		return result
	}

	allowedFacts := chunkLocalFactIDAllowlist(chunk)
	allowedEntities := chunkLocalEntityIDAllowlist(chunk)
	financeAvailable := fullPack.Tables.FinanceRollup.DataStatus == "available"
	priceConstraints := fullPack.Constraints.PriceConstraintsAvailable
	seenSignalIDs := map[string]struct{}{}
	inventedRejected := 0

	for i, sig := range analysis.Signals {
		if sig.SignalID == "" {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("signal_%d_missing_signal_id_rejected", i))
			continue
		}
		if _, ok := seenSignalIDs[sig.SignalID]; ok {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("duplicate_signal_id_%s_rejected", sig.SignalID))
			continue
		}
		seenSignalIDs[sig.SignalID] = struct{}{}
		if _, ok := allowedChunkSignalTypes[sig.SignalType]; !ok {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("signal_%d_invalid_type_rejected", i))
			continue
		}

		repaired, rejected, invented, warns := repairChunkSignal(sig, allowedFacts, allowedEntities, financeAvailable, priceConstraints, i)
		inventedRejected += invented
		result.RepairableErrors = append(result.RepairableErrors, warns...)
		if rejected {
			continue
		}
		if len(repaired.FactIDs) == 0 && repaired.SignalType != ChunkSignalDataQuality {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("signal_%d_no_valid_fact_ids_rejected", i))
			continue
		}
		if len(repaired.Observation) > 500 {
			result.RepairableErrors = append(result.RepairableErrors, fmt.Sprintf("signal_%d_observation_too_long_rejected", i))
			continue
		}
		result.Signals = append(result.Signals, repaired)
		if len(warns) > 0 {
			result.Repaired = true
		}
	}

	result.Valid = len(result.Signals) > 0 || analysis.AnalysisStatus == "insufficient_data"
	if !result.Valid && analysis.AnalysisStatus == "completed" {
		result.HardErrors = append(result.HardErrors, "no_valid_signals_after_validation")
	}
	result.InventedFactRejected = inventedRejected
	result.RejectedSignalCount = len(analysis.Signals) - len(result.Signals)

	if analysis.SchemaVersion == DashboardChunkAnalysisSchemaVersionV1 {
		result.ActionCandidates = []actions.AIFirstActionCandidate{}
		result.RejectedCandidateCount = 0
		return result
	}

	validSignalIDs := make(map[string]struct{}, len(result.Signals))
	for _, sig := range result.Signals {
		validSignalIDs[sig.SignalID] = struct{}{}
	}
	acceptedCandidates, candidateErrors, rejectedCandidates := validateChunkActionCandidates(
		chunk,
		analysis.ActionCandidates,
		validSignalIDs,
		allowedFacts,
		allowedEntities,
		chunk.LocalEntityCatalog,
	)
	result.RepairableErrors = append(result.RepairableErrors, candidateErrors...)
	if acceptedCandidates == nil {
		acceptedCandidates = []actions.AIFirstActionCandidate{}
	}
	result.ActionCandidates = acceptedCandidates
	result.RejectedCandidateCount = rejectedCandidates
	appendChunkSemanticWarnings(&result, analysis, chunk)
	return result
}

func appendChunkSemanticWarnings(result *ChunkValidationResult, analysis DashboardChunkAnalysis, chunk aidatapack.DataPackChunk) {
	metrics := aidatapack.MetricsPresentInChunk(chunk)
	for i, sig := range analysis.Signals {
		for _, issue := range aidatapack.DetectMetricSemanticMismatches(sig.Observation, metrics, fmt.Sprintf("signals[%d].observation", i)) {
			result.RepairableErrors = append(result.RepairableErrors,
				fmt.Sprintf("signal_%d:%s:%s", i, aidatapack.MetricSemanticMismatchCode, issue.ForbiddenPhrase))
		}
	}
	for i, c := range analysis.ActionCandidates {
		for _, text := range []string{c.Problem, c.RecommendedAction, c.WhyActionable} {
			for _, issue := range aidatapack.DetectMetricSemanticMismatches(text, metrics, fmt.Sprintf("candidates[%d]", i)) {
				result.RepairableErrors = append(result.RepairableErrors,
					fmt.Sprintf("candidate_%d:%s:%s", i+1, aidatapack.MetricSemanticMismatchCode, issue.ForbiddenPhrase))
			}
			for _, issue := range aidatapack.ValidateOrderDeclineGrounding(text, c.EvidenceFactIDs, fmt.Sprintf("candidates[%d]", i)) {
				result.RepairableErrors = append(result.RepairableErrors,
					fmt.Sprintf("candidate_%d:%s", i+1, issue.Code))
			}
		}
	}
}

func repairChunkSignal(
	sig DashboardChunkSignal,
	allowedFacts map[string]struct{},
	allowedEntities map[string]struct{},
	financeAvailable bool,
	priceConstraints bool,
	index int,
) (DashboardChunkSignal, bool, int, []string) {
	var warnings []string
	invented := 0
	prefix := fmt.Sprintf("signal_%d", index)

	cleanFacts := make([]string, 0, len(sig.FactIDs))
	for _, fid := range sig.FactIDs {
		fid = strings.TrimSpace(fid)
		if fid == "" {
			continue
		}
		if looksLikeEntityID(fid) && !looksLikeFactID(fid) {
			warnings = append(warnings, prefix+"_entity_id_in_fact_ids_removed:"+fid)
			continue
		}
		if _, ok := allowedFacts[fid]; !ok {
			invented++
			warnings = append(warnings, prefix+"_invented_fact_id_rejected:"+fid)
			continue
		}
		cleanFacts = append(cleanFacts, fid)
	}

	cleanEntities := make([]string, 0, len(sig.EntityIDs))
	for _, eid := range sig.EntityIDs {
		eid = strings.TrimSpace(eid)
		if eid == "" {
			continue
		}
		if looksLikeFactID(eid) && !looksLikeEntityID(eid) {
			warnings = append(warnings, prefix+"_fact_id_in_entity_ids_removed:"+eid)
			continue
		}
		if _, ok := allowedEntities[eid]; !ok {
			warnings = append(warnings, prefix+"_unknown_entity_id_rejected:"+eid)
			return sig, true, invented, warnings
		}
		cleanEntities = append(cleanEntities, eid)
	}

	sig.FactIDs = compactStringIDs(cleanFacts)
	sig.EntityIDs = compactStringIDs(cleanEntities)

	text := strings.ToLower(sig.Observation + " " + sig.WhyItMayMatter)
	for _, pat := range dataPackForbiddenSKULevelAdPatterns {
		if strings.Contains(text, pat) {
			return sig, true, invented, append(warnings, prefix+"_forbidden_sku_ad")
		}
	}
	if !financeAvailable {
		for _, pat := range dataPackForbiddenFinancePatterns {
			if strings.Contains(text, pat) {
				return sig, true, invented, append(warnings, prefix+"_finance_unavailable")
			}
		}
	}
	if !priceConstraints {
		for _, pat := range dataPackForbiddenPriceCutPatterns {
			if strings.Contains(text, pat) {
				return sig, true, invented, append(warnings, prefix+"_forbidden_price_cut")
			}
		}
	}
	return sig, false, invented, warnings
}

func compactStringIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func invalidChunkResult(r *ChunkValidationResult, code string) ChunkValidationResult {
	r.Valid = false
	r.HardErrors = append(r.HardErrors, code)
	r.Signals = nil
	return *r
}

func entityExistsInDataPack(pack *aidatapack.SellerAIDataPack, entityID string) bool {
	switch entityID {
	case "account:seller", "funnel:account":
		return true
	}
	if _, ok := pack.EntityRegistry.SKUs[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Campaigns[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Categories[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Warehouses[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.SKUGroups[entityID]; ok {
		return true
	}
	return false
}

func parseDashboardChunkAnalysis(raw string) (*DashboardChunkAnalysis, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty chunk analysis json")
	}
	repaired, _, err := repairRawDashboardNarrativeJSON(raw)
	if err == nil {
		raw = repaired
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return nil, err
	}
	if limRaw, ok := root["limitations_acknowledged"]; ok {
		root["limitations_acknowledged"] = normalizeStringArrayJSON(limRaw)
	}
	if notesRaw, ok := root["data_quality_notes"]; ok {
		root["data_quality_notes"] = normalizeDataQualityNotesJSON(notesRaw)
	}
	if signalsRaw, ok := root["signals"]; ok {
		root["signals"] = normalizeDashboardChunkSignalsJSON(signalsRaw)
	}
	if candidatesRaw, ok := root["action_candidates"]; ok {
		root["action_candidates"] = normalizeActionCandidatesJSON(candidatesRaw)
	}
	normalized, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	var analysis DashboardChunkAnalysis
	if err := json.Unmarshal(normalized, &analysis); err != nil {
		return nil, err
	}
	if analysis.Signals == nil {
		analysis.Signals = []DashboardChunkSignal{}
	}
	if analysis.ActionCandidates == nil {
		analysis.ActionCandidates = []actions.AIFirstActionCandidate{}
	}
	return &analysis, nil
}

func normalizeStringArrayJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		out, _ := json.Marshal([]string{})
		return out
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		out, _ := json.Marshal(compactStringIDs(arr))
		return out
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && strings.TrimSpace(single) != "" {
		out, _ := json.Marshal([]string{single})
		return out
	}
	out, _ := json.Marshal([]string{})
	return out
}

func normalizeDashboardChunkSignalsJSON(raw json.RawMessage) json.RawMessage {
	signals, err := parseFlexibleChunkSignals(raw)
	if err != nil {
		out, _ := json.Marshal([]DashboardChunkSignal{})
		return out
	}
	out, err := json.Marshal(signals)
	if err != nil {
		out, _ := json.Marshal([]DashboardChunkSignal{})
		return out
	}
	return out
}

func normalizeActionCandidatesJSON(raw json.RawMessage) json.RawMessage {
	candidates, err := parseFlexibleActionCandidates(raw)
	if err != nil {
		out, _ := json.Marshal([]actions.AIFirstActionCandidate{})
		return out
	}
	out, err := json.Marshal(candidates)
	if err != nil {
		out, _ := json.Marshal([]actions.AIFirstActionCandidate{})
		return out
	}
	return out
}

func normalizeDataQualityNotesJSON(raw json.RawMessage) json.RawMessage {
	var notes []ChunkDataQualityNote
	if err := json.Unmarshal(raw, &notes); err == nil {
		out, _ := json.Marshal(notes)
		return out
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		converted := make([]ChunkDataQualityNote, 0, len(arr))
		for _, n := range arr {
			converted = append(converted, ChunkDataQualityNote{Note: n})
		}
		out, _ := json.Marshal(converted)
		return out
	}
	return raw
}
