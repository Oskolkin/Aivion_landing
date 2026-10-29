package chatv2

const (
	ResultSchemaV1      = "chat_turn_result_v1"
	PromptVersionV1     = "chat_prompt_v1"
	ModelPolicyV1       = "chat_model_policy_v1"
	ToolPolicyV1        = "chat_tool_policy_v1"
	MemoryPolicyV1      = "chat_memory_policy_v1"
	ToolCatalogV1       = "chat_tool_catalog_v1"
	ExportSchemaV1      = "chat_export_schema_v1"
	ChatProjectionV1    = "chat_context_projection_v1"
	AnswerRetryPolicyV1 = "chat_answer_retry_policy_v1"
)

type ToolPolicy struct {
	Version              string
	MaxToolRounds        int
	MaxToolCalls         int
	MaxParallelCalls     int
	MaxEntities          int
	MaxRowsPerTool       int
	MaxTotalRows         int
	MaxPeriodDays        int
	MaxToolResultBytes   int
	MaxTotalContextBytes int
}

func DefaultToolPolicy() ToolPolicy {
	return ToolPolicy{
		Version:              ToolPolicyV1,
		MaxToolRounds:        6,
		MaxToolCalls:         12,
		MaxParallelCalls:     4,
		MaxEntities:          20,
		MaxRowsPerTool:       50,
		MaxTotalRows:         200,
		MaxPeriodDays:        90,
		MaxToolResultBytes:   32_000,
		MaxTotalContextBytes: 80_000,
	}
}

type MemoryPolicy struct {
	Version                   string
	MaxRecentTurns            int
	MaxHistoryBytes           int
	MaxReferencedEntities     int
	MaxAttachmentHistoryBytes int
	MaxAttachmentExcerptBytes int
}

func DefaultMemoryPolicy() MemoryPolicy {
	return MemoryPolicy{
		Version:                   MemoryPolicyV1,
		MaxRecentTurns:            12,
		MaxHistoryBytes:           24_000,
		MaxReferencedEntities:     32,
		MaxAttachmentHistoryBytes: 8_000,
		MaxAttachmentExcerptBytes: 4_000,
	}
}

type RetryPolicy struct {
	Version     string
	MaxAttempts int
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Version: AnswerRetryPolicyV1, MaxAttempts: 2}
}
