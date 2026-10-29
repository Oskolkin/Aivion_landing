package dashboardhome

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type dashboardAccountHealthBlock struct {
	HealthScore          *int32                             `json:"health_score,omitempty"`
	Severity             string                             `json:"severity"`
	HealthStatusCode     string                             `json:"health_status_code,omitempty"`
	HealthStatusLabel    string                             `json:"health_status_label,omitempty"`
	HealthStatusExplain  string                             `json:"health_status_explanation,omitempty"`
	Headline             string                             `json:"headline,omitempty"`
	ExecutiveSummary     string                             `json:"executive_summary,omitempty"`
	ImprovementSummary   string                             `json:"improvement_summary,omitempty"`
	ImprovementLevers    []ImprovementLever                 `json:"improvement_levers,omitempty"`
	RiskDrivers          []dashboardNarrativeRiskDriverView `json:"risk_drivers"`
	HealthScoreBreakdown *HealthScoreBreakdown              `json:"health_score_breakdown,omitempty"`
	// HealthPolicyVersion/HealthFactsHash mirror the same fields the Unified
	// publication adapter stores (see adapter.NarrativeAccountHealth). They
	// round-trip through the persisted narrative JSON so the legacy GET path
	// can also surface which backend health policy/facts snapshot produced
	// health_score.
	HealthPolicyVersion string `json:"health_policy_version,omitempty"`
	HealthFactsHash     string `json:"health_facts_hash,omitempty"`
}

type aiAccountHealthApplyResult struct {
	HealthScore          *int32
	Severity             string
	HealthStatusCode     string
	HealthStatusLabel    string
	HealthStatusExplain  string
	Headline             string
	ExecutiveSummary     string
	ImprovementSummary   string
	ImprovementLevers    []ImprovementLever
	RiskDrivers          []RiskDriver
	HealthScoreBreakdown *HealthScoreBreakdown
	HealthPolicyVersion  string
	HealthFactsHash      string
}

func applyUnifiedAccountHealthAIOnly(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
	previousSnapshot *accountMetricsSnapshot,
	narrativeBuildOverride *dashboardNarrativeBuildResult,
) {
	accountSummary := resp.AIDiagnostics.AccountSummary
	accountSummary.WidgetBase = WidgetBase{Status: WidgetAvailable}
	accountSummary.MetricDate = metricDate.UTC().Format("2006-01-02")
	accountSummary.HealthScoreBreakdown = nil

	var build dashboardNarrativeBuildResult
	if narrativeBuildOverride != nil {
		build = *narrativeBuildOverride
	} else {
		build = buildDashboardNarrativeContextMinimalForAIOnly(metricDate, dto, resp, recs, alertRows, previousSnapshot)
	}
	accountSummary.narrativeBuild = &build

	acceptance := acceptStoredDashboardNarrative(
		accountSummary.storedNarrative,
		build,
		nil,
		actionPool,
		nil,
		topActionsLimit,
	)

	narrativeApplied := acceptance.Applied
	narrativePayload := acceptance.Payload

	if narrativeApplied && narrativePayload != nil {
		health := applyAIAccountHealthFromPayload(narrativePayload)
		accountSummary.HealthScore = health.HealthScore
		accountSummary.Severity = health.Severity
		accountSummary.HealthStatusCode = health.HealthStatusCode
		accountSummary.HealthStatusLabel = health.HealthStatusLabel
		accountSummary.HealthStatusExplain = health.HealthStatusExplain
		accountSummary.RiskDrivers = sanitizeRiskDriversForSellerFacing(health.RiskDrivers, resp)
		accountSummary.Headline = firstNonEmpty(strings.TrimSpace(narrativePayload.Headline), health.Headline)
		accountSummary.ExecutiveSummary = firstNonEmpty(strings.TrimSpace(narrativePayload.ExecutiveSummary), health.ExecutiveSummary)
		accountSummary.ImprovementSummary = firstNonEmpty(strings.TrimSpace(narrativePayload.ImprovementSummary), health.ImprovementSummary)
		accountSummary.ImprovementLevers = health.ImprovementLevers
		if health.HealthScoreBreakdown != nil {
			accountSummary.HealthScoreBreakdown = health.HealthScoreBreakdown
		} else if len(accountSummary.RiskDrivers) > 0 {
			bd := ComputeHealthScoreBreakdown(accountSummary.RiskDrivers)
			accountSummary.HealthScoreBreakdown = &bd
			if accountSummary.HealthScore == nil {
				score := bd.FinalScore
				accountSummary.HealthScore = &score
			}
		}
		accountSummary.HealthPolicyVersion = health.HealthPolicyVersion
		accountSummary.HealthFactsHash = health.HealthFactsHash
		accountSummary.NarrativeSource = "ai"
		accountSummary.ValidationStatus = acceptance.ValidationStatus
		accountSummary.Freshness = acceptance.Freshness
		if accountSummary.storedNarrative != nil {
			accountSummary.PromptVersion = accountSummary.storedNarrative.PromptVersion
			accountSummary.NarrativeRunID = accountSummary.storedNarrative.ID
		}
		resp.AIDiagnostics.TopActions = acceptance.TopActions
	} else {
		accountSummary.NarrativeSource = ""
		accountSummary.HealthScore = nil
		accountSummary.Severity = ""
		accountSummary.RiskDrivers = []RiskDriver{}
		accountSummary.ValidationStatus = acceptance.ValidationStatus
		accountSummary.Freshness = acceptance.Freshness
		if accountSummary.storedNarrative != nil {
			accountSummary.PromptVersion = accountSummary.storedNarrative.PromptVersion
			accountSummary.NarrativeRunID = accountSummary.storedNarrative.ID
		}
		if accountSummary.ValidationStatus == "" {
			accountSummary.ValidationStatus = "failed"
		}
		resp.AIDiagnostics.TopActions = []TopActionItem{}
	}

	resp.AIDiagnostics.AccountSummary = accountSummary
	applyStockCoverageNarrativeToResponse(dto, resp, recs, narrativeApplied, narrativePayload)
	applyAdEfficiencyNarrativeToResponse(resp, narrativeApplied, narrativePayload)
	applyWhatChangedToResponse(resp, narrativePayload, build.WhatChanged, narrativeApplied)
	setBlockApplySource(resp, "account_summary", appliedBlockSource(narrativeApplied))
	applyAIOnlyPayloadToResponse(resp, narrativePayload, narrativeApplied)
	reconcileSellerPulseRisksKPI(resp)
	reconcileRiskDistributionMessage(resp, accountSummary.RiskDrivers)
}

func applyAIAccountHealthFromPayload(payload *dashboardNarrativePayload) aiAccountHealthApplyResult {
	out := aiAccountHealthApplyResult{RiskDrivers: []RiskDriver{}}
	if payload == nil {
		return out
	}
	block := payload.AccountHealth
	if block == nil {
		return out
	}
	// Only set HealthScore when the JSON field was present (non-nil pointer).
	// Do not pointerize an omitted/zero default.
	if block.HealthScore != nil {
		score := *block.HealthScore
		out.HealthScore = &score
	}
	out.Severity = strings.TrimSpace(block.Severity)
	out.HealthStatusCode = strings.TrimSpace(block.HealthStatusCode)
	out.HealthStatusLabel = strings.TrimSpace(block.HealthStatusLabel)
	out.HealthStatusExplain = strings.TrimSpace(block.HealthStatusExplain)
	out.Headline = strings.TrimSpace(block.Headline)
	out.ExecutiveSummary = strings.TrimSpace(block.ExecutiveSummary)
	out.ImprovementSummary = strings.TrimSpace(block.ImprovementSummary)
	out.ImprovementLevers = append([]ImprovementLever(nil), block.ImprovementLevers...)
	out.RiskDrivers = riskDriversFromNarrativeViews(block.RiskDrivers)
	if block.HealthScoreBreakdown != nil {
		bd := *block.HealthScoreBreakdown
		out.HealthScoreBreakdown = &bd
	}
	out.HealthPolicyVersion = strings.TrimSpace(block.HealthPolicyVersion)
	out.HealthFactsHash = strings.TrimSpace(block.HealthFactsHash)
	return out
}

func validateAIOnlyAccountHealth(payload *dashboardNarrativePayload) []string {
	if payload == nil || payload.AccountHealth == nil {
		return []string{"missing_account_health"}
	}
	block := payload.AccountHealth
	if block.HealthScore == nil {
		return []string{"missing_account_health_score"}
	}
	if *block.HealthScore < 0 || *block.HealthScore > 100 {
		return []string{"invalid_account_health_score"}
	}
	if strings.TrimSpace(block.Severity) == "" {
		return []string{"missing_account_health_severity"}
	}
	if len(block.RiskDrivers) == 0 {
		return []string{"missing_account_health_risk_drivers"}
	}
	for _, d := range block.RiskDrivers {
		if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Reason) == "" {
			return []string{"invalid_account_health_risk_driver"}
		}
		if strings.TrimSpace(d.CategoryLabel) == "" {
			return []string{"missing_account_health_category_label"}
		}
	}
	return nil
}

func validateAIOnlyCategoryScoresRequired(payload *dashboardNarrativePayload) []string {
	if payload == nil || len(payload.CategorySummaries) == 0 {
		return nil
	}
	errs := make([]string, 0)
	for i, card := range payload.CategorySummaries {
		if card.CategoryScore == nil || *card.CategoryScore < 0 || *card.CategoryScore > categoryScoreMax {
			errs = append(errs, fmt.Sprintf("missing_category_score_%d", i))
		}
	}
	return errs
}
