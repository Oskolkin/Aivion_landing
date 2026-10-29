package dashboardhome

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// ValidateDashboardSynthesis validates final dashboard narrative from chunk signals.
func ValidateDashboardSynthesis(payload *dashboardNarrativePayload, validatedSignals []DashboardChunkSignal, pack *aidatapack.SellerAIDataPack, build dashboardNarrativeBuildResult) DashboardSynthesisValidationResult {
	result := DashboardSynthesisValidationResult{Valid: true}
	if payload == nil {
		return DashboardSynthesisValidationResult{Valid: false, HardErrors: []string{"nil_payload"}}
	}
	if pack == nil {
		return DashboardSynthesisValidationResult{Valid: false, HardErrors: []string{"nil_data_pack"}}
	}

	allowedFacts := signalFactIDSet(validatedSignals)
	allowedEntities := signalEntityIDSet(validatedSignals)
	// Manifest/account facts may also be referenced.
	for _, id := range build.DataPackFactIDs {
		allowedFacts[id] = struct{}{}
	}

	ctx := buildDashboardNarrativeValidationContextFromBuild(build)
	ctx.ContextSource = ContextSourceSellerAIDataPackChunks
	ctx.DataPackFactIDs = allowedFacts

	for i, reason := range payload.HealthReasons {
		for _, fid := range reason.FactIDs {
			if _, ok := allowedFacts[fid]; !ok {
				result.HardErrors = append(result.HardErrors, fmt.Sprintf("health_reason_%d_unvalidated_fact_%s", i, fid))
			}
		}
		if reason.DriverID != "" {
			if _, ok := ctx.DriversByID[reason.DriverID]; !ok {
				result.HardErrors = append(result.HardErrors, fmt.Sprintf("health_reason_%d_unknown_driver_%s", i, reason.DriverID))
			}
		}
	}
	for i, action := range payload.TopActions {
		for _, fid := range action.FactIDs {
			if _, ok := allowedFacts[fid]; !ok {
				result.HardErrors = append(result.HardErrors, fmt.Sprintf("top_action_%d_unvalidated_fact_%s", i, fid))
			}
		}
		text := strings.ToLower(action.Title + " " + action.Reason + " " + action.NextStep)
		for _, pat := range dataPackForbiddenSKULevelAdPatterns {
			if strings.Contains(text, pat) {
				result.HardErrors = append(result.HardErrors, fmt.Sprintf("top_action_%d_forbidden_sku_ad", i))
				break
			}
		}
	}
	_ = allowedEntities

	if len(result.HardErrors) > 0 {
		result.Valid = false
	}
	return result
}

func signalFactIDSet(signals []DashboardChunkSignal) map[string]struct{} {
	out := map[string]struct{}{}
	for _, sig := range signals {
		for _, id := range sig.FactIDs {
			out[id] = struct{}{}
		}
	}
	return out
}

func signalEntityIDSet(signals []DashboardChunkSignal) map[string]struct{} {
	out := map[string]struct{}{}
	for _, sig := range signals {
		for _, id := range sig.EntityIDs {
			out[id] = struct{}{}
		}
	}
	return out
}
