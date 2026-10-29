package outputvalidator

import (
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// SanitizeResult summarizes deterministic grounding repairs applied before
// technical validation. These repairs never invent narrative claims; they only
// correct known identity / entity / target failures that models emit despite
// explicit prompt rules.
type SanitizeResult struct {
	PackIdentityForced       bool
	UnknownSKUsRemoved       int
	DuplicateSKUsRemoved     int
	CompletionTargetsRaised  int
	SignalIDConflictsCleared int
}

// SanitizeGrounding applies deterministic repairs so a provider payload can
// pass the publication contract when the failures are mechanical:
//   - analysis_meta pack_id / pack_hash / metric_date overwritten from the run
//   - unknown Pack SKUs stripped from Action scopes
//   - SKU ids reused across Actions kept only on the first Action
//   - completion targets already met at analysis time nudged beyond baseline
//   - duplicate signal_id across exclusive what_changed sections cleared on later hits
func SanitizeGrounding(out *contract.UnifiedAnalysisOutputV1, meta ExpectedRunMeta, idx *PackReferenceIndex) SanitizeResult {
	var res SanitizeResult
	if out == nil {
		return res
	}

	if strings.TrimSpace(meta.PackID) != "" && out.AnalysisMeta.PackID != meta.PackID {
		out.AnalysisMeta.PackID = meta.PackID
		res.PackIdentityForced = true
	}
	if strings.TrimSpace(meta.PackHash) != "" && out.AnalysisMeta.PackHash != meta.PackHash {
		out.AnalysisMeta.PackHash = meta.PackHash
		res.PackIdentityForced = true
	}
	if strings.TrimSpace(meta.MetricDate) != "" && out.AnalysisMeta.MetricDate != meta.MetricDate {
		out.AnalysisMeta.MetricDate = meta.MetricDate
		res.PackIdentityForced = true
	}

	seenSKU := map[string]struct{}{}
	for i := range out.Actions {
		a := &out.Actions[i]
		scopeType := strings.ToLower(strings.TrimSpace(a.Scope.ScopeType))
		filtered := make([]string, 0, len(a.Scope.EntityIDs))
		for _, raw := range a.Scope.EntityIDs {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if idx != nil {
				isSKU := idx.HasSKU(id)
				isCampaign := idx.HasCampaign(id)
				switch scopeType {
				case "sku":
					if !isSKU {
						res.UnknownSKUsRemoved++
						continue
					}
				case "campaign":
					// Leave campaign validation to the hard contract.
				default:
					if !isSKU && !isCampaign {
						res.UnknownSKUsRemoved++
						continue
					}
				}
				if isSKU {
					if _, dup := seenSKU[id]; dup {
						res.DuplicateSKUsRemoved++
						continue
					}
					seenSKU[id] = struct{}{}
				}
			}
			filtered = append(filtered, id)
		}
		a.Scope.EntityIDs = filtered

		// If sku/campaign scope lost every entity to unknown/duplicate filters,
		// promote to account scope so the Action remains (count floors stay
		// intact) without inventing entity ids.
		if (scopeType == "sku" || scopeType == "campaign") && len(filtered) == 0 {
			a.Scope.ScopeType = "account"
			a.Scope.EntityIDs = nil
		}

		for j := range a.CompletionConditions {
			c := &a.CompletionConditions[j]
			if c.BaselineValue == nil || c.TargetValue == nil || strings.TrimSpace(c.Operator) == "" {
				continue
			}
			if !conditionAlreadyMet(*c.BaselineValue, c.Operator, *c.TargetValue) {
				continue
			}
			raised := unmetTarget(*c.BaselineValue, c.Operator)
			c.TargetValue = &raised
			res.CompletionTargetsRaised++
		}
	}

	res.SignalIDConflictsCleared = clearDuplicateWhatChangedSignalIDs(out)
	return res
}

func unmetTarget(baseline float64, operator string) float64 {
	step := math.Max(1, math.Abs(baseline)*0.1)
	switch normalizeOperator(operator) {
	case ">=":
		return baseline + step
	case ">":
		return baseline + step
	case "<=":
		return baseline - step
	case "<":
		return baseline - step
	case "==":
		if baseline == 0 {
			return 1
		}
		return baseline + step
	default:
		return baseline + step
	}
}

func clearDuplicateWhatChangedSignalIDs(out *contract.UnifiedAnalysisOutputV1) int {
	cleared := 0
	seen := map[string]struct{}{}
	for gi := range out.Dashboard.WhatChanged.Groups {
		g := &out.Dashboard.WhatChanged.Groups[gi]
		for ii := range g.Items {
			item := &g.Items[ii]
			id := strings.TrimSpace(item.SignalID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				item.SignalID = ""
				cleared++
				continue
			}
			seen[id] = struct{}{}
		}
	}
	return cleared
}
