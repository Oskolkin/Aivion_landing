package dashboardhome

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const stage4PipelineSchemaVersion = "dashboard_stage4_pipeline_v1"

type Stage4BlockValidationEntry struct {
	Block  string   `json:"block"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type Stage4PipelineLog struct {
	SchemaVersion    string                       `json:"schema_version"`
	DomainSignals    int                          `json:"domain_signals"`
	GlobalContextOK  bool                         `json:"global_context_ok"`
	BlockValidations []Stage4BlockValidationEntry `json:"block_validations"`
	CrossBlockOK     bool                         `json:"cross_block_ok"`
	CrossBlockIssues []string                     `json:"cross_block_issues,omitempty"`
	RetryLog         []string                     `json:"retry_log,omitempty"`
	ValidationLog    []string                     `json:"validation_log,omitempty"`
}

type Stage4PipelineResult struct {
	Payload          *dashboardNarrativePayload
	Log              Stage4PipelineLog
	Accepted         bool
	Repaired         bool
	HardErrors       []string
	RepairableErrors []string
	Validation       DashboardValidationResult
}

var stage4RequiredBlocks = []string{
	"account_health",
	"what_changed",
	"top_actions",
	"seller_pulse_narrative",
	"stock_coverage_narrative",
	"ad_efficiency_narrative",
	"funnel_ai_analysis",
	"category_summaries",
	"risk_overview",
}

func runStage4Pipeline(
	ctx context.Context,
	o *DataPackChunksOrchestrator,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	build dashboardNarrativeBuildResult,
) Stage4PipelineResult {
	out := Stage4PipelineResult{
		Log: Stage4PipelineLog{SchemaVersion: stage4PipelineSchemaVersion},
	}
	if o == nil || pack == nil || chunkSet == nil {
		out.HardErrors = []string{"missing_pipeline_inputs"}
		return out
	}

	gen := o.Generate(ctx, pack, chunkSet, build, NarrativeRunOptions{})
	out.Log.DomainSignals = gen.ChunkRun.Completed
	out.Log.GlobalContextOK = gen.Error == nil && gen.Payload != nil
	out.Log.ValidationLog = append(out.Log.ValidationLog, fmt.Sprintf("synthesis_status=%s", gen.Pipeline.ChunkSynthesisStatus))

	if gen.Fatal || gen.Payload == nil {
		if gen.Error != nil {
			out.HardErrors = []string{gen.Error.Error()}
		} else {
			out.HardErrors = []string{"generation_failed"}
		}
		return out
	}

	payload := gen.Payload
	accepted, repaired, repair, hard := acceptGeneratedDashboardNarrative(payload, build)
	out.Repaired = repair.Repaired
	out.RepairableErrors = repair.RepairableErrors
	if !accepted {
		out.HardErrors = append(hard, repair.RepairableErrors...)
		out.Log.ValidationLog = append(out.Log.ValidationLog, "acceptance_failed")
		return out
	}

	blockResult := validateStage4Blocks(repaired)
	out.Log.BlockValidations = blockResult
	for _, entry := range blockResult {
		if !entry.Valid {
			out.Log.ValidationLog = append(out.Log.ValidationLog, fmt.Sprintf("block_invalid:%s", entry.Block))
		}
	}
	mode := CurrentDashboardValidationMode()
	blockValidation := evaluateStage4BlockValidation(repaired, mode)
	crossValidation := evaluateStage4CrossBlockReview(repaired, mode)
	out.Validation = mergeDashboardValidationResults(blockValidation, crossValidation)
	out.Log.CrossBlockIssues = crossValidation.WarningCodes()
	out.Log.CrossBlockIssues = append(out.Log.CrossBlockIssues, crossValidation.BlockingCodes()...)
	out.Log.CrossBlockOK = out.Validation.Valid && len(out.Validation.Warnings) == 0
	out.Log.ValidationLog = append(out.Log.ValidationLog, crossBlockReviewLog(out.Log.CrossBlockIssues)...)
	if !out.Validation.Valid {
		out.HardErrors = out.Validation.BlockingCodes()
		return out
	}
	if len(out.Validation.Warnings) > 0 {
		out.Log.ValidationLog = append(out.Log.ValidationLog, dashboardValidationLogMessage(out.Validation, "cross_block_review_completed_with_warnings"))
	}

	out.Payload = repaired
	out.Accepted = true
	out.Log.ValidationLog = append(out.Log.ValidationLog, "pipeline_completed")
	return out
}

func validateStage4Blocks(payload *dashboardNarrativePayload) []Stage4BlockValidationEntry {
	out := make([]Stage4BlockValidationEntry, 0, len(stage4RequiredBlocks))
	for _, block := range stage4RequiredBlocks {
		entry := Stage4BlockValidationEntry{Block: block, Valid: true}
		switch block {
		case "account_health":
			entry.Errors = validateAIOnlyAccountHealth(payload)
		case "what_changed":
			if payload == nil || payload.WhatChanged == nil || len(payload.WhatChanged.Groups) == 0 {
				entry.Errors = []string{"missing_what_changed"}
			}
		case "top_actions":
			if payload == nil || len(payload.TopActions) < 3 {
				entry.Errors = []string{"top_actions_lt_3"}
			}
		case "seller_pulse_narrative":
			if payload == nil || payload.SellerPulseNarrative == nil || strings.TrimSpace(payload.SellerPulseNarrative.Summary) == "" {
				entry.Errors = []string{"missing_seller_pulse_narrative"}
			}
		case "stock_coverage_narrative":
			if payload == nil || payload.StockCoverageNarrative == nil || strings.TrimSpace(payload.StockCoverageNarrative.Summary) == "" {
				entry.Errors = []string{"missing_stock_coverage_narrative"}
			}
		case "ad_efficiency_narrative":
			if payload == nil || payload.AdEfficiencyNarrative == nil || strings.TrimSpace(payload.AdEfficiencyNarrative.Summary) == "" {
				entry.Errors = []string{"missing_ad_efficiency_narrative"}
			}
		case "funnel_ai_analysis":
			if payload == nil || payload.FunnelAIAnalysis == nil || payload.FunnelAIAnalysis.Status != WidgetAvailable {
				entry.Errors = []string{"invalid_funnel_ai_analysis"}
			}
		case "category_summaries":
			entry.Errors = validateAIOnlyCategoryScoresRequired(payload)
			if payload == nil || len(payload.CategorySummaries) == 0 {
				entry.Errors = append(entry.Errors, "missing_category_summaries")
			}
		case "risk_overview":
			if payload == nil || payload.RiskOverview == nil || payload.RiskOverview.Status != WidgetAvailable {
				entry.Errors = []string{"invalid_risk_overview"}
			}
		}
		if len(entry.Errors) > 0 {
			entry.Valid = false
		}
		out = append(out, entry)
	}
	return out
}

func allStage4BlocksValid(entries []Stage4BlockValidationEntry) bool {
	for _, e := range entries {
		if !e.Valid {
			return false
		}
	}
	return true
}

func collectStage4BlockErrors(entries []Stage4BlockValidationEntry) []string {
	out := make([]string, 0)
	for _, e := range entries {
		for _, err := range e.Errors {
			out = append(out, e.Block+":"+err)
		}
	}
	return out
}

func runStage4CrossBlockReview(payload *dashboardNarrativePayload) []string {
	if payload == nil {
		return []string{"nil_payload"}
	}
	issues := make([]string, 0, 4)
	contentIssues := validateAIOnlyContentBlocks(payload)
	for _, issue := range contentIssues {
		issues = append(issues, issue)
	}
	if payload.AccountHealth != nil && strings.TrimSpace(payload.Headline) != "" &&
		strings.TrimSpace(payload.AccountHealth.Headline) != "" &&
		payload.Headline != payload.AccountHealth.Headline {
		issues = append(issues, "headline_mismatch_between_root_and_account_health")
	}
	return issues
}

func crossBlockReviewLog(issues []string) []string {
	if len(issues) == 0 {
		return []string{"cross_block_review_passed"}
	}
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, "cross_block_issue:"+issue)
	}
	return out
}

func encodeStage4PipelineLog(log Stage4PipelineLog) string {
	raw, err := json.Marshal(log)
	if err != nil {
		return ""
	}
	return string(raw)
}
