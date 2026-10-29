package dashboardhome

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"

const DashboardChunkAnalysisSchemaVersionV1 = "dashboard_chunk_analysis_v1"

const DashboardChunkAnalysisSchemaVersionV2 = "dashboard_chunk_analysis_v2"

// DashboardChunkAnalysisSchemaVersion is the schema version emitted by the current
// production chunk analysis prompt.
const DashboardChunkAnalysisSchemaVersion = DashboardChunkAnalysisSchemaVersionV3

// IsSupportedDashboardChunkAnalysisSchemaVersion reports whether the schema
// version is accepted by chunk analysis parser and validator.
func IsSupportedDashboardChunkAnalysisSchemaVersion(version string) bool {
	switch version {
	case DashboardChunkAnalysisSchemaVersionV1, DashboardChunkAnalysisSchemaVersionV2, DashboardChunkAnalysisSchemaVersionV3:
		return true
	default:
		return false
	}
}

// Allowed chunk signal types.
const (
	ChunkSignalObservation          = "observation"
	ChunkSignalChange               = "change"
	ChunkSignalRiskCandidate        = "risk_candidate"
	ChunkSignalOpportunityCandidate = "opportunity_candidate"
	ChunkSignalDataQuality          = "data_quality"
	ChunkSignalRelationship         = "relationship"
)

var allowedChunkSignalTypes = map[string]struct{}{
	ChunkSignalObservation:          {},
	ChunkSignalChange:               {},
	ChunkSignalRiskCandidate:        {},
	ChunkSignalOpportunityCandidate: {},
	ChunkSignalDataQuality:          {},
	ChunkSignalRelationship:         {},
}

var allowedChunkAnalysisStatuses = map[string]struct{}{
	"completed":         {},
	"insufficient_data": {},
}

type DashboardChunkAnalysis struct {
	SchemaVersion           string                           `json:"schema_version"`
	ChunkID                 string                           `json:"chunk_id"`
	ChunkType               string                           `json:"chunk_type"`
	AnalysisStatus          string                           `json:"analysis_status"`
	Signals                 []DashboardChunkSignal           `json:"signals"`
	ActionCandidates        []actions.AIFirstActionCandidate `json:"action_candidates,omitempty"`
	DataQualityNotes        []ChunkDataQualityNote           `json:"data_quality_notes,omitempty"`
	LimitationsAcknowledged []string                         `json:"limitations_acknowledged,omitempty"`
}

type DashboardChunkSignal struct {
	SignalID       string   `json:"signal_id"`
	SignalType     string   `json:"signal_type"`
	EntityIDs      []string `json:"entity_ids"`
	FactIDs        []string `json:"fact_ids"`
	EntityRefs     []string `json:"entity_refs,omitempty"`
	FactRefs       []string `json:"fact_refs,omitempty"`
	Observation    string   `json:"observation"`
	WhyItMayMatter string   `json:"why_it_may_matter,omitempty"`
	RelatedMetrics []string `json:"related_metrics,omitempty"`
	Limitations    []string `json:"limitations,omitempty"`
}

type ChunkDataQualityNote struct {
	Note    string `json:"note"`
	ChunkID string `json:"chunk_id,omitempty"`
}

type ChunkValidationResult struct {
	Valid                  bool
	HardErrors             []string
	RepairableErrors       []string
	Repaired               bool
	Signals                []DashboardChunkSignal
	ActionCandidates       []actions.AIFirstActionCandidate
	InventedFactRejected   int
	RejectedSignalCount    int
	RejectedCandidateCount int
}

const (
	ChunkCallStatusCompleted        = "completed"
	ChunkCallStatusFailed           = "failed"
	ChunkCallStatusValidationFailed = "validation_failed"
	ChunkCallStatusSkipped          = "skipped"
)

// ChunkCallUsage captures provider usage for a single chunk AI call.
type ChunkCallUsage struct {
	ChunkID                  string `json:"chunk_id"`
	ChunkType                string `json:"chunk_type"`
	Provider                 string `json:"provider,omitempty"`
	Model                    string `json:"model,omitempty"`
	RequestID                string `json:"request_id,omitempty"`
	InputTokens              int    `json:"input_tokens"`
	OutputTokens             int    `json:"output_tokens"`
	LatencyMs                int64  `json:"latency_ms"`
	Status                   string `json:"status"`
	UsageSource              string `json:"usage_source,omitempty"`
	Error                    string `json:"error,omitempty"`
	ProviderStatus           string `json:"provider_status,omitempty"`
	FinishReason             string `json:"finish_reason,omitempty"`
	IncompleteReason         string `json:"incomplete_reason,omitempty"`
	OutputTruncated          bool   `json:"output_truncated,omitempty"`
	RequestedMaxOutputTokens int    `json:"requested_max_output_tokens,omitempty"`
	RawBody                  string `json:"raw_body,omitempty"`
	ParseStatus              string `json:"parse_status,omitempty"`
	ParseError               string `json:"parse_error,omitempty"`
}

type ChunkAnalysisRunResult struct {
	SellerAccountID           int64
	MetricDate                string
	ChunkCount                int
	Completed                 int
	Failed                    int
	ProviderFailedCount       int
	ValidationFailedCount     int
	InsufficientDataCount     int
	ValidatedSignalCount      int
	RejectedSignalCount       int
	ValidatedSignals          []DashboardChunkSignal
	ValidatedActionCandidates []actions.AIFirstActionCandidate
	ValidatedCandidateCount   int
	RejectedCandidateCount    int
	FailedChunkIDs            []string
	FailedChunkTypes          []string
	Analyses                  []DashboardChunkAnalysis
	ChunkHashes               []string
	ChunkCalls                []ChunkCallUsage
	InputTokens               int
	OutputTokens              int
	ParseErrors               int
	InventedFactIDsRejected   int
	ChunkFactRefsReturned     int `json:"chunk_fact_refs_returned,omitempty"`
	ChunkFactRefsValid        int `json:"chunk_fact_refs_valid,omitempty"`
	ChunkFactRefsUnknown      int `json:"chunk_fact_refs_unknown,omitempty"`
	ChunkEntityRefsReturned   int `json:"chunk_entity_refs_returned,omitempty"`
	ChunkEntityRefsValid      int `json:"chunk_entity_refs_valid,omitempty"`
	ChunkEntityRefsUnknown    int `json:"chunk_entity_refs_unknown,omitempty"`
	AnalysisStatus            string
}
