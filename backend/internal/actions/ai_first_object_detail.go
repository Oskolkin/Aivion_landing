package actions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// BuildObjectDetailBatches splits authoritative entities into batches.
func BuildObjectDetailBatches(entities []AIFirstAffectedEntityRef, batchSize int) [][]AIFirstAffectedEntityRef {
	if batchSize <= 0 {
		batchSize = ActionsObjectDetailBatchSize()
	}
	if len(entities) == 0 {
		return nil
	}
	batches := make([][]AIFirstAffectedEntityRef, 0)
	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}
		batches = append(batches, entities[i:end])
	}
	return batches
}

func buildObjectDetailBatchInput(
	input ActionsAIFirstInput,
	base AIFirstBaseAction,
	batchEntities []AIFirstAffectedEntityRef,
	batchIndex, batchCount int,
	candidateByID map[string]AIFirstActionCandidate,
) (ObjectDetailBatchInput, ObjectDetailRefMapping) {
	steps := base.Steps
	if len(steps) == 0 {
		for _, s := range base.ActionPlan {
			if instr := strings.TrimSpace(s.Instruction); instr != "" {
				steps = append(steps, instr)
			}
		}
	}

	batchInput := ObjectDetailBatchInput{
		Action: ObjectDetailActionContext{
			ActionKey:         base.ActionKey,
			Type:              base.Type,
			ScopeType:         base.ScopeType,
			OperationalGoal:   base.OperationalGoal,
			Priority:          base.Priority,
			Reason:            base.Reason,
			Steps:             steps,
			ObjectDetailBrief: base.ObjectDetailBrief,
		},
		Batch: ObjectDetailBatchMeta{
			BatchIndex:  batchIndex,
			BatchCount:  batchCount,
			EntityCount: len(batchEntities),
		},
	}
	mapping := ObjectDetailRefMapping{
		EntityRefs: map[string]ObjectDetailEntityMapping{},
		FactRefs:   map[string]string{},
	}

	factCounter := 0
	for ei, ent := range batchEntities {
		entityRef := fmt.Sprintf("e%d", ei+1)
		mapping.EntityRefs[entityRef] = ObjectDetailEntityMapping{
			EntityType: ent.EntityType,
			EntityID:   ent.EntityID,
		}
		entityInput := ObjectDetailEntityInput{
			EntityRef:        entityRef,
			EntityType:       ent.EntityType,
			Name:             resolveEntityDisplayName(input, ent),
			CandidateContext: buildCandidateContextForEntity(ent, base.ValidSourceCandidates, candidateByID),
			AvailableFacts:   buildAvailableFactsForEntity(input, ent, base.ValidSourceCandidates, candidateByID, entityRef, &factCounter, mapping.FactRefs),
		}
		if offerID := resolveEntityOfferID(input, ent); offerID != "" {
			entityInput.OfferID = offerID
		}
		batchInput.Entities = append(batchInput.Entities, entityInput)
	}
	return batchInput, mapping
}

func resolveEntityDisplayName(input ActionsAIFirstInput, ent AIFirstAffectedEntityRef) string {
	if strings.TrimSpace(ent.Name) != "" {
		return ent.Name
	}
	if ce, ok := input.EntityCatalog[ent.EntityID]; ok && strings.TrimSpace(ce.Name) != "" {
		return ce.Name
	}
	return ent.EntityID
}

func resolveEntityOfferID(input ActionsAIFirstInput, ent AIFirstAffectedEntityRef) string {
	if ce, ok := input.EntityCatalog[ent.EntityID]; ok {
		return strings.TrimSpace(ce.OfferID)
	}
	return ""
}

func buildCandidateContextForEntity(
	ent AIFirstAffectedEntityRef,
	sourceCandidates []string,
	candidateByID map[string]AIFirstActionCandidate,
) []ObjectDetailCandidateCtx {
	out := make([]ObjectDetailCandidateCtx, 0)
	entKey := entityRefKey(ent)
	for _, cid := range sourceCandidates {
		c, ok := candidateByID[cid]
		if !ok {
			continue
		}
		for _, e := range c.AffectedEntities {
			if entityRefKey(e) != entKey {
				continue
			}
			out = append(out, ObjectDetailCandidateCtx{
				CandidateID: cid,
				Observation: strings.TrimSpace(c.Problem),
			})
			break
		}
	}
	return out
}

func buildAvailableFactsForEntity(
	input ActionsAIFirstInput,
	ent AIFirstAffectedEntityRef,
	sourceCandidates []string,
	candidateByID map[string]AIFirstActionCandidate,
	entityRef string,
	factCounter *int,
	factRefMap map[string]string,
) []ObjectDetailFactInput {
	seen := map[string]bool{}
	out := make([]ObjectDetailFactInput, 0)

	addFact := func(factID string) {
		factID = strings.TrimSpace(factID)
		if factID == "" || seen[factID] {
			return
		}
		fact, ok := input.factByID(factID)
		if !ok {
			return
		}
		if !factBelongsToEntity(fact, ent) {
			return
		}
		seen[factID] = true
		*factCounter++
		factRef := fmt.Sprintf("%sf%d", entityRef, *factCounter)
		factRefMap[factRef] = factID
		out = append(out, compactFactToDetailInput(fact, factRef))
	}

	entKey := entityRefKey(ent)
	for _, cid := range sourceCandidates {
		c, ok := candidateByID[cid]
		if !ok {
			continue
		}
		for _, e := range c.AffectedEntities {
			if entityRefKey(e) != entKey {
				continue
			}
			for _, fid := range e.FactIDs {
				addFact(fid)
			}
			for _, fid := range c.EvidenceFactIDs {
				addFact(fid)
			}
		}
	}

	for factID, fact := range input.FactCatalog {
		if seen[factID] {
			continue
		}
		if strings.TrimSpace(fact.EntityID) != strings.TrimSpace(ent.EntityID) {
			continue
		}
		if fullFact, ok := input.factByID(factID); ok {
			addFact(fullFact.FactID)
		}
	}

	return out
}

func factBelongsToEntity(fact aidatapack.Fact, ent AIFirstAffectedEntityRef) bool {
	if strings.TrimSpace(fact.EntityID) == "" {
		return true
	}
	return strings.TrimSpace(fact.EntityID) == strings.TrimSpace(ent.EntityID)
}

func compactFactToDetailInput(fact aidatapack.Fact, factRef string) ObjectDetailFactInput {
	code := metricCodeFromFact(fact)
	return ObjectDetailFactInput{
		FactRef:      factRef,
		MetricCode:   code,
		Value:        fact.Value,
		Unit:         fact.Unit,
		DisplayLabel: metricDisplayLabel(fact, code),
	}
}

// ValidateObjectDetailBatchOutput validates and maps batch AI output to AIFirstObjectAction.
func ValidateObjectDetailBatchOutput(
	batchInput ObjectDetailBatchInput,
	output *ObjectDetailBatchOutput,
	mapping ObjectDetailRefMapping,
	actionIdx int,
) (valid []AIFirstObjectAction, warnings []ValidationIssue, missing []string) {
	expectedRefs := map[string]ObjectDetailEntityInput{}
	for _, e := range batchInput.Entities {
		expectedRefs[e.EntityRef] = e
	}

	seenRef := map[string]bool{}
	if output != nil {
		for oi, obj := range output.ObjectActions {
			ref := strings.TrimSpace(obj.EntityRef)
			if ref == "" {
				continue
			}
			if seenRef[ref] {
				i := actionIdx
				warnings = append(warnings, ValidationIssue{
					Severity: IssueSeverityWarning, Code: "duplicate_object_detail_entity_ref",
					Message:     fmt.Sprintf("duplicate entity_ref %q", ref),
					ActionIndex: &i, Field: fmt.Sprintf("object_actions[%d]", oi),
				})
				continue
			}
			seenRef[ref] = true

			entMapping, ok := mapping.EntityRefs[ref]
			if !ok {
				i := actionIdx
				warnings = append(warnings, ValidationIssue{
					Severity: IssueSeverityWarning, Code: "unknown_object_detail_entity_ref",
					Message:     fmt.Sprintf("unknown entity_ref %q", ref),
					ActionIndex: &i,
				})
				continue
			}
			entityInput, ok := expectedRefs[ref]
			if !ok {
				continue
			}

			metrics, metricWarnings := mapObjectDetailMetricRefs(obj.MetricRefs, ref, entityInput, mapping, actionIdx, oi)
			warnings = append(warnings, metricWarnings...)

			rec := strings.TrimSpace(obj.Recommendation)
			reason := strings.TrimSpace(obj.Reason)
			if rec == "" && reason == "" {
				continue
			}

			steps := obj.Steps
			if len(steps) == 0 {
				steps = nil
			}
			valid = append(valid, AIFirstObjectAction{
				EntityType:        entMapping.EntityType,
				EntityID:          entMapping.EntityID,
				Name:              entityInput.Name,
				RecommendedAction: rec,
				Reason:            reason,
				Metrics:           metrics,
			})
		}
	}

	for ref := range expectedRefs {
		if !seenRef[ref] {
			missing = append(missing, ref)
			mappingEnt, _ := mapping.EntityRefs[ref]
			i := actionIdx
			warnings = append(warnings, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "object_detail_entity_missing",
				Message:     fmt.Sprintf("missing object action for entity_ref %q (%s)", ref, mappingEnt.EntityID),
				ActionIndex: &i,
			})
		}
	}
	sort.Strings(missing)
	return valid, warnings, missing
}

func mapObjectDetailMetricRefs(
	metricRefs []string,
	entityRef string,
	entityInput ObjectDetailEntityInput,
	mapping ObjectDetailRefMapping,
	actionIdx, objectIdx int,
) ([]AIFirstObjectMetric, []ValidationIssue) {
	allowedFactRefs := map[string]ObjectDetailFactInput{}
	for _, f := range entityInput.AvailableFacts {
		allowedFactRefs[f.FactRef] = f
	}
	entMapping := mapping.EntityRefs[entityRef]
	var metrics []AIFirstObjectMetric
	var warnings []ValidationIssue
	for _, ref := range metricRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		factID, ok := mapping.FactRefs[ref]
		if !ok {
			i := actionIdx
			warnings = append(warnings, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "unknown_object_detail_fact_ref",
				Message:     fmt.Sprintf("unknown fact_ref %q", ref),
				ActionIndex: &i, Field: fmt.Sprintf("object_actions[%d].metric_refs", objectIdx),
			})
			continue
		}
		if _, allowed := allowedFactRefs[ref]; !allowed {
			i := actionIdx
			warnings = append(warnings, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "object_detail_fact_ref_entity_mismatch",
				Message:     fmt.Sprintf("fact_ref %q does not belong to entity_ref %q", ref, entityRef),
				ActionIndex: &i,
			})
			continue
		}
		factInput := allowedFactRefs[ref]
		metrics = append(metrics, AIFirstObjectMetric{
			FactID:         factID,
			MetricCode:     factInput.MetricCode,
			DisplayLabel:   factInput.DisplayLabel,
			FormattedValue: formatDetailFactValue(factInput),
			Label:          factInput.DisplayLabel,
			Value:          formatDetailFactValue(factInput),
			Unit:           factInput.Unit,
		})
		_ = entMapping
	}
	return metrics, warnings
}

func formatDetailFactValue(f ObjectDetailFactInput) string {
	if f.Value == nil {
		return ""
	}
	return fmt.Sprint(f.Value)
}

// MergeObjectDetailResults merges batch results preserving authoritative entity order.
func MergeObjectDetailResults(
	base AIFirstBaseAction,
	batchResults []ObjectDetailBatchResult,
) ObjectDetailMergeResult {
	byEntityID := map[string]AIFirstObjectAction{}
	for _, br := range batchResults {
		for _, obj := range br.ValidObjectActions {
			key := entityRefKey(AIFirstAffectedEntityRef{EntityType: obj.EntityType, EntityID: obj.EntityID})
			if _, ok := byEntityID[key]; !ok {
				byEntityID[key] = obj
			}
		}
	}

	ordered := make([]AIFirstObjectAction, 0, len(base.AuthoritativeEntities))
	for _, ent := range base.AuthoritativeEntities {
		key := entityRefKey(ent)
		if obj, ok := byEntityID[key]; ok {
			if obj.Name == "" {
				obj.Name = ent.Name
			}
			ordered = append(ordered, obj)
		}
	}

	total := len(base.AuthoritativeEntities)
	displayed := len(ordered)
	partial := false
	for _, br := range batchResults {
		if br.Status == "failed" || br.Status == "partial" || br.Status == "truncated" {
			partial = true
		}
	}
	if displayed < total {
		partial = true
	}

	return ObjectDetailMergeResult{
		ActionKey:             base.ActionKey,
		AuthoritativeTotal:    total,
		DisplayedCount:        displayed,
		Truncated:             displayed < total || partial,
		ObjectActions:         ordered,
		BatchResults:          batchResults,
		DetailCoveragePartial: partial,
	}
}

type objectDetailBatchJob struct {
	base        AIFirstBaseAction
	actionIndex int
	batchIndex  int
	batchCount  int
	batchInput  ObjectDetailBatchInput
	mapping     ObjectDetailRefMapping
}

// RunObjectDetailBatches executes all detail batches with concurrency control.
func RunObjectDetailBatches(
	ctx context.Context,
	client ActionsAIFirstClient,
	input ActionsAIFirstInput,
	baseActions []AIFirstBaseAction,
) ([]ObjectDetailMergeResult, int, int, int, []AIPipelineNormalizationEvent) {
	candidateByID := buildCandidateIndex(input.ActionCandidates)
	batchSize := ActionsObjectDetailBatchSize()
	maxConcurrency := ActionsObjectDetailMaxConcurrency()

	var jobs []objectDetailBatchJob
	for ai, base := range baseActions {
		if isAccountScope(base.ScopeType) || len(base.AuthoritativeEntities) == 0 {
			continue
		}
		batches := BuildObjectDetailBatches(base.AuthoritativeEntities, batchSize)
		for bi, batchEntities := range batches {
			batchInput, mapping := buildObjectDetailBatchInput(input, base, batchEntities, bi, len(batches), candidateByID)
			jobs = append(jobs, objectDetailBatchJob{
				base: base, actionIndex: ai, batchIndex: bi, batchCount: len(batches),
				batchInput: batchInput, mapping: mapping,
			})
		}
	}

	if len(jobs) == 0 {
		merges := make([]ObjectDetailMergeResult, len(baseActions))
		for i, base := range baseActions {
			merges[i] = ObjectDetailMergeResult{
				ActionKey: base.ActionKey, AuthoritativeTotal: len(base.AuthoritativeEntities),
			}
		}
		return merges, 0, 0, 0, nil
	}

	type jobResult struct {
		index  int
		result ObjectDetailBatchResult
	}

	results := make([]jobResult, len(jobs))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	totalInputTokens, totalOutputTokens, detailCalls := 0, 0, 0

	for ji, job := range jobs {
		wg.Add(1)
		go func(ji int, job objectDetailBatchJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			br := executeObjectDetailBatch(ctx, client, job)
			mu.Lock()
			if br.RawResponse != nil {
				totalInputTokens += br.RawResponse.InputTokens
				totalOutputTokens += br.RawResponse.OutputTokens
				detailCalls++
			}
			mu.Unlock()
			results[ji] = jobResult{index: ji, result: br}
		}(ji, job)
	}
	wg.Wait()

	mergeByAction := map[int][]ObjectDetailBatchResult{}
	var events []AIPipelineNormalizationEvent
	for _, jr := range results {
		job := jobs[jr.index]
		mergeByAction[job.actionIndex] = append(mergeByAction[job.actionIndex], jr.result)
		appendNormEvent(&events, AIPipelineNormalizationEvent{
			Stage: "object_detail_batch", ActionIndex: job.actionIndex, ActionKey: job.base.ActionKey,
			EventCode: batchStatusEventCode(jr.result.Status), Severity: IssueSeverityWarning,
			Details: map[string]any{
				"batch_index": job.batchIndex, "batch_count": job.batchCount,
				"entity_count": job.batchInput.Batch.EntityCount, "status": jr.result.Status,
			},
		})
	}

	merges := make([]ObjectDetailMergeResult, len(baseActions))
	for i, base := range baseActions {
		batchResults := mergeByAction[i]
		if len(batchResults) == 0 {
			merges[i] = ObjectDetailMergeResult{
				ActionKey: base.ActionKey, AuthoritativeTotal: len(base.AuthoritativeEntities),
				Truncated: len(base.AuthoritativeEntities) > 0,
			}
			continue
		}
		sort.SliceStable(batchResults, func(a, b int) bool {
			return batchResults[a].BatchIndex < batchResults[b].BatchIndex
		})
		merges[i] = MergeObjectDetailResults(base, batchResults)
	}
	return merges, detailCalls, totalInputTokens, totalOutputTokens, events
}

func executeObjectDetailBatch(ctx context.Context, client ActionsAIFirstClient, job objectDetailBatchJob) ObjectDetailBatchResult {
	timeoutSec := ActionsObjectDetailTimeoutSeconds()
	started := time.Now().UTC()
	br := ObjectDetailBatchResult{
		ActionIndex:    job.actionIndex,
		ActionKey:      job.base.ActionKey,
		BatchIndex:     job.batchIndex,
		BatchCount:     job.batchCount,
		EntityCount:    job.batchInput.Batch.EntityCount,
		Input:          job.batchInput,
		Mapping:        job.mapping,
		Status:         "failed",
		StartedAt:      started.Format(time.RFC3339),
		TimeoutSeconds: timeoutSec,
	}
	batchCtx, cancel := perCallTimeoutContext(ctx, timeoutSec)
	defer cancel()
	output, raw, err := client.ComposeObjectDetailBatch(batchCtx, job.batchInput)
	finished := time.Now().UTC()
	br.FinishedAt = finished.Format(time.RFC3339)
	br.ElapsedMs = finished.Sub(started).Milliseconds()
	br.RawResponse = raw
	if err != nil {
		br.ProviderError = err.Error()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(batchCtx.Err(), context.DeadlineExceeded) {
			br.ProviderError = "provider_timeout"
		}
		br.Warnings = []ValidationIssue{{
			Severity: IssueSeverityWarning, Code: "object_detail_batch_failed",
			Message: err.Error(), ActionIndex: &job.actionIndex,
		}}
		return br
	}
	requested := ActionsObjectDetailRequestedMaxOutputTokens()
	if IsResponseOutputTruncated(raw, requested) {
		br.Status = "truncated"
		br.OutputTruncated = true
		br.Warnings = append(br.Warnings, ValidationIssue{
			Severity: IssueSeverityWarning, Code: "object_detail_batch_output_truncated",
			Message: "object detail batch output truncated", ActionIndex: &job.actionIndex,
		})
	} else {
		br.Status = "completed"
	}
	br.Output = output
	valid, warnings, missing := ValidateObjectDetailBatchOutput(job.batchInput, output, job.mapping, job.actionIndex)
	br.Warnings = append(br.Warnings, warnings...)
	br.ValidObjectActions = valid
	br.MissingEntityRefs = missing
	if len(valid) == 0 && len(job.batchInput.Entities) > 0 {
		br.Status = "failed"
	} else if len(missing) > 0 || br.Status == "truncated" {
		br.Status = "partial"
	}
	return br
}

func batchStatusEventCode(status string) string {
	switch status {
	case "completed":
		return "object_detail_batch_completed"
	case "truncated":
		return "object_detail_batch_truncated"
	case "partial":
		return "object_detail_batch_completed"
	default:
		return "object_detail_batch_failed"
	}
}
