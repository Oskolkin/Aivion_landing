package aipipeline

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

const fakeDashboardModel = "fake-dashboard-deterministic"

type fakeDashboardNarrativePayload struct {
	Headline           string                      `json:"headline"`
	ExecutiveSummary   string                      `json:"executive_summary"`
	HealthReasons      []fakeDashboardHealthReason `json:"health_reasons"`
	ImprovementSummary string                      `json:"improvement_summary"`
	WhatChanged        *fakeDashboardWhatChanged   `json:"what_changed,omitempty"`
	TopActions         []fakeDashboardTopAction    `json:"top_actions"`
	DataQualityNotes   []string                    `json:"data_quality_notes"`
}

type fakeDashboardHealthReason struct {
	DriverID string `json:"driver_id"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
}

type fakeDashboardWhatChanged struct {
	Summary string `json:"summary"`
}

type fakeDashboardTopAction struct {
	DriverID          string `json:"driver_id"`
	SourceCandidateID string `json:"source_candidate_id"`
	Priority          string `json:"priority"`
	Category          string `json:"category"`
	Title             string `json:"title"`
	Reason            string `json:"reason"`
	NextStep          string `json:"next_step"`
	ExpectedEffect    string `json:"expected_effect"`
}

func buildFakeDashboardPayload(globalCtx *aicontext.GlobalSemanticContext, metricDate string) *fakeDashboardNarrativePayload {
	if globalCtx == nil {
		return &fakeDashboardNarrativePayload{
			Headline:         "Fake dashboard narrative",
			ExecutiveSummary: "Deterministic offline dashboard placeholder.",
		}
	}

	headline := globalCtx.OverallStory
	if len(globalCtx.PrimaryRisks) > 0 && strings.TrimSpace(globalCtx.PrimaryRisks[0].Title) != "" {
		headline = globalCtx.PrimaryRisks[0].Title
	}
	if len(headline) > 160 {
		headline = headline[:160]
	}

	healthReasons := make([]fakeDashboardHealthReason, 0, len(globalCtx.PrimaryRisks)+len(globalCtx.SecondaryRisks))
	for i, risk := range globalCtx.PrimaryRisks {
		healthReasons = append(healthReasons, fakeDashboardHealthReason{
			DriverID: fmt.Sprintf("driver-%s-%d", risk.Category, i+1),
			Category: risk.Category,
			Title:    risk.Title,
			Reason:   risk.Rationale,
		})
	}
	for i, risk := range globalCtx.SecondaryRisks {
		if len(healthReasons) >= 5 {
			break
		}
		healthReasons = append(healthReasons, fakeDashboardHealthReason{
			DriverID: fmt.Sprintf("driver-secondary-%s-%d", risk.Category, i+1),
			Category: risk.Category,
			Title:    risk.Title,
			Reason:   risk.Rationale,
		})
	}

	topActions := buildFakeDashboardTopActions(globalCtx)
	whatChanged := buildFakeDashboardWhatChanged(globalCtx, metricDate)

	limitations := append([]string(nil), globalCtx.DataLimitations...)
	if len(limitations) == 0 {
		limitations = []string{"Offline fake dashboard: ограничения данных взяты из Global Context."}
	}

	return &fakeDashboardNarrativePayload{
		Headline:           headline,
		ExecutiveSummary:   globalCtx.OverallStory,
		HealthReasons:      healthReasons,
		ImprovementSummary: buildFakeImprovementSummary(globalCtx),
		WhatChanged:        whatChanged,
		TopActions:         topActions,
		DataQualityNotes:   limitations,
	}
}

func buildFakeDashboardTopActions(globalCtx *aicontext.GlobalSemanticContext) []fakeDashboardTopAction {
	out := make([]fakeDashboardTopAction, 0, 3)
	for i, risk := range globalCtx.PrimaryRisks {
		if len(out) >= 3 {
			break
		}
		driverID := driverIDForCategory(risk.Category)
		out = append(out, fakeDashboardTopAction{
			DriverID:          driverID,
			SourceCandidateID: fmt.Sprintf("fake-candidate-%s-%d", risk.Category, i+1),
			Priority:          "high",
			Category:          risk.Category,
			Title:             fmt.Sprintf("Проверить %s", categoryLabel(risk.Category)),
			Reason:            risk.Rationale,
			NextStep:          "Сверить метрики и приоритет из Global Context.",
			ExpectedEffect:    "Снизить риск по направлению " + categoryLabel(risk.Category) + ".",
		})
	}
	if len(out) == 0 {
		out = append(out, fakeDashboardTopAction{
			DriverID:          "driver-advertising",
			SourceCandidateID: "fake-candidate-advertising-1",
			Priority:          "medium",
			Category:          "advertising",
			Title:             "Проверить рекламу",
			Reason:            "Placeholder top action из Global Context (offline fake).",
			NextStep:          "Сверить кампании с приоритетами кабинета.",
			ExpectedEffect:    "Удержать рекламные риски под контролем.",
		})
	}
	return out
}

func buildFakeDashboardWhatChanged(globalCtx *aicontext.GlobalSemanticContext, metricDate string) *fakeDashboardWhatChanged {
	priority := strings.Join(globalCtx.PriorityOrder, ", ")
	if priority == "" {
		priority = "advertising, stock"
	}
	summary := fmt.Sprintf(
		"На %s приоритетные направления: %s. Сводка построена из Global Context (offline fake).",
		metricDate,
		priority,
	)
	return &fakeDashboardWhatChanged{Summary: summary}
}

func buildFakeImprovementSummary(globalCtx *aicontext.GlobalSemanticContext) string {
	if len(globalCtx.Opportunities) > 0 {
		return "Точки опоры: " + globalCtx.Opportunities[0].Rationale
	}
	return "Индекс улучшится после работы с приоритетными рисками из Global Context."
}

func driverIDForCategory(category string) string {
	switch category {
	case "stock", "slow_turnover":
		return "driver-stock"
	case "advertising":
		return "driver-advertising"
	case "funnel":
		return "driver-funnel"
	case "sku", "sales":
		return "driver-sku"
	default:
		return "driver-" + category
	}
}

func categoryLabel(category string) string {
	switch category {
	case "stock":
		return "остатки"
	case "slow_turnover":
		return "медленный оборот"
	case "advertising":
		return "рекламу"
	case "funnel":
		return "воронку"
	case "sku":
		return "SKU"
	case "sales":
		return "продажи"
	default:
		return category
	}
}

func fakeDashboardHealthScore(globalCtx *aicontext.GlobalSemanticContext) int32 {
	score := int32(72)
	if globalCtx != nil {
		score -= int32(len(globalCtx.PrimaryRisks) * 6)
		score -= int32(len(globalCtx.SecondaryRisks) * 2)
	}
	if score < 28 {
		return 28
	}
	if score > 88 {
		return 88
	}
	return score
}

func fakeDashboardSeverity(globalCtx *aicontext.GlobalSemanticContext) string {
	if globalCtx != nil && len(globalCtx.PrimaryRisks) >= 2 {
		return "high"
	}
	return "medium"
}
