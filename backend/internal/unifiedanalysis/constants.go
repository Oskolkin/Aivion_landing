package unifiedanalysis

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

const (
	ModeUnifiedAnalysis  = "unified_analysis"
	PipelineVersionV1    = "unified_analysis_v1"
	PromptVersionSmoke   = provider.PromptVersionSmokeV1
	PromptVersionAnalyst = "unified_analyst_v1_31"

	TriggerManual   = "manual"
	TriggerPostSync = "post_sync"

	DataPackHashPending = "unified:pending"

	ProviderNameOpenAI = "openai"

	PackFilename = "analysis_pack.zip"
)

const (
	AnalysisPackSchemaVersion = contract.AnalysisPackSchemaVersionV1
)
