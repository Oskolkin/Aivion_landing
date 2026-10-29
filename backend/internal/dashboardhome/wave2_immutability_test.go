package dashboardhome

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// semanticFingerprint captures factual/semantic fields that must stay frozen
// across repeated Unified GET finalization. User-state and presentation URLs
// are intentionally excluded.
func semanticFingerprint(resp Response) string {
	payload := map[string]any{
		"health_score":           resp.AIDiagnostics.AccountSummary.HealthScore,
		"health_breakdown":       resp.AIDiagnostics.AccountSummary.HealthScoreBreakdown,
		"risk_drivers":           resp.AIDiagnostics.AccountSummary.RiskDrivers,
		"top_actions":            compactTopActionFingerprint(resp.AIDiagnostics.TopActions),
		"seller_pulse_kpis":      resp.SellerPulse.KPIs,
		"seller_pulse_narrative": resp.SellerPulse.Narrative,
		"sku_focus":              resp.SKUFocus,
		"what_changed":           resp.AIDiagnostics.WhatChanged,
		"ad_narrative":           resp.AIDiagnostics.AdEfficiency.Narrative,
		"funnel_ai":              resp.AIDiagnostics.FunnelAnalysis.AIAnalysis,
		"stock_narrative":        resp.AIDiagnostics.StockCoverage.Narrative,
		"headline":               resp.AIDiagnostics.AccountSummary.Headline,
		"executive_summary":      resp.AIDiagnostics.AccountSummary.ExecutiveSummary,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func compactTopActionFingerprint(items []TopActionItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"action_key": it.ActionKey,
			"action_id":  it.ActionID,
			"title":      it.Title,
			"priority":   it.Priority,
		})
	}
	return out
}

func TestUnifiedFinalize_SemanticFingerprintImmutable(t *testing.T) {
	score := int32(63)
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				PromptVersion:    UnifiedPublicationAdapterPromptVersion,
				HealthScore:      &score,
				Headline:         "Кабинет под давлением стока",
				ExecutiveSummary: "Ключевые SKU теряют покрытие.",
				RiskDrivers: []RiskDriver{{
					ID:       "stockout",
					Category: "stock",
					Severity: "high",
					Title:    "Стокаут",
					Reason:   "Нулевые остатки на топ SKU",
				}},
				HealthScoreBreakdown: &HealthScoreBreakdown{FinalScore: 63, RawPenalty: 40},
			},
			TopActions: []TopActionItem{
				{ActionKey: "a1", ActionID: 11, Title: "Пополнить A", Priority: "critical"},
				{ActionKey: "a2", ActionID: 12, Title: "Пополнить B", Priority: "important"},
			},
			WhatChanged: WhatChangedWidget{
				MainConclusion: "Выручка снизилась на ключевых SKU.",
			},
			AdEfficiency: AdEfficiencyWidget{
				Narrative: &AdEfficiencyNarrative{
					Headline: "Реклама умеренно нагружает кабинет",
					Summary:  "Доля расходов в общей выручке 16,8%. ДРР атрибуции 91,8%.",
				},
			},
			FunnelAnalysis: FunnelAnalysisWidget{
				AIAnalysis: FunnelAIAnalysisWidget{
					ShortSummary:   "Конверсия в заказ просела.",
					MainConclusion: "Нужно проверить карточку.",
				},
			},
			StockCoverage: StockCoverageWidget{
				Narrative: &StockCoverageNarrative{
					Headline: "Покрытие критично",
					Summary:  "Часть топ SKU без стока.",
				},
			},
		},
		SellerPulse: SellerPulse{
			KPIs: []KPICard{{Key: "revenue", Label: "Выручка", Value: "100 000 ₽"}},
			Narrative: &SellerPulseNarrative{
				Headline: "Пульс негативный",
				Summary:  "Заказы и выручка вниз.",
			},
		},
		SKUFocus: SKUFocus{
			OrderDrop: []SKUFocusCard{{OzonProductID: 1, ProductName: "A", Problem: "Падение"}},
			KeySKUs:   []KeySkuCard{{OzonProductID: 1, ProductName: "A", RevenueSharePct: 55}},
			Surplus:   []SKUFocusCard{{OzonProductID: 9, ProductName: "SurplusKeep"}},
		},
		TopActions: []TopActionItem{
			{ActionKey: "a1", ActionID: 11, Title: "Пополнить A", Priority: "critical"},
			{ActionKey: "a2", ActionID: 12, Title: "Пополнить B", Priority: "important"},
		},
	}

	// First finalize may apply presentation-only helpers (labels, health status
	// codes). Subsequent Unified GET finalizes must not change semantics.
	finalizeSellerFacingDashboardResponse(&resp)
	baseline := semanticFingerprint(resp)
	for i := 0; i < 3; i++ {
		finalizeSellerFacingDashboardResponse(&resp)
		got := semanticFingerprint(resp)
		if got != baseline {
			t.Fatalf("pass %d mutated semantic fingerprint\nbaseline=%s\ngot=%s", i+1, baseline, got)
		}
	}
	if len(resp.SKUFocus.Surplus) != 1 {
		t.Fatalf("surplus taxonomy mutated on unified GET: %+v", resp.SKUFocus.Surplus)
	}
	if len(resp.AIDiagnostics.TopActions) != 2 || resp.AIDiagnostics.TopActions[0].ActionKey != "a1" {
		t.Fatalf("top actions mutated: %+v", resp.AIDiagnostics.TopActions)
	}
}
