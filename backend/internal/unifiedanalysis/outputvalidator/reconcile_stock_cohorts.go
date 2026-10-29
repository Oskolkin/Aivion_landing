package outputvalidator

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// ReconcileStockActionCohorts drops mixed/umbrella stock actions so that
// EnsureMaterialDomainActions can publish mutually exclusive band Actions
// (OOS / critical 0–3 / short 4–7 / surplus) without competing with an
// AI-authored "N дефицитных SKU" umbrella that mixes healthy and deficit SKUs.
//
// Must run after SanitizeGrounding and before EnsureMaterialDomainActions.
func ReconcileStockActionCohorts(out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) int {
	if out == nil || idx == nil || len(out.Actions) == 0 {
		return 0
	}
	kept := make([]contract.UnifiedActionV1, 0, len(out.Actions))
	dropped := 0
	for _, a := range out.Actions {
		if !isStockCohortActionType(a.Type) {
			kept = append(kept, a)
			continue
		}
		if shouldDropStockCohortAction(a, idx) {
			dropped++
			continue
		}
		// Homogeneous band action: keep only entities that still match the band.
		band := dominantStockBand(a, idx)
		filtered := filterEntitiesToStockBand(a.Scope.EntityIDs, band, idx)
		if len(filtered) == 0 {
			dropped++
			continue
		}
		a.Scope.EntityIDs = filtered
		kept = append(kept, a)
	}
	out.Actions = kept
	return dropped
}

func isStockCohortActionType(typ string) bool {
	switch strings.TrimSpace(typ) {
	case "stock_replenishment", "stock_redistribution", "key_sku_protection":
		return true
	default:
		return false
	}
}

func shouldDropStockCohortAction(a contract.UnifiedActionV1, idx *PackReferenceIndex) bool {
	// Preferred publication model: mutually exclusive band Actions are rebuilt
	// deterministically by EnsureMaterialDomainActions. Any prior stock cohort
	// Action (AI umbrella or earlier backfill) is dropped so bands stay clean
	// and do not fragment into "3 + 1" / "5 + 2" duplicates of the same band.
	_ = idx
	return isStockCohortActionType(a.Type)
}

func dominantStockBand(a contract.UnifiedActionV1, idx *PackReferenceIndex) string {
	bands := stockBandsForEntities(a.Scope.EntityIDs, idx)
	if len(bands) == 1 {
		for b := range bands {
			return b
		}
	}
	return ""
}

func stockBandsForEntities(entityIDs []string, idx *PackReferenceIndex) map[string]int {
	out := make(map[string]int)
	for _, raw := range entityIDs {
		id := bareEntityID(raw)
		f, ok := idx.SKUFacts[id]
		if !ok || !f.HasStockData {
			continue
		}
		seg := stockpolicy.MapToCoverageSegment(f.StockStatus)
		if seg == "" {
			seg = stockpolicy.SegmentCollecting
		}
		out[seg]++
	}
	return out
}

func filterEntitiesToStockBand(entityIDs []string, band string, idx *PackReferenceIndex) []string {
	if band == "" {
		return nil
	}
	out := make([]string, 0, len(entityIDs))
	seen := make(map[string]struct{}, len(entityIDs))
	for _, raw := range entityIDs {
		id := bareEntityID(raw)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		f, ok := idx.SKUFacts[id]
		if !ok || !f.HasStockData {
			continue
		}
		if stockpolicy.MapToCoverageSegment(f.StockStatus) != band {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id) // always persist bare canonical SKU ids
	}
	return out
}

func bareEntityID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return ""
	}
	if i := strings.IndexByte(id, ':'); i >= 0 {
		prefix := strings.ToLower(id[:i])
		if prefix == "sku" || prefix == "campaign" || prefix == "product" {
			return strings.TrimSpace(id[i+1:])
		}
	}
	return id
}

// validateStockCohortExclusivity rejects publications where a SKU appears in
// more than one stock cohort action, or where deficit and surplus overlap.
func validateStockCohortExclusivity(report *Report, out *contract.UnifiedAnalysisOutputV1, idx *PackReferenceIndex) {
	if out == nil || idx == nil {
		return
	}
	owner := map[string]string{}
	deficit := map[string]string{}
	surplus := map[string]string{}
	for ai, a := range out.Actions {
		if !isStockCohortActionType(a.Type) {
			continue
		}
		path := "$.actions[" + itoa(ai) + "]"
		bands := stockBandsForEntities(a.Scope.EntityIDs, idx)
		if len(bands) > 1 {
			report.addError(CodeStockCohortOverlap, path+".scope.entity_ids",
				"stock action mixes mutually exclusive coverage bands")
		}
		for _, raw := range a.Scope.EntityIDs {
			id := bareEntityID(raw)
			if id == "" {
				continue
			}
			if prev, ok := owner[id]; ok {
				report.addError(CodeStockCohortOverlap, path+".scope.entity_ids",
					"SKU "+id+" already owned by "+prev)
				continue
			}
			owner[id] = a.AnalysisActionID
			f := idx.SKUFacts[id]
			seg := stockpolicy.MapToCoverageSegment(f.StockStatus)
			switch seg {
			case stockpolicy.SegmentOutOfStock, stockpolicy.SegmentCritical0_3, stockpolicy.SegmentShortage4_7:
				deficit[id] = a.AnalysisActionID
			case stockpolicy.SegmentSurplus:
				surplus[id] = a.AnalysisActionID
			}
		}
	}
	for id, defAction := range deficit {
		if surAction, ok := surplus[id]; ok {
			report.addError(CodeStockDeficitSurplusOverlap, "$.actions",
				"SKU "+id+" in both deficit ("+defAction+") and surplus ("+surAction+") without warehouse scope")
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
