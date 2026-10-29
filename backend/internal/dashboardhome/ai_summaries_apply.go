package dashboardhome

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type storedAISummaryRow struct {
	ID          string
	SummaryType string
	ScopeID     string
	ScopeName   string
	Status      string
	Headline    string
	HealthScore *int32
	SummaryJSON []byte
	Validation  []string
}

func shouldExposeAISummariesDebug() bool {
	return shouldExposeNarrativeStaleDebug() || strings.TrimSpace(os.Getenv("DASHBOARD_AI_SUMMARIES_DEBUG")) == "1"
}

func acceptStoredAccountSummaryPayload(row storedAISummaryRow) (aisummaries.SummaryPayload, aisummaries.SummaryAcceptanceOutcome, bool) {
	var payload aisummaries.SummaryPayload
	if len(row.SummaryJSON) > 0 {
		_ = json.Unmarshal(row.SummaryJSON, &payload)
	}
	outcome := aisummaries.ReacceptSummaryPayload(&payload, aisummaries.SummaryRepairContext{
		SummaryType: "account_daily_summary",
	})
	if !outcome.Accepted {
		return payload, outcome, false
	}
	return *outcome.Payload, outcome, true
}

func acceptStoredCategoryPayload(row storedAISummaryRow, categoryName string) (aisummaries.SummaryPayload, aisummaries.SummaryAcceptanceOutcome, bool) {
	var payload aisummaries.SummaryPayload
	if len(row.SummaryJSON) > 0 {
		_ = json.Unmarshal(row.SummaryJSON, &payload)
	}
	name := categoryName
	if name == "" {
		name = row.ScopeName
	}
	outcome := aisummaries.ReacceptSummaryPayload(&payload, aisummaries.SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      row.ScopeID,
		CategoryName: name,
	})
	if !outcome.Accepted {
		return payload, outcome, false
	}
	return *outcome.Payload, outcome, true
}

func buildCategorySummaryCard(row storedAISummaryRow, fallbackName string) (CategorySummaryCard, CategoryDebugRow) {
	debug := CategoryDebugRow{
		ScopeID: row.ScopeID,
		Status:  row.Status,
	}
	name := strings.TrimSpace(row.ScopeName)
	if name == "" {
		name = fallbackName
	}
	debug.CategoryName = name

	if row.Status != "completed" {
		debug.FallbackReason = "row_status_not_completed"
		return buildFallbackCategoryCard(row, name), debug
	}

	payload, outcome, ok := acceptStoredCategoryPayload(row, name)
	debug.ValidationStatus = outcome.ValidationStatus
	debug.RepairableErrors = outcome.Repair.RepairableErrors
	if !ok {
		debug.FallbackReason = firstNonEmpty(strings.Join(outcome.Repair.HardErrors, ","), "validation_failed")
		return buildFallbackCategoryCard(row, name), debug
	}
	debug.Accepted = true

	card := CategorySummaryCard{
		CategoryID:   row.ScopeID,
		CategoryName: name,
		Severity:     payload.Severity,
		Headline:     firstNonEmpty(payload.Headline, row.Headline),
	}
	if row.HealthScore != nil {
		card.HealthScore = row.HealthScore
	} else if payload.HealthScore >= 0 && payload.HealthScore <= 100 {
		v := int32(payload.HealthScore)
		card.HealthScore = &v
	}
	if strings.TrimSpace(payload.KeyRisk) != "" {
		card.KeyRisk = payload.KeyRisk
	} else if len(payload.WhatChangedSincePreviousSummary.NewRisks) > 0 {
		card.KeyRisk = payload.WhatChangedSincePreviousSummary.NewRisks[0]
	}
	if strings.TrimSpace(payload.RecommendedAction) != "" {
		card.RecommendedAction = payload.RecommendedAction
	} else if len(payload.RecommendedActions) > 0 {
		card.RecommendedAction = payload.RecommendedActions[0].Action
	}
	if card.Headline == "" {
		card.Headline = "Сводка по категории «" + name + "»"
		debug.FallbackReason = "headline_filled_on_apply"
	}
	return card, debug
}

func buildFallbackCategoryCard(row storedAISummaryRow, name string) CategorySummaryCard {
	card := CategorySummaryCard{
		CategoryID:   row.ScopeID,
		CategoryName: name,
		Headline:     strings.TrimSpace(row.Headline),
	}
	if card.Headline == "" {
		card.Headline = "Сводка по категории «" + name + "»"
	}
	if row.HealthScore != nil {
		card.HealthScore = row.HealthScore
	}
	return card
}

func funnelAIAnalysisFromAcceptedPayload(payload aisummaries.SummaryPayload, outcome aisummaries.SummaryAcceptanceOutcome) (FunnelAIAnalysisWidget, string) {
	widget := funnelAIAnalysisWidgetFromPayload(payload)
	if payload.FunnelAIAnalysis == nil {
		return widget, "missing_funnel_ai_analysis"
	}
	if len(outcome.Repair.FallbackBlocks) > 0 {
		return widget, strings.Join(outcome.Repair.FallbackBlocks, ",")
	}
	if widget.Status != WidgetAvailable && widget.Status != WidgetNotEnoughData {
		return widget, string(widget.Status)
	}
	if widget.Status == WidgetNotEnoughData {
		return widget, "funnel_not_enough_data"
	}
	return widget, ""
}

func buildAIActionsFromAcceptedPayload(payload aisummaries.SummaryPayload, limit int) ([]ActionItem, aisummaries.RecommendedDebug) {
	debug := aisummaries.RecommendedDebug{}
	reaccept := aisummaries.ReacceptSummaryPayload(&payload, aisummaries.SummaryRepairContext{SummaryType: "account_daily_summary"})
	if reaccept.Payload != nil {
		payload = *reaccept.Payload
	}
	debug.AIReceived = len(payload.RecommendedActions)
	out := make([]ActionItem, 0, limit)
	for _, a := range payload.RecommendedActions {
		if strings.TrimSpace(a.Action) == "" {
			debug.Dropped++
			continue
		}
		out = append(out, ActionItem{
			Priority:          a.Priority,
			UrgencyGroup:      urgencyGroupForPriority(a.Priority),
			Action:            a.Action,
			Reason:            a.Reason,
			ExpectedEffect:    a.ExpectedEffect,
			RelatedEntityType: a.RelatedEntityType,
			RelatedEntityID:   a.RelatedEntityID,
			Source:            "ai_summary",
		})
		if len(out) >= limit {
			break
		}
	}
	debug.AIAccepted = len(out)
	debug.Dropped += reaccept.Repair.DroppedActions
	return out, debug
}

func storedRowFromDB(row dbgen.AiDailySummary) storedAISummaryRow {
	stored := storedAISummaryRow{
		SummaryType: row.SummaryType,
		ScopeID:     row.ScopeID,
		Status:      row.Status,
		SummaryJSON: append([]byte(nil), row.SummaryJson...),
	}
	if row.ID.Valid {
		stored.ID = row.ID.String()
	}
	if row.ScopeName.Valid {
		stored.ScopeName = row.ScopeName.String
	}
	if row.Headline.Valid {
		stored.Headline = row.Headline.String
	}
	if row.HealthScore.Valid {
		v := row.HealthScore.Int32
		stored.HealthScore = &v
	}
	if len(row.ValidationErrors) > 0 {
		_ = json.Unmarshal(row.ValidationErrors, &stored.Validation)
	}
	return stored
}
