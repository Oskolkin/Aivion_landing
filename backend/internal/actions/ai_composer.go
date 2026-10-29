package actions

import (
	"context"
	"encoding/json"
	"log/slog"
)

// processDomain runs compose → validate → repair → validate → fallback for one domain.
func (s *GenerationService) processDomain(
	ctx context.Context,
	candidateCtx *ActionCandidateContext,
	domainCtx ActionDomainContext,
	opts GenerateActionsOptions,
) DomainPipelineResult {
	result := DomainPipelineResult{
		Domain: domainCtx.Domain,
		Status: CategoryRunStatusFailed,
	}

	inputCtx, _ := json.Marshal(compactDomainContext(domainCtx))
	result.InputContext = inputCtx

	if len(domainCtx.Candidates) == 0 || domainCtx.Status == DomainStatusEmpty {
		result.Status = CategoryRunStatusSkippedEmpty
		return result
	}

	rules := validationRulesForDomain(domainCtx.Domain, candidateCtx.ForbiddenClaims)

	var (
		output       *DomainComposerOutput
		rawResp      *AIRawResponse
		usedFallback bool
		repaired     bool
	)

	if !opts.AIEnabled {
		output = &DomainComposerOutput{Domain: domainCtx.Domain, Actions: BuildFallbackActions(domainCtx)}
		usedFallback = true
	} else {
		var composeErr error
		output, rawResp, composeErr = s.ai.ComposeDomainActions(ctx, DomainComposerInput{
			Domain:          domainCtx.Domain,
			DomainContext:   domainCtx,
			AccountSnapshot: candidateCtx.AccountSnapshot,
			ForbiddenClaims: candidateCtx.ForbiddenClaims,
			AllowedTypes:    domainAllowedTypes[domainCtx.Domain],
		})
		result.RawResponse = rawResp
		if composeErr != nil {
			slog.Warn("actions: domain compose failed", "domain", domainCtx.Domain, "err", composeErr)
			output = &DomainComposerOutput{Domain: domainCtx.Domain, Actions: BuildFallbackActions(domainCtx)}
			usedFallback = true
		} else if output == nil {
			output = &DomainComposerOutput{Domain: domainCtx.Domain, Actions: BuildFallbackActions(domainCtx)}
			usedFallback = true
		}
	}

	validation := ValidateDomainComposerOutput(domainCtx, *output, rules)
	result.Validation = validation

	if !usedFallback && shouldAttemptRepair(validation, opts.AlwaysRunAIRepair) && opts.EnableAIRepair && opts.AIEnabled {
		repairedOutput, repairResp, repairErr := s.ai.RepairDomainActions(ctx, DomainRepairInput{
			DomainContext:    domainCtx,
			DraftOutput:      *output,
			ValidationIssues: validation.Issues,
		})
		result.RepairResponse = repairResp
		if repairErr == nil && repairedOutput != nil {
			repaired = true
			finalValidation := ValidateDomainComposerOutput(domainCtx, *repairedOutput, rules)
			result.FinalValidation = finalValidation
			validation = finalValidation
			result.Validation = validation
		}
	}

	if len(validation.AcceptedActions) == 0 {
		fallback := BuildFallbackActions(domainCtx)
		fbValidation := ValidateDomainComposerOutput(domainCtx, DomainComposerOutput{
			Domain:  domainCtx.Domain,
			Actions: fallback,
		}, rules)
		result.FinalValidation = fbValidation
		validation = fbValidation
		usedFallback = true
	}

	result.AcceptedActions = finalizeAcceptedActions(domainCtx, validation.AcceptedActions)
	result.DroppedCount = len(validation.DroppedActions)
	result.UsedFallback = usedFallback
	result.Repaired = repaired

	switch {
	case len(result.AcceptedActions) == 0:
		result.Status = CategoryRunStatusFailed
	case usedFallback:
		// Deterministic fallback recovered actions; this is a partial completion,
		// not a generic dead-end. UsedFallback still drives run-level fallback_count.
		result.Status = CategoryRunStatusCompletedPartial
	case repaired && result.DroppedCount > 0:
		result.Status = CategoryRunStatusCompletedPartial
	case repaired:
		result.Status = CategoryRunStatusRepair
	case result.DroppedCount > 0:
		result.Status = CategoryRunStatusCompletedPartial
	default:
		result.Status = CategoryRunStatusCompleted
	}
	return result
}

func findDomainContext(ctx *ActionCandidateContext, domain string) (ActionDomainContext, bool) {
	for _, d := range ctx.Domains {
		if d.Domain == domain {
			return d, true
		}
	}
	return ActionDomainContext{Domain: domain, Status: DomainStatusEmpty}, false
}

func domainOrder() []string {
	return []string{ActionDomainStock, ActionDomainSKUSales, ActionDomainAdvertising, ActionDomainFunnel}
}

func marshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func rawResponseModel(resp *AIRawResponse) string {
	if resp == nil {
		return ""
	}
	return resp.Model
}

func rawResponseJSON(resp *AIRawResponse) []byte {
	if resp == nil {
		return nil
	}
	payload := map[string]any{
		"model":         resp.Model,
		"raw_text":      resp.RawText,
		"latency_ms":    resp.LatencyMs,
		"response_json": resp.ResponseJSON,
	}
	return marshalJSON(payload)
}
