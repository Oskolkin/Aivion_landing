package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type readinessCheck struct {
	Name   string
	Pass   bool
	Detail string
}

func runCheckAIRunReadiness(args []string) int {
	fs := flag.NewFlagSet("check-ai-run-readiness", flag.ContinueOnError)
	sellerID := fs.Int64("seller-id", 0, "seller account id")
	metricDate := fs.String("metric-date", "", "metric date YYYY-MM-DD")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sellerID <= 0 || strings.TrimSpace(*metricDate) == "" {
		fmt.Fprintln(os.Stderr, "note: seller-id and metric-date are optional for configuration readiness checks")
	}

	_ = config.LoadEnvFiles()

	checks := []readinessCheck{
		checkReadinessConfigFromEnv(),
		checkGlobalContextOutputLimit(),
		checkMetricSemanticsRegistry(),
		checkChunkPromptVersion(),
		checkActionsTwoPhaseContract(),
		checkActionsComposeOutputLimitWiring(),
		checkPlaceholderValidator(),
		checkGlobalContextPrompt(),
		checkFactRegistryIDs(),
		checkInspectorLegacyAdapter(),
		checkDebugArtifactPersistence(),
		checkFinalDashboardSemanticGuard(),
		checkFinalActionsSemanticGuard(),
		checkPublicationGuard(),
		checkActionsTimeoutTopology(),
		checkInspectorMonitoringSections(),
		checkAICallDiagnosticsEnvelope(),
		checkRawProviderResponseCapture(),
		checkInspectorEntityFlattening(),
		checkInspectorSearchWiring(),
		checkAICallDiagnosticsCoverage(),
	}

	fmt.Println("AI RUN READINESS")
	fmt.Println()
	allPass := true
	for _, c := range checks {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
			allPass = false
		}
		fmt.Printf("%s: %s", c.Name, status)
		if c.Detail != "" {
			fmt.Printf(" (%s)", c.Detail)
		}
		fmt.Println()
	}
	printAICallDiagnosticsSection()
	fmt.Println()
	if allPass {
		fmt.Println("Ready for real AI run: YES")
		return 0
	}
	fmt.Println("Ready for real AI run: NO")
	return 1
}

func checkGlobalContextOutputLimit() readinessCheck {
	requested := aipipeline.GlobalContextRequestedMaxOutputTokens()
	effective := aipipeline.GlobalContextEffectiveMaxOutputTokens()
	if requested < 16384 || effective < 16384 {
		return readinessCheck{
			Name:   "Global Context output limit wiring",
			Pass:   false,
			Detail: fmt.Sprintf("requested=%d effective=%d", requested, effective),
		}
	}
	return readinessCheck{Name: "Global Context output limit wiring", Pass: true, Detail: fmt.Sprintf("requested=%d effective=%d", requested, effective)}
}

func checkReadinessConfigFromEnv() readinessCheck {
	requiredPositive := []string{
		"OPENAI_TIMEOUT_SECONDS",
		"OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS",
		"OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS",
		"OPENAI_ACTIONS_TIMEOUT_SECONDS",
		"OPENAI_ACTIONS_MAX_OUTPUT_TOKENS",
		"ACTIONS_OBJECT_DETAIL_BATCH_SIZE",
		"ACTIONS_OBJECT_DETAIL_MAX_CONCURRENCY",
		"OPENAI_ACTION_OBJECT_DETAIL_TIMEOUT_SECONDS",
		"OPENAI_ACTION_OBJECT_DETAIL_MAX_OUTPUT_TOKENS",
		"OPENAI_MAX_RETRIES",
	}
	missing := make([]string, 0)
	for _, key := range requiredPositive {
		if key == "OPENAI_MAX_RETRIES" {
			if _, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err != nil {
				missing = append(missing, key)
			}
			continue
		}
		if positiveEnvInt(key) <= 0 {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return readinessCheck{Name: "Configuration", Pass: false, Detail: "missing or invalid: " + strings.Join(missing, ", ")}
	}
	if strings.TrimSpace(os.Getenv("DASHBOARD_VALIDATION_MODE")) == "" {
		return readinessCheck{Name: "Configuration", Pass: false, Detail: "DASHBOARD_VALIDATION_MODE unset"}
	}
	if actions.ActionsComposeRequestedMaxOutputTokens() <= 0 {
		return readinessCheck{Name: "Configuration", Pass: false, Detail: "compose max output tokens invalid"}
	}
	if actions.ActionsObjectDetailBatchSize() < 1 {
		return readinessCheck{Name: "Configuration", Pass: false, Detail: "batch size invalid"}
	}
	return readinessCheck{Name: "Configuration", Pass: true}
}

func positiveEnvInt(key string) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func checkMetricSemanticsRegistry() readinessCheck {
	def, ok := aidatapack.MetricSemanticDefinitionFor("idc_days")
	if !ok || def.CanonicalLabelRU != "Покрытие запасом" {
		return readinessCheck{Name: "Metric semantics", Pass: false, Detail: "idc_days registry missing"}
	}
	return readinessCheck{Name: "Metric semantics", Pass: true}
}

func checkChunkPromptVersion() readinessCheck {
	if dashboardhome.ProductionChunkAnalysisSchemaVersion() != dashboardhome.DashboardChunkAnalysisSchemaVersionV3 {
		return readinessCheck{Name: "Chunk local refs", Pass: false, Detail: "expected v3 schema"}
	}
	prompt := dashboardhome.ProductionChunkSystemPrompt()
	if !strings.Contains(prompt, "idc_days") || !strings.Contains(prompt, "срок хранения") {
		return readinessCheck{Name: "Chunk local refs", Pass: false, Detail: "prompt missing idc_days semantics"}
	}
	return readinessCheck{Name: "Chunk local refs", Pass: true}
}

func checkActionsTwoPhaseContract() readinessCheck {
	if !actions.IsTwoPhaseActionsPrompt(actions.ProductionActionsPromptVersion()) {
		return readinessCheck{Name: "Actions two-phase contract", Pass: false, Detail: "expected v2_4"}
	}
	return readinessCheck{Name: "Actions two-phase contract", Pass: true}
}

func checkActionsComposeOutputLimitWiring() readinessCheck {
	requested := actions.ActionsComposeRequestedMaxOutputTokens()
	effective := actions.ActionsComposeEffectiveMaxOutputTokens()
	wiring := actions.ActionsComposeOutputLimitWiring()
	detailLimit := actions.ActionsObjectDetailRequestedMaxOutputTokens()
	if requested <= 0 || effective <= 0 {
		return readinessCheck{
			Name:   "Actions Compose output limit wiring",
			Pass:   false,
			Detail: fmt.Sprintf("requested=%d effective=%d wiring=%s", requested, effective, wiring),
		}
	}
	if requested != effective {
		return readinessCheck{
			Name:   "Actions Compose output limit wiring",
			Pass:   false,
			Detail: fmt.Sprintf("requested=%d effective=%d wiring=%s", requested, effective, wiring),
		}
	}
	if detailLimit <= 0 {
		return readinessCheck{
			Name:   "Actions Compose output limit wiring",
			Pass:   false,
			Detail: "object detail max output tokens invalid",
		}
	}
	return readinessCheck{
		Name:   "Actions Compose output limit wiring",
		Pass:   true,
		Detail: fmt.Sprintf("compose requested=%d effective=%d wiring=%s; object_detail=%d", requested, effective, wiring, detailLimit),
	}
}

func checkPlaceholderValidator() readinessCheck {
	issues := aipipeline.DetectUnresolvedPlaceholders("Выручка снизилась на %.")
	if len(issues) == 0 {
		return readinessCheck{Name: "Global Context placeholder validator", Pass: false, Detail: "detector inactive"}
	}
	valid := aipipeline.DetectUnresolvedPlaceholders("Выручка снизилась на 7,79%.")
	if len(valid) > 0 {
		return readinessCheck{Name: "Global Context placeholder validator", Pass: false, Detail: "false positive on filled value"}
	}
	return readinessCheck{Name: "Global Context placeholder validator", Pass: true}
}

func checkGlobalContextPrompt() readinessCheck {
	prompt := aipipeline.ProductionGlobalContextSystemPromptV2()
	if !strings.Contains(prompt, "снизил") && !strings.Contains(prompt, "на %") {
		return readinessCheck{Name: "Global Context prompt", Pass: false, Detail: "missing placeholder rule"}
	}
	return readinessCheck{Name: "Global Context prompt", Pass: true}
}

func checkFactRegistryIDs() readinessCheck {
	if len(aidatapack.RequiredAccountOrderFactIDs) < 4 {
		return readinessCheck{Name: "Fact Registry", Pass: false, Detail: "orders fact IDs not registered"}
	}
	return readinessCheck{Name: "Fact Registry", Pass: true, Detail: strings.Join(aidatapack.RequiredAccountOrderFactIDs, ", ")}
}

func checkInspectorLegacyAdapter() readinessCheck {
	adapter := aipipeline.LegacyActionsInspectorAdapter{
		ActionsOutput: []byte(`{"final_status":"failed","accepted_count":0,"dropped_count":1}`),
	}
	if !adapter.Available() {
		return readinessCheck{Name: "Inspector legacy adapter", Pass: false}
	}
	return readinessCheck{Name: "Inspector legacy adapter", Pass: true}
}

func checkDebugArtifactPersistence() readinessCheck {
	if !aipipeline.ReadinessVerifyDebugArtifactBuilders() {
		return readinessCheck{Name: "Debug artifact persistence", Pass: false, Detail: "artifact builders missing schema versions"}
	}
	return readinessCheck{Name: "Debug artifact persistence", Pass: true}
}

func checkFinalDashboardSemanticGuard() readinessCheck {
	if !aipipeline.ReadinessVerifyFinalSemanticValidators() {
		return readinessCheck{Name: "Final Dashboard semantic guard", Pass: false, Detail: "validator wiring failed"}
	}
	return readinessCheck{Name: "Final Dashboard semantic guard", Pass: true}
}

func checkFinalActionsSemanticGuard() readinessCheck {
	if !aipipeline.ReadinessVerifyFinalSemanticValidators() {
		return readinessCheck{Name: "Final Actions semantic guard", Pass: false, Detail: "validator wiring failed"}
	}
	return readinessCheck{Name: "Final Actions semantic guard", Pass: true}
}

func checkPublicationGuard() readinessCheck {
	if !aipipeline.ReadinessVerifyPublicationGuardWiring() {
		return readinessCheck{Name: "Publication guard", Pass: false, Detail: "guard did not block semantic defect"}
	}
	return readinessCheck{Name: "Publication guard", Pass: true}
}

func checkActionsTimeoutTopology() readinessCheck {
	if !aipipeline.ReadinessVerifyActionsTimeoutTopology() {
		return readinessCheck{Name: "Actions timeout topology", Pass: false, Detail: "compose/detail timeout invalid"}
	}
	return readinessCheck{Name: "Actions timeout topology", Pass: true}
}

func checkInspectorMonitoringSections() readinessCheck {
	if !aipipeline.ReadinessVerifyInspectorMonitoringSections() {
		return readinessCheck{Name: "Inspector monitoring sections", Pass: false}
	}
	return readinessCheck{Name: "Inspector monitoring sections", Pass: true}
}

func checkAICallDiagnosticsEnvelope() readinessCheck {
	if !aipipeline.ReadinessVerifyAICallDiagnosticBuilders() {
		return readinessCheck{Name: "AI call diagnostics envelope", Pass: false}
	}
	return readinessCheck{Name: "AI call diagnostics envelope", Pass: true}
}

func checkRawProviderResponseCapture() readinessCheck {
	if !aipipeline.ReadinessVerifyAICallDiagnosticBuilders() {
		return readinessCheck{Name: "Raw provider response capture", Pass: false}
	}
	return readinessCheck{Name: "Raw provider response capture", Pass: true}
}

func checkInspectorEntityFlattening() readinessCheck {
	if !aipipeline.ReadinessVerifyInspectorEntityFlattening() {
		return readinessCheck{Name: "Inspector entity flattening", Pass: false}
	}
	return readinessCheck{Name: "Inspector entity flattening", Pass: true}
}

func checkInspectorSearchWiring() readinessCheck {
	if !aipipeline.ReadinessVerifyInspectorSearchWiring() {
		return readinessCheck{Name: "Inspector search wiring", Pass: false}
	}
	return readinessCheck{Name: "Inspector search wiring", Pass: true}
}

func checkAICallDiagnosticsCoverage() readinessCheck {
	if !aipipeline.AllProductionAIPathsCovered() {
		return readinessCheck{Name: "AI call diagnostics coverage", Pass: false}
	}
	return readinessCheck{Name: "AI call diagnostics coverage", Pass: true}
}

func printAICallDiagnosticsSection() {
	checks := []struct {
		name string
		ok   bool
	}{
		{"Shared Chunk raw capture", aipipeline.ReadinessVerifyChunkInvalidJSONWiring()},
		{"Global Context raw capture", aipipeline.ReadinessVerifyGlobalContextInvalidJSONWiring()},
		{"Global Context invalid JSON path", aipipeline.ReadinessVerifyGlobalContextInvalidJSONWiring()},
		{"Global Context provider incomplete metadata", aipipeline.ReadinessVerifyGlobalContextTruncationWiring()},
		{"Dashboard raw capture", aipipeline.ReadinessVerifyDashboardInvalidJSONWiring()},
		{"Dashboard invalid JSON path", aipipeline.ReadinessVerifyDashboardInvalidJSONWiring()},
		{"Actions Compose raw capture", aipipeline.ReadinessVerifyAICallDiagnosticBuilders()},
		{"Object Detail raw capture", aipipeline.ReadinessVerifyAICallDiagnosticBuilders()},
		{"Provider incomplete metadata", aipipeline.ReadinessVerifyGlobalContextTruncationWiring()},
		{"Failed-run artifact persistence", aipipeline.ReadinessVerifyFailedRunArtifactPersistence()},
		{"Inspector AI call views", aipipeline.ReadinessVerifyInspectorAICallViews()},
	}
	fmt.Println("AI CALL DIAGNOSTICS")
	for _, c := range checks {
		status := "PASS"
		if !c.ok {
			status = "FAIL"
		}
		fmt.Printf("%s: %s\n", c.name, status)
	}
	fmt.Printf("All production AI call paths covered: %s\n", yesNo(aipipeline.AllProductionAIPathsCovered()))
}

func yesNo(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}
