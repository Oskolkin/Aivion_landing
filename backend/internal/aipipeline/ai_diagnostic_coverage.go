package aipipeline

// AIDiagnosticCoverageRegistration describes diagnostic wiring for one production AI call path.
type AIDiagnosticCoverageRegistration struct {
	Stage                string
	Component            string
	RawCaptureEnabled    bool
	ParsedCaptureEnabled bool
	ErrorCaptureEnabled  bool
	InspectorSupported   bool
}

// ProductionAIDiagnosticCoverage is the explicit registry of production AI call paths.
func ProductionAIDiagnosticCoverage() []AIDiagnosticCoverageRegistration {
	return []AIDiagnosticCoverageRegistration{
		{Stage: "shared_chunk_analysis", Component: "chunk_analysis", RawCaptureEnabled: true, ParsedCaptureEnabled: true, ErrorCaptureEnabled: true, InspectorSupported: true},
		{Stage: "global_context", Component: AIUsageComponentGlobalContext, RawCaptureEnabled: true, ParsedCaptureEnabled: true, ErrorCaptureEnabled: true, InspectorSupported: true},
		{Stage: "dashboard", Component: AIUsageComponentDashboardSynthesis, RawCaptureEnabled: true, ParsedCaptureEnabled: true, ErrorCaptureEnabled: true, InspectorSupported: true},
		{Stage: "actions_compose", Component: AIUsageComponentActionsSynthesis, RawCaptureEnabled: true, ParsedCaptureEnabled: true, ErrorCaptureEnabled: true, InspectorSupported: true},
		{Stage: "action_object_detail", Component: "action_object_detail", RawCaptureEnabled: true, ParsedCaptureEnabled: true, ErrorCaptureEnabled: true, InspectorSupported: true},
		{Stage: "actions_reviewer", Component: "legacy_global_check", RawCaptureEnabled: false, ParsedCaptureEnabled: false, ErrorCaptureEnabled: false, InspectorSupported: false},
	}
}

// AllProductionAIPathsCovered reports whether every registered path has runtime wiring verified.
func AllProductionAIPathsCovered() bool {
	return ReadinessVerifyGlobalContextInvalidJSONWiring() &&
		ReadinessVerifyGlobalContextTruncationWiring() &&
		ReadinessVerifyDashboardInvalidJSONWiring() &&
		ReadinessVerifyChunkInvalidJSONWiring() &&
		ReadinessVerifyAICallDiagnosticBuilders() &&
		ReadinessVerifyFailedRunArtifactPersistence() &&
		ReadinessVerifyInspectorAICallViews()
}
