package dashboardhome

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	DashboardValidationModePilotStructural = "pilot_structural"
	DashboardValidationModeStrict          = "strict"
)

// DashboardValidationIssueSeverity classifies whether an issue blocks acceptance.
type DashboardValidationIssueSeverity string

const (
	DashboardIssueSeverityBlocking DashboardValidationIssueSeverity = "blocking"
	DashboardIssueSeverityWarning  DashboardValidationIssueSeverity = "warning"
)

// DashboardValidationIssue is a single dashboard validation finding.
type DashboardValidationIssue struct {
	Code     string                           `json:"code"`
	Message  string                           `json:"message,omitempty"`
	Path     string                           `json:"path,omitempty"`
	Severity DashboardValidationIssueSeverity `json:"severity"`
	Source   string                           `json:"source,omitempty"`
}

// DashboardValidationResult is the structured outcome of dashboard validation.
type DashboardValidationResult struct {
	Mode           string                     `json:"mode"`
	Valid          bool                       `json:"valid"`
	BlockingErrors []DashboardValidationIssue `json:"blocking_errors"`
	Warnings       []DashboardValidationIssue `json:"warnings"`
}

func (r DashboardValidationResult) BlockingCodes() []string {
	out := make([]string, 0, len(r.BlockingErrors))
	for _, issue := range r.BlockingErrors {
		if issue.Code != "" {
			out = append(out, issue.Code)
		}
	}
	return out
}

func (r DashboardValidationResult) WarningCodes() []string {
	out := make([]string, 0, len(r.Warnings))
	for _, issue := range r.Warnings {
		if issue.Code != "" {
			out = append(out, issue.Code)
		}
	}
	return out
}

func (r DashboardValidationResult) Status() string {
	if !r.Valid {
		return "failed"
	}
	if len(r.Warnings) > 0 {
		return "completed_with_warnings"
	}
	return "completed"
}

func (r DashboardValidationResult) Stage4WarningCount() int {
	n := 0
	for _, issue := range r.Warnings {
		if issue.Source == "stage4_cross_block" || issue.Source == "stage4_blocks" {
			n++
		}
	}
	return n
}

// CurrentDashboardValidationMode reads DASHBOARD_VALIDATION_MODE from the environment.
// Unknown values fall back to pilot_structural with a startup warning.
func CurrentDashboardValidationMode() string {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("DASHBOARD_VALIDATION_MODE")))
	switch raw {
	case "", DashboardValidationModePilotStructural:
		return DashboardValidationModePilotStructural
	case DashboardValidationModeStrict:
		return DashboardValidationModeStrict
	default:
		slog.Warn("unknown dashboard validation mode; using pilot_structural", "value", raw)
		return DashboardValidationModePilotStructural
	}
}

func dashboardValidationModeOrDefault(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DashboardValidationModeStrict:
		return DashboardValidationModeStrict
	case DashboardValidationModePilotStructural, "":
		return DashboardValidationModePilotStructural
	default:
		return DashboardValidationModePilotStructural
	}
}

func isPilotStructuralMode(mode string) bool {
	return dashboardValidationModeOrDefault(mode) == DashboardValidationModePilotStructural
}

// dashboardPilotStructuralBlockingAllowlist is the positive allowlist of issue codes
// that may block Dashboard acceptance in pilot_structural mode.
// Any code not in this set is automatically a warning.
var dashboardPilotStructuralBlockingAllowlist = map[string]struct{}{
	"nil_payload":                      {},
	"nil_dashboard_narrative_payload":  {},
	"invalid_json":                     {},
	"invalid_root_type":                {},
	"openai_output_incomplete_length":  {},
	"provider_response_empty":          {},
	"generation_failed":                {},
	"missing_pipeline_inputs":          {},
	"missing_account_summary_headline": {},
	"missing_account_health":           {},
	"required_root_missing":            {},
	"required_field_missing":           {},
	"required_block_missing":           {},
	"required_field_wrong_type":        {},
	"dashboard_unmarshal_failed":       {},
	"dashboard_mapping_failed":         {},
	"dashboard_serialization_failed":   {},
}

func normalizeDashboardIssueCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if i := strings.Index(code, ":"); i >= 0 {
		return strings.TrimSpace(code[i+1:])
	}
	return code
}

func isDashboardPilotStructuralBlockingCode(code string) bool {
	code = normalizeDashboardIssueCode(code)
	if code == "" {
		return false
	}
	_, ok := dashboardPilotStructuralBlockingAllowlist[code]
	return ok
}

func isDashboardIssueBlocking(code, issueSeverity, mode string) bool {
	if dashboardValidationModeOrDefault(mode) == DashboardValidationModeStrict {
		if strings.EqualFold(strings.TrimSpace(issueSeverity), "warning") {
			return false
		}
		return true
	}
	return isDashboardPilotStructuralBlockingCode(code)
}

func classifyConsistencyIssues(
	issues []DashboardHomeConsistencyIssue,
	mode string,
	source string,
) DashboardValidationResult {
	mode = dashboardValidationModeOrDefault(mode)
	result := DashboardValidationResult{
		Mode:  mode,
		Valid: true,
	}
	for _, issue := range issues {
		code := strings.TrimSpace(issue.Code)
		if code == "" {
			continue
		}
		entry := DashboardValidationIssue{
			Code:     code,
			Message:  issue.Message,
			Severity: DashboardIssueSeverityWarning,
			Source:   source,
		}
		if isDashboardIssueBlocking(code, issue.Severity, mode) {
			entry.Severity = DashboardIssueSeverityBlocking
			result.BlockingErrors = append(result.BlockingErrors, entry)
			result.Valid = false
			continue
		}
		result.Warnings = append(result.Warnings, entry)
	}
	return result
}

func classifyBlockingCodes(codes []string, mode, source string) DashboardValidationResult {
	mode = dashboardValidationModeOrDefault(mode)
	result := DashboardValidationResult{
		Mode:  mode,
		Valid: true,
	}
	for _, raw := range codes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		code := normalizeDashboardIssueCode(raw)
		if code == "" {
			code = raw
		}
		entry := DashboardValidationIssue{
			Code:     code,
			Message:  raw,
			Severity: DashboardIssueSeverityWarning,
			Source:   source,
		}
		if isDashboardIssueBlocking(code, "failed", mode) {
			entry.Severity = DashboardIssueSeverityBlocking
			result.BlockingErrors = append(result.BlockingErrors, entry)
			result.Valid = false
			continue
		}
		result.Warnings = append(result.Warnings, entry)
	}
	return result
}

func classifyStringIssues(codes []string, mode, source string) DashboardValidationResult {
	return classifyBlockingCodes(codes, mode, source)
}

func mergeDashboardValidationResults(parts ...DashboardValidationResult) DashboardValidationResult {
	out := DashboardValidationResult{
		Mode:  DashboardValidationModePilotStructural,
		Valid: true,
	}
	for _, part := range parts {
		if part.Mode != "" {
			out.Mode = part.Mode
		}
		out.BlockingErrors = append(out.BlockingErrors, part.BlockingErrors...)
		out.Warnings = append(out.Warnings, part.Warnings...)
		if !part.Valid {
			out.Valid = false
		}
	}
	return out
}

func evaluateAIOnlyContentValidation(payload *dashboardNarrativePayload, mode string) DashboardValidationResult {
	issues := collectAIOnlySemanticContentIssues(payload)
	return classifyConsistencyIssues(issues, mode, "content_validation")
}

func collectAIOnlySemanticContentIssues(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil {
		return nil
	}
	issues := ValidateDashboardAIOnlyContent(payload, DashboardSynthesisInput{}, nil)
	issues = append(issues, validateAIOnlyV61Content(payload, DashboardSynthesisInput{})...)
	return issues
}

func evaluateStage4CrossBlockReview(payload *dashboardNarrativePayload, mode string) DashboardValidationResult {
	if payload == nil {
		return classifyStringIssues([]string{"nil_payload"}, mode, "stage4_cross_block")
	}
	issues := collectAIOnlySemanticContentIssues(payload)
	if payload.AccountHealth != nil &&
		strings.TrimSpace(payload.Headline) != "" &&
		strings.TrimSpace(payload.AccountHealth.Headline) != "" &&
		payload.Headline != payload.AccountHealth.Headline {
		issues = append(issues, DashboardHomeConsistencyIssue{
			Code:     "headline_mismatch_between_root_and_account_health",
			Severity: "failed",
			Message:  "root headline differs from account_health headline",
		})
	}
	return classifyConsistencyIssues(issues, mode, "stage4_cross_block")
}

func evaluateStage4BlockValidation(payload *dashboardNarrativePayload, mode string) DashboardValidationResult {
	blockResult := validateStage4Blocks(payload)
	return classifyBlockingCodes(collectStage4BlockErrors(blockResult), mode, "stage4_blocks")
}

func blockingErrorsFromStrings(codes []string, source string) []DashboardValidationIssue {
	out := make([]DashboardValidationIssue, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		out = append(out, DashboardValidationIssue{
			Code:     normalizeDashboardIssueCode(code),
			Message:  code,
			Severity: DashboardIssueSeverityBlocking,
			Source:   source,
		})
	}
	return out
}

func dashboardValidationResultFromBlocking(codes []string, mode, source string) DashboardValidationResult {
	return classifyBlockingCodes(codes, mode, source)
}

func dashboardValidationArtifactPayload(result DashboardValidationResult) map[string]any {
	blocking := make([]map[string]any, 0, len(result.BlockingErrors))
	for _, issue := range result.BlockingErrors {
		blocking = append(blocking, map[string]any{
			"code":     issue.Code,
			"message":  issue.Message,
			"severity": issue.Severity,
			"source":   issue.Source,
		})
	}
	warnings := make([]map[string]any, 0, len(result.Warnings))
	for _, issue := range result.Warnings {
		warnings = append(warnings, map[string]any{
			"code":     issue.Code,
			"message":  issue.Message,
			"severity": issue.Severity,
			"source":   issue.Source,
		})
	}
	return map[string]any{
		"mode":            result.Mode,
		"valid":           result.Valid,
		"accepted":        result.Valid,
		"blocking_errors": blocking,
		"warnings":        warnings,
		"status":          result.Status(),
		"warning_count":   len(result.Warnings),
		"error_count":     len(result.BlockingErrors),
		"warning_codes":   result.WarningCodes(),
		"error_codes":     result.BlockingCodes(),
		"stage4_warnings": result.Stage4WarningCount(),
	}
}

func dashboardValidationLogMessage(result DashboardValidationResult, stage string) string {
	if !result.Valid {
		return fmt.Sprintf("%s failed: %s", stage, strings.Join(result.BlockingCodes(), ", "))
	}
	if len(result.Warnings) > 0 {
		return fmt.Sprintf("%s completed with warnings: %s", stage, strings.Join(result.WarningCodes(), ", "))
	}
	return stage + " passed"
}

// DashboardValidationArtifactFromResult builds the pipeline validation artifact payload.
func DashboardValidationArtifactFromResult(
	mode string,
	status string,
	warningCodes []string,
	errorCodes []string,
	warningCount int,
	errorCount int,
	stage4WarningCount int,
) map[string]any {
	valid := errorCount == 0
	warnings := make([]map[string]any, 0, len(warningCodes))
	for _, code := range warningCodes {
		warnings = append(warnings, map[string]any{"code": code})
	}
	blocking := make([]map[string]any, 0, len(errorCodes))
	for _, code := range errorCodes {
		blocking = append(blocking, map[string]any{"code": code})
	}
	if mode == "" {
		mode = CurrentDashboardValidationMode()
	}
	if status == "" {
		if valid && warningCount > 0 {
			status = "completed_with_warnings"
		} else if valid {
			status = "completed"
		} else {
			status = "failed"
		}
	}
	return map[string]any{
		"mode":            mode,
		"valid":           valid,
		"accepted":        valid,
		"status":          status,
		"blocking_errors": blocking,
		"warnings":        warnings,
		"warning_count":   warningCount,
		"error_count":     errorCount,
		"warning_codes":   warningCodes,
		"error_codes":     errorCodes,
		"stage4_warnings": stage4WarningCount,
	}
}
