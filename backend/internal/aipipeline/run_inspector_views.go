package aipipeline

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// InspectorSignalView is a read-only signal row for Inspector UI.
type InspectorSignalView struct {
	SignalID         string   `json:"signal_id"`
	ChunkID          string   `json:"chunk_id"`
	ChunkIDSource    string   `json:"chunk_id_source,omitempty"`
	Type             string   `json:"type"`
	Category         string   `json:"category,omitempty"`
	Observation      string   `json:"observation"`
	FactIDs          []string `json:"fact_ids,omitempty"`
	EntityIDs        []string `json:"entity_ids,omitempty"`
	Status           string   `json:"status"`
	ValidationIssues []string `json:"validation_issues,omitempty"`
}

// InspectorCandidateView is a read-only candidate row for Inspector UI.
type InspectorCandidateView struct {
	CandidateID       string   `json:"candidate_id"`
	ChunkID           string   `json:"chunk_id"`
	Type              string   `json:"type"`
	Category          string   `json:"category"`
	Priority          string   `json:"priority,omitempty"`
	Problem           string   `json:"problem"`
	WhyActionable     string   `json:"why_actionable,omitempty"`
	RecommendedAction string   `json:"recommended_action,omitempty"`
	SourceSignalIDs   []string `json:"source_signal_ids,omitempty"`
	EvidenceFactIDs   []string `json:"evidence_fact_ids,omitempty"`
	EntityIDs         []string `json:"entity_ids,omitempty"`
	Status            string   `json:"status"`
	Disposition       string   `json:"disposition,omitempty"`
}

// InspectorFactView is a read-only fact row for Inspector UI.
type InspectorFactView struct {
	FactID       string `json:"fact_id"`
	MetricCode   string `json:"metric_code,omitempty"`
	EntityID     string `json:"entity_id,omitempty"`
	Value        any    `json:"value,omitempty"`
	Unit         string `json:"unit,omitempty"`
	DisplayLabel string `json:"display_label,omitempty"`
	SourceChunk  string `json:"source_chunk,omitempty"`
}

// InspectorEntityView is a read-only entity row for Inspector UI.
type InspectorEntityView struct {
	EntityID   string `json:"entity_id"`
	EntityType string `json:"entity_type,omitempty"`
	Name       string `json:"name,omitempty"`
	OfferID    string `json:"offer_id,omitempty"`
	CampaignID string `json:"campaign_id,omitempty"`
	CategoryID string `json:"category_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

// InspectorActionsInputSummary summarizes actions compose input.
type InspectorActionsInputSummary struct {
	SchemaVersion     string `json:"schema_version"`
	CandidateCount    int    `json:"candidate_count"`
	FactCount         int    `json:"fact_count"`
	EntityCount       int    `json:"entity_count"`
	MaxActions        int    `json:"max_actions"`
	FailedChunkCount  int    `json:"failed_chunk_count"`
	DataPackHash      string `json:"data_pack_hash,omitempty"`
	GlobalContextHash string `json:"global_context_hash,omitempty"`
	RawAvailable      bool   `json:"raw_available"`
}

// InspectorArtifactRow describes one artifact in the index table.
type InspectorArtifactRow struct {
	ID                  int64  `json:"id"`
	Kind                string `json:"kind"`
	Hash                string `json:"hash,omitempty"`
	AdapterUsed         string `json:"adapter_used"`
	ParsedStatus        string `json:"parsed_status"`
	ContentCompleteness string `json:"content_completeness,omitempty"`
	RawBodyAvailable    bool   `json:"raw_body_available"`
	RequiredDebug       string `json:"required_debug,omitempty"`
	SchemaVersion       string `json:"schema_version,omitempty"`
}

func buildInspectorViewModels(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) map[string]any {
	candidateBySignal := buildCandidateSignalIndex(byKind)
	chunkMapping := buildChunkRefMappingIndex(byKind)
	views := map[string]any{
		"signals":       buildSignalViews(byKind, candidateBySignal, chunkMapping),
		"candidates":    buildCandidateViews(byKind, legacy),
		"facts":         buildFactViews(byKind),
		"entities":      buildEntityViews(byKind),
		"artifacts":     buildArtifactRows(byKind, legacy),
		"quality":       buildQualitySummary(byKind, legacy),
		"actions_input": buildActionsInputView(byKind),
		"diagnostics":   buildInspectorDiagnostics(byKind, legacy),
	}
	if legacyView := legacy.BuildView(); legacyView != nil {
		views["legacy_actions"] = legacyView
	}
	views["monitoring"] = buildInspectorMonitoringViews(byKind)
	return views
}

func buildSignalViews(byKind map[string]dbgen.AiPipelineArtifact, candidateBySignal map[string]string, chunkMapping map[string]string) []InspectorSignalView {
	shared, ok := byKind[ArtifactKindSharedChunkAnalysis]
	if !ok {
		return nil
	}
	payload, ok := decodeArtifactPayload(shared.PayloadJson).(map[string]any)
	if !ok {
		return nil
	}
	validated, _ := payload["validated_signals"].([]any)
	rejectedCount := intValue(payload["rejected_signal_count"])
	var out []InspectorSignalView
	for _, item := range validated {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		chunkID, chunkIDSource := resolveSignalChunkIDWithSource(m, candidateBySignal, chunkMapping)
		out = append(out, InspectorSignalView{
			SignalID:      stringValue(m["signal_id"]),
			ChunkID:       chunkID,
			ChunkIDSource: chunkIDSource,
			Type:          stringValue(m["signal_type"]),
			Observation:   stringValue(m["observation"]),
			FactIDs:       stringSlice(m["fact_ids"]),
			EntityIDs:     stringSlice(m["entity_ids"]),
			Status:        "accepted",
		})
	}
	if rejectedCount > 0 && len(out) == 0 {
		_ = rejectedCount
	}
	return out
}

func buildCandidateViews(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) []InspectorCandidateView {
	shared, ok := byKind[ArtifactKindSharedChunkAnalysis]
	if !ok {
		return nil
	}
	payload, ok := decodeArtifactPayload(shared.PayloadJson).(map[string]any)
	if !ok {
		return nil
	}
	validated, _ := payload["validated_action_candidates"].([]any)
	coverage := legacy.extractCandidateCoverage()
	dispositionByID := buildCandidateDispositionMap(coverage, legacy)
	var out []InspectorCandidateView
	for _, item := range validated {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(m["candidate_id"])
		entIDs := []string{}
		if ents, ok := m["affected_entities"].([]any); ok {
			for _, e := range ents {
				if em, ok := e.(map[string]any); ok {
					if eid := stringValue(em["entity_id"]); eid != "" {
						entIDs = append(entIDs, eid)
					}
				}
			}
		}
		out = append(out, InspectorCandidateView{
			CandidateID:       id,
			ChunkID:           stringValue(m["chunk_id"]),
			Type:              stringValue(m["type"]),
			Category:          stringValue(m["category"]),
			Priority:          stringValue(m["local_priority"]),
			Problem:           stringValue(m["problem"]),
			WhyActionable:     stringValue(m["why_actionable"]),
			RecommendedAction: stringValue(m["recommended_action"]),
			SourceSignalIDs:   stringSlice(m["source_signal_ids"]),
			EvidenceFactIDs:   stringSlice(m["evidence_fact_ids"]),
			EntityIDs:         entIDs,
			Status:            "validated",
			Disposition:       dispositionByID[id],
		})
	}
	return enrichCandidateDispositions(out, coverage, legacy.rawReferencedCandidateIDs())
}

func buildFactViews(byKind map[string]dbgen.AiPipelineArtifact) []InspectorFactView {
	pack, ok := byKind[ArtifactKindDataPack]
	if !ok {
		return nil
	}
	payload, ok := decodeArtifactPayload(pack.PayloadJson).(map[string]any)
	if !ok {
		return nil
	}
	registry, _ := payload["fact_registry"].(map[string]any)
	var out []InspectorFactView
	for id, raw := range registry {
		fm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, InspectorFactView{
			FactID:       id,
			MetricCode:   stringValue(fm["metric"]),
			EntityID:     stringValue(fm["entity_id"]),
			Value:        fm["value"],
			Unit:         stringValue(fm["unit"]),
			DisplayLabel: stringValue(fm["display_label"]),
		})
	}
	return out
}

func buildEntityViews(byKind map[string]dbgen.AiPipelineArtifact) []InspectorEntityView {
	pack, ok := byKind[ArtifactKindDataPack]
	if !ok {
		return nil
	}
	payload, ok := decodeArtifactPayload(pack.PayloadJson).(map[string]any)
	if !ok {
		return nil
	}
	registry, _ := payload["entity_registry"].(map[string]any)
	return flattenEntityRegistry(registry)
}

func flattenEntityRegistry(registry map[string]any) []InspectorEntityView {
	if registry == nil {
		return nil
	}
	containerKeys := []string{"skus", "campaigns", "categories", "sku_groups", "warehouses"}
	var out []InspectorEntityView
	for _, container := range containerKeys {
		group, ok := registry[container].(map[string]any)
		if !ok {
			continue
		}
		for entityID, raw := range group {
			em, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := stringValue(em["entity_id"])
			if id == "" {
				id = entityID
			}
			if id == container {
				continue
			}
			view := InspectorEntityView{
				EntityID:   id,
				EntityType: stringValue(em["entity_type"]),
				Name:       stringValue(em["name"]),
				OfferID:    stringValue(em["offer_id"]),
				Status:     stringValue(em["status"]),
			}
			if view.EntityType == "" {
				view.EntityType = strings.TrimSuffix(container, "s")
			}
			if view.EntityType == "sku" {
				view.OfferID = firstNonEmpty(view.OfferID, stringValue(em["offer_id"]))
				view.CategoryID = stringValue(em["category_id"])
			}
			if view.EntityType == "campaign" {
				view.CampaignID = stringValue(em["campaign_external_id"])
			}
			if view.EntityType == "category" {
				view.CategoryID = stringValue(em["category_id"])
			}
			out = append(out, view)
		}
	}
	for id, raw := range registry {
		if isEntityContainerKey(id) {
			continue
		}
		em, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, InspectorEntityView{
			EntityID:   id,
			EntityType: stringValue(em["entity_type"]),
			Name:       stringValue(em["name"]),
			OfferID:    stringValue(em["offer_id"]),
		})
	}
	return out
}

func isEntityContainerKey(key string) bool {
	switch key {
	case "skus", "campaigns", "categories", "sku_groups", "warehouses":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func buildArtifactRows(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) []InspectorArtifactRow {
	rows := make([]InspectorArtifactRow, 0, len(byKind))
	for kind, art := range byKind {
		adapter := "native"
		if isLegacyActionsKind(kind) {
			adapter = "legacy"
		}
		if kind == ArtifactKindActionsComposeOutput && legacy.Available() {
			adapter = "legacy_projection"
		}
		payload := decodeArtifactPayloadMapSingle(art.PayloadJson)
		contentType := ClassifyArtifactContentCompleteness(kind, payload)
		rawAvailable := artifactRawBodyAvailable(kind, payload)
		rows = append(rows, InspectorArtifactRow{
			ID:                  art.ID,
			Kind:                kind,
			Hash:                pgText(art.Hash),
			AdapterUsed:         adapter,
			ParsedStatus:        "ok",
			ContentCompleteness: contentType,
			RawBodyAvailable:    rawAvailable,
			SchemaVersion:       stringValue(payload["schema_version"]),
		})
	}
	return rows
}

func isLegacyActionsKind(kind string) bool {
	switch kind {
	case ArtifactKindActionsInput, ArtifactKindActionsOutput, ArtifactKindActionsValidation,
		ArtifactKindActionsRawOutput, ArtifactKindPipelineSummary:
		return true
	default:
		return false
	}
}

func buildQualitySummary(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) map[string]any {
	summary := map[string]any{}
	if shared, ok := byKind[ArtifactKindSharedChunkAnalysis]; ok {
		if payload, ok := decodeArtifactPayload(shared.PayloadJson).(map[string]any); ok {
			summary["validated_signals"] = lenSlice(payload["validated_signals"])
			summary["rejected_signals"] = intValue(payload["rejected_signal_count"])
			summary["validated_candidates"] = lenSlice(payload["validated_action_candidates"])
			summary["rejected_candidates"] = intValue(payload["rejected_candidate_count"])
			if usage, ok := payload["token_usage"].(map[string]any); ok {
				summary["invented_fact_ids_rejected"] = intValue(usage["invented_fact_ids_rejected"])
			}
		}
	}
	if legacyView := legacy.BuildView(); legacyView != nil {
		if diag, ok := legacyView["diagnostics"].(map[string]any); ok {
			for k, v := range diag {
				summary[k] = v
			}
		}
		if final, ok := legacyView["final_actions"].(map[string]any); ok {
			summary["actions_returned"] = legacyReturnedCount(legacy)
			summary["actions_persisted"] = intValue(final["accepted_count"])
			summary["actions_dropped"] = intValue(final["dropped_count"])
		}
	}
	return summary
}

func legacyReturnedCount(legacy LegacyActionsInspectorAdapter) any {
	compose := legacy.buildComposeView()
	if status, _ := compose["returned_count_status"].(string); status == "unknown" {
		return nil
	}
	if n, ok := compose["returned_count"].(int); ok {
		return n
	}
	if compose, ok := compose["raw_actions"].([]any); ok {
		return len(compose)
	}
	return nil
}

func lenSlice(v any) int {
	arr, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(arr)
}

func intValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func enrichCandidateDispositions(candidates []InspectorCandidateView, coverage map[string]any, rawIDs []string) []InspectorCandidateView {
	if len(candidates) == 0 {
		return candidates
	}
	if len(coverage) == 0 && len(rawIDs) == 0 {
		for i := range candidates {
			if candidates[i].Disposition == "" {
				candidates[i].Disposition = "not_reconstructable"
			}
		}
		return candidates
	}
	rawSet := map[string]struct{}{}
	for _, id := range rawIDs {
		rawSet[id] = struct{}{}
	}
	persistedSet := map[string]struct{}{}
	droppedSet := map[string]struct{}{}
	if entries, ok := coverage["entries"].([]any); ok {
		for _, item := range entries {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id := stringValue(entry["candidate_id"])
			switch stringValue(entry["disposition"]) {
			case "referenced_in_persisted_action":
				persistedSet[id] = struct{}{}
			case "referenced_in_dropped_action_only":
				droppedSet[id] = struct{}{}
			}
		}
	}
	for i := range candidates {
		id := candidates[i].CandidateID
		if _, ok := persistedSet[id]; ok {
			candidates[i].Disposition = "referenced_in_persisted_action"
			continue
		}
		if _, ok := droppedSet[id]; ok {
			candidates[i].Disposition = "referenced_in_dropped_action_only"
			continue
		}
		if _, ok := rawSet[id]; ok {
			if intValue(coverage["actions_candidate_referenced_dropped_only_count"]) > 0 {
				candidates[i].Disposition = "referenced_in_dropped_action_only"
			} else {
				candidates[i].Disposition = "referenced_in_persisted_action"
			}
			continue
		}
		if candidates[i].Disposition == "" {
			candidates[i].Disposition = "not_referenced_by_ai"
		}
	}
	return candidates
}

func mergeLegacyValidationIntoViews(views map[string]any, legacy LegacyActionsInspectorAdapter) {
	la, ok := views["legacy_actions"].(map[string]any)
	if !ok || la == nil {
		return
	}
	if validation, ok := la["validation"].(map[string]any); ok {
		views["validation"] = validation
	}
	if norm, ok := la["normalization"].(map[string]any); ok {
		views["normalization"] = norm
	}
	_ = legacy
}

func inspectorHasTwoPhaseActions(byKind map[string]dbgen.AiPipelineArtifact) bool {
	_, ok := byKind[ArtifactKindActionsComposeOutput]
	return ok
}

func actionsStageAdapterLabel(byKind map[string]dbgen.AiPipelineArtifact, legacy LegacyActionsInspectorAdapter) string {
	if inspectorHasTwoPhaseActions(byKind) {
		return "two_phase"
	}
	if legacy.Available() {
		return "legacy"
	}
	return "missing"
}

func placeholderCountInGlobalContext(byKind map[string]dbgen.AiPipelineArtifact) int {
	art, ok := byKind[ArtifactKindGlobalContextFinal]
	if !ok {
		return 0
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return 0
	}
	story := stringValue(payload["overall_story"])
	return len(DetectUnresolvedPlaceholders(story))
}

func metricSemanticMismatchCount(text string, metrics ...string) int {
	count := 0
	for _, m := range metrics {
		_ = m
	}
	if strings.Contains(strings.ToLower(text), "срок хранения") {
		count++
	}
	return count
}

func buildActionsInputView(byKind map[string]dbgen.AiPipelineArtifact) map[string]any {
	kind := ArtifactKindActionsInput
	if _, ok := byKind[kind]; !ok {
		kind = ArtifactKindActionsComposeInput
	}
	art, ok := byKind[kind]
	if !ok {
		return map[string]any{"available": false}
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return map[string]any{"available": false}
	}
	factCount := 0
	if fc, ok := payload["fact_catalog"].(map[string]any); ok {
		factCount = len(fc)
	} else if fr, ok := payload["fact_registry"].(map[string]any); ok {
		factCount = len(fr)
	}
	entityCount := countActionsInputEntities(payload)
	candidates, _ := payload["action_candidates"].([]any)
	candidates = enrichActionsInputCandidates(candidates)
	return map[string]any{
		"available":            true,
		"artifact_id":          art.ID,
		"schema_version":       stringValue(payload["schema_version"]),
		"candidate_count":      len(candidates),
		"fact_count":           factCount,
		"entity_count":         entityCount,
		"max_actions":          intValue(payload["max_actions"]),
		"failed_chunk_count":   intValue(payload["failed_chunk_ids"]),
		"data_pack_hash":       stringValue(payload["data_pack_hash"]),
		"global_context_hash":  stringValue(payload["global_context_hash"]),
		"allowed_types":        payload["allowed_action_types"],
		"allowed_categories":   payload["allowed_action_categories"],
		"allowed_priorities":   payload["allowed_action_priorities"],
		"forbidden_claims":     payload["forbidden_claims"],
		"candidates":           candidates,
		"fact_catalog":         payload["fact_catalog"],
		"entity_catalog":       payload["entity_catalog"],
		"global_context_input": payload["global_context"],
		"raw":                  payload,
	}
}

func enrichActionsInputCandidates(candidates []any) []any {
	if len(candidates) == 0 {
		return candidates
	}
	out := make([]any, 0, len(candidates))
	for _, item := range candidates {
		m, ok := item.(map[string]any)
		if !ok {
			out = append(out, item)
			continue
		}
		cp := make(map[string]any, len(m)+1)
		for k, v := range m {
			cp[k] = v
		}
		cp["search_text"] = buildCandidateSearchText(m)
		out = append(out, cp)
	}
	return out
}

func buildCandidateSearchText(c map[string]any) string {
	parts := []string{
		stringValue(c["candidate_id"]),
		stringValue(c["chunk_id"]),
		stringValue(c["type"]),
		stringValue(c["category"]),
		stringValue(c["local_priority"]),
		stringValue(c["priority"]),
		stringValue(c["problem"]),
		stringValue(c["recommended_action"]),
	}
	parts = append(parts, stringSlice(c["source_signal_ids"])...)
	parts = append(parts, stringSlice(c["evidence_fact_ids"])...)
	if ents, ok := c["affected_entities"].([]any); ok {
		for _, e := range ents {
			if em, ok := e.(map[string]any); ok {
				parts = append(parts, stringValue(em["entity_id"]), stringValue(em["offer_id"]), stringValue(em["sku"]))
			}
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func countActionsInputEntities(payload map[string]any) int {
	if ec, ok := payload["entity_catalog"].(map[string]any); ok {
		if hasNestedEntityContainers(ec) {
			return len(flattenEntityRegistry(ec))
		}
		return len(ec)
	}
	if er, ok := payload["entity_registry"].(map[string]any); ok {
		return len(flattenEntityRegistry(er))
	}
	return 0
}

func hasNestedEntityContainers(registry map[string]any) bool {
	for _, key := range []string{"skus", "campaigns", "categories", "sku_groups", "warehouses"} {
		if _, ok := registry[key].(map[string]any); ok {
			return true
		}
	}
	return false
}

func buildCandidateDispositionMap(coverage map[string]any, legacy LegacyActionsInspectorAdapter) map[string]string {
	out := map[string]string{}
	if entries, ok := coverage["entries"].([]any); ok {
		for _, item := range entries {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out[stringValue(entry["candidate_id"])] = stringValue(entry["disposition"])
		}
	}
	return out
}

func buildCandidateSignalIndex(byKind map[string]dbgen.AiPipelineArtifact) map[string]string {
	out := map[string]string{}
	shared, ok := byKind[ArtifactKindSharedChunkAnalysis]
	if !ok {
		return out
	}
	payload, ok := decodeArtifactPayload(shared.PayloadJson).(map[string]any)
	if !ok {
		return out
	}
	candidates, _ := payload["validated_action_candidates"].([]any)
	for _, item := range candidates {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		chunkID := stringValue(m["chunk_id"])
		for _, sid := range stringSlice(m["source_signal_ids"]) {
			if sid != "" && chunkID != "" {
				out[sid] = chunkID
			}
		}
	}
	return out
}

func buildChunkRefMappingIndex(byKind map[string]dbgen.AiPipelineArtifact) map[string]string {
	art, ok := byKind[ArtifactKindSharedChunkRefMapping]
	if !ok {
		return nil
	}
	payload, ok := decodeArtifactPayload(art.PayloadJson).(map[string]any)
	if !ok {
		return nil
	}
	mappings, _ := payload["mappings"].([]any)
	out := map[string]string{}
	for _, item := range mappings {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		signalID := stringValue(m["signal_id"])
		chunkID := stringValue(m["chunk_id"])
		if signalID != "" && chunkID != "" {
			out[signalID] = chunkID
		}
	}
	return out
}

func resolveSignalChunkIDWithSource(m map[string]any, candidateBySignal, chunkMapping map[string]string) (chunkID, source string) {
	signalID := stringValue(m["signal_id"])
	if chunkID = stringValue(m["chunk_id"]); chunkID != "" {
		return chunkID, "explicit"
	}
	if chunkID, ok := candidateBySignal[signalID]; ok && chunkID != "" {
		return chunkID, "candidate_link"
	}
	if chunkMapping != nil {
		if chunkID, ok := chunkMapping[signalID]; ok && chunkID != "" {
			return chunkID, "shared_analysis_mapping"
		}
	}
	if idx := strings.Index(signalID, "-signal-"); idx > 0 {
		return signalID[:idx], "legacy_id_prefix"
	}
	return "", "unknown"
}

func resolveSignalChunkID(m map[string]any, candidateBySignal, chunkMapping map[string]string) string {
	chunkID, _ := resolveSignalChunkIDWithSource(m, candidateBySignal, chunkMapping)
	return chunkID
}
