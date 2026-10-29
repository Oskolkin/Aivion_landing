package unifiedanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
)

const maxArtifactRawResponseBytes = 2 << 20 // 2MB

// Artifact kind constants for unified analysis provider orchestration.
const (
	ArtifactUnifiedProviderRequestSummary   = "unified_provider_request_summary"
	ArtifactUnifiedProviderResponseRaw      = "unified_provider_response_raw"
	ArtifactUnifiedProviderResponseSummary  = "unified_provider_response_summary"
	ArtifactUnifiedProviderUsage            = "unified_provider_usage"
	ArtifactUnifiedProviderTerminalAttempt  = "unified_provider_terminal_attempt"
	ArtifactUnifiedProviderCleanup          = "unified_provider_cleanup"
	ArtifactUnifiedAnalysisPackReference    = "unified_analysis_pack_reference"
	ArtifactUnifiedAnalysisDateResolution   = "unified_analysis_date_resolution"
	ArtifactUnifiedDashboardFacts           = "unified_dashboard_facts_v1"
	ArtifactUnifiedPackGenerationValidation = "unified_pack_generation_validation"
	ArtifactUnifiedSourceReconciliation     = "unified_source_reconciliation_v1"

	ArtifactUnifiedAnalysisOutputRaw        = "unified_analysis_output_raw"
	ArtifactUnifiedAnalysisOutputValidation = "unified_analysis_output_validation"
	ArtifactUnifiedAnalysisOutputValidated  = "unified_analysis_output_validated"
	ArtifactUnifiedAnalysisOutputSummary    = "unified_analysis_output_summary"
)

// ArtifactStore persists unified analysis orchestration artifacts.
type ArtifactStore struct {
	repo *Repository
}

// NewArtifactStore creates an ArtifactStore.
func NewArtifactStore(repo *Repository) *ArtifactStore {
	return &ArtifactStore{repo: repo}
}

type providerRequestSummary struct {
	Attempt         int               `json:"attempt"`
	Model           string            `json:"model"`
	Background      bool              `json:"background"`
	ContainerID     string            `json:"container_id"`
	ContainerFileID string            `json:"container_file_id"`
	ToolChoice      string            `json:"tool_choice"`
	MaxOutputTokens int               `json:"max_output_tokens"`
	PromptProfile   string            `json:"prompt_profile"`
	PromptVersion   string            `json:"prompt_version"`
	PromptHash      string            `json:"prompt_hash"`
	SchemaVersion   string            `json:"schema_version,omitempty"`
	SchemaHash      string            `json:"schema_hash,omitempty"`
	SchemaName      string            `json:"schema_name,omitempty"`
	Strict          *bool             `json:"strict,omitempty"`
	PipelineMode    string            `json:"pipeline_mode"`
	PackID          string            `json:"pack_id,omitempty"`
	PackHash        string            `json:"pack_hash,omitempty"`
	RequestID       string            `json:"request_id,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

type providerResponseSummary struct {
	Attempt               int      `json:"attempt,omitempty"`
	ResponseID            string   `json:"response_id"`
	Status                string   `json:"status"`
	RequestID             string   `json:"request_id,omitempty"`
	IncompleteReason      string   `json:"incomplete_reason,omitempty"`
	OutputTextPreview     string   `json:"output_text_preview,omitempty"`
	AssistantMessageCount int      `json:"assistant_message_count,omitempty"`
	OutputTextBytes       int      `json:"output_text_bytes,omitempty"`
	OutputItemTypes       []string `json:"output_item_types,omitempty"`
	CodeInterpreterCalls  int      `json:"code_interpreter_calls"`
	RawStoredInDB         bool     `json:"raw_stored_in_db"`
	RawBytes              int      `json:"raw_bytes"`
	RawOmittedReason      string   `json:"raw_omitted_reason,omitempty"`
}

type providerUsageArtifact struct {
	Attempt      int    `json:"attempt,omitempty"`
	ResponseID   string `json:"response_id,omitempty"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
	LatencyMs    int64  `json:"latency_ms,omitempty"`
}

// providerTerminalAttemptArtifact is the durable diagnostic record for one
// terminal provider response (completed/incomplete/failed/cancelled).
type providerTerminalAttemptArtifact struct {
	Attempt               int      `json:"attempt"`
	ResponseID            string   `json:"response_id"`
	Status                string   `json:"status"`
	RequestID             string   `json:"request_id"`
	RetrieveRequestID     string   `json:"retrieve_request_id,omitempty"`
	Model                 string   `json:"model"`
	IncompleteReason      string   `json:"incomplete_reason"`
	ProviderErrorCode     string   `json:"provider_error_code"`
	ProviderErrorMessage  string   `json:"provider_error_message"`
	RefusalPresent        bool     `json:"refusal_present"`
	AssistantMessageCount int      `json:"assistant_message_count"`
	OutputTextBytes       int      `json:"output_text_bytes"`
	OutputItemTypes       []string `json:"output_item_types"`
	CodeInterpreterCalls  int      `json:"code_interpreter_calls"`
	InputTokens           int64    `json:"input_tokens"`
	OutputTokens          int64    `json:"output_tokens"`
	TotalTokens           int64    `json:"total_tokens"`
	RawBytes              int      `json:"raw_bytes"`
	LatencyMs             int64    `json:"latency_ms,omitempty"`
}

type packReferenceArtifact struct {
	PackID        string `json:"pack_id"`
	PackHash      string `json:"pack_hash"`
	StorageKey    string `json:"storage_key"`
	SchemaVersion string `json:"schema_version"`
}

type cleanupArtifact struct {
	ContainerID string `json:"container_id"`
	FileID      string `json:"file_id"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func (s *ArtifactStore) SaveProviderRequestSummary(ctx context.Context, runID int64, summary providerRequestSummary) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedProviderRequestSummary, summary, "")
}

func (s *ArtifactStore) SaveProviderResponse(ctx context.Context, runID int64, responseID, status, outputText string, rawJSON []byte, ciCalls int) error {
	summary := providerResponseSummary{
		ResponseID:           responseID,
		Status:               status,
		CodeInterpreterCalls: ciCalls,
		RawBytes:             len(rawJSON),
		OutputTextBytes:      len(outputText),
	}
	if len(outputText) > 500 {
		summary.OutputTextPreview = outputText[:500]
	} else {
		summary.OutputTextPreview = outputText
	}

	var rawPayload any
	if len(rawJSON) > 0 && len(rawJSON) <= maxArtifactRawResponseBytes {
		summary.RawStoredInDB = true
		if err := json.Unmarshal(rawJSON, &rawPayload); err != nil {
			rawPayload = map[string]string{"raw_text": string(rawJSON)}
		}
		if err := s.saveJSON(ctx, runID, ArtifactUnifiedProviderResponseRaw, rawPayload, hashBytes(rawJSON)); err != nil {
			return err
		}
	} else if len(rawJSON) > maxArtifactRawResponseBytes {
		summary.RawOmittedReason = fmt.Sprintf("raw response exceeds %d bytes", maxArtifactRawResponseBytes)
	}

	return s.saveJSON(ctx, runID, ArtifactUnifiedProviderResponseSummary, summary, hashBytes(rawJSON))
}

func (s *ArtifactStore) SaveProviderUsage(ctx context.Context, runID int64, usage providerUsageArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedProviderUsage, usage, "")
}

// SaveProviderTerminalAttempt persists raw + summary + usage + dedicated attempt
// diagnostic for any terminal provider status. Artifacts append; prior attempts
// remain listable via ListArtifacts / ListArtifactsByKind.
func (s *ArtifactStore) SaveProviderTerminalAttempt(ctx context.Context, runID int64, attempt providerTerminalAttemptArtifact, rawJSON []byte) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("artifact store is not configured")
	}
	attempt.RawBytes = len(rawJSON)

	summary := providerResponseSummary{
		Attempt:               attempt.Attempt,
		ResponseID:            attempt.ResponseID,
		Status:                attempt.Status,
		RequestID:             attempt.RequestID,
		IncompleteReason:      attempt.IncompleteReason,
		AssistantMessageCount: attempt.AssistantMessageCount,
		OutputTextBytes:       attempt.OutputTextBytes,
		OutputItemTypes:       attempt.OutputItemTypes,
		CodeInterpreterCalls:  attempt.CodeInterpreterCalls,
		RawBytes:              attempt.RawBytes,
	}

	var rawPayload any
	if len(rawJSON) > 0 && len(rawJSON) <= maxArtifactRawResponseBytes {
		summary.RawStoredInDB = true
		if err := json.Unmarshal(rawJSON, &rawPayload); err != nil {
			rawPayload = map[string]string{"raw_text": string(rawJSON)}
		}
		if err := s.saveJSON(ctx, runID, ArtifactUnifiedProviderResponseRaw, rawPayload, hashBytes(rawJSON)); err != nil {
			return err
		}
	} else if len(rawJSON) > maxArtifactRawResponseBytes {
		summary.RawOmittedReason = fmt.Sprintf("raw response exceeds %d bytes", maxArtifactRawResponseBytes)
	}

	if err := s.saveJSON(ctx, runID, ArtifactUnifiedProviderResponseSummary, summary, hashBytes(rawJSON)); err != nil {
		return err
	}
	if err := s.saveJSON(ctx, runID, ArtifactUnifiedProviderUsage, providerUsageArtifact{
		Attempt:      attempt.Attempt,
		ResponseID:   attempt.ResponseID,
		InputTokens:  attempt.InputTokens,
		OutputTokens: attempt.OutputTokens,
		TotalTokens:  attempt.TotalTokens,
		LatencyMs:    attempt.LatencyMs,
	}, ""); err != nil {
		return err
	}
	return s.saveJSON(ctx, runID, ArtifactUnifiedProviderTerminalAttempt, attempt, hashBytes(rawJSON))
}

// ListArtifactsByKind returns payloads for every artifact of a kind (oldest→newest).
func (s *ArtifactStore) ListArtifactsByKind(ctx context.Context, runID int64, kind string) [][]byte {
	if s == nil || s.repo == nil {
		return nil
	}
	arts, err := s.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return nil
	}
	out := make([][]byte, 0)
	for _, art := range arts {
		if art.Kind != kind {
			continue
		}
		out = append(out, append([]byte(nil), art.PayloadJson...))
	}
	return out
}

func (s *ArtifactStore) SavePackReference(ctx context.Context, runID int64, ref packReferenceArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisPackReference, ref, ref.PackHash)
}

func (s *ArtifactStore) SavePackGenerationValidation(ctx context.Context, runID int64, report pack.GenerationValidationReport) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedPackGenerationValidation, report, report.PackHash)
}

// SaveSourceReconciliation persists the pre-OpenAI source reconciliation
// report for a run. It is saved on both success and failure so the gate
// decision is always auditable from artifacts alone.
func (s *ArtifactStore) SaveSourceReconciliation(ctx context.Context, runID int64, report sourcerecon.Report) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedSourceReconciliation, report, "")
}

// LoadSourceReconciliation returns the latest reconciliation report for a run.
func (s *ArtifactStore) LoadSourceReconciliation(ctx context.Context, runID int64) (sourcerecon.Report, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedSourceReconciliation)
	if !ok || len(art.PayloadJson) == 0 {
		return sourcerecon.Report{}, false
	}
	var out sourcerecon.Report
	if err := json.Unmarshal(art.PayloadJson, &out); err != nil {
		return sourcerecon.Report{}, false
	}
	return out, true
}

func (s *ArtifactStore) SaveDashboardFacts(ctx context.Context, runID int64, facts dashboardfacts.DashboardFactsV1) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedDashboardFacts, facts, facts.FactsHash)
}

func (s *ArtifactStore) LoadDashboardFacts(ctx context.Context, runID int64) (dashboardfacts.DashboardFactsV1, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedDashboardFacts)
	if !ok || len(art.PayloadJson) == 0 {
		return dashboardfacts.DashboardFactsV1{}, false
	}
	var out dashboardfacts.DashboardFactsV1
	if err := json.Unmarshal(art.PayloadJson, &out); err != nil {
		return dashboardfacts.DashboardFactsV1{}, false
	}
	if hash := textValue(art.Hash); hash != "" {
		out.FactsHash = hash
	}
	return out, true
}

// ListArtifactsWithError returns immutable artifacts for a run in creation
// order. Deterministic reprojection uses this to carry forward validated AI
// output without executing or regenerating provider work.
func (s *ArtifactStore) ListArtifactsWithError(ctx context.Context, runID int64) ([]dbgen.AiPipelineArtifact, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("artifact store is not configured")
	}
	return s.repo.ListArtifacts(ctx, runID)
}

// DateResolutionArtifact stores metric-date resolution metadata on a unified run.
type DateResolutionArtifact struct {
	Resolution       analysisdata.MetricDateResolution `json:"resolution"`
	PublishAsCurrent bool                              `json:"publish_as_current"`
}

func (s *ArtifactStore) SaveDateResolution(ctx context.Context, runID int64, art DateResolutionArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisDateResolution, art, "")
}

func (s *ArtifactStore) LoadDateResolution(ctx context.Context, runID int64) (DateResolutionArtifact, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedAnalysisDateResolution)
	if !ok || len(art.PayloadJson) == 0 {
		return DateResolutionArtifact{}, false
	}
	var out DateResolutionArtifact
	if err := json.Unmarshal(art.PayloadJson, &out); err != nil {
		return DateResolutionArtifact{}, false
	}
	return out, true
}

func (s *ArtifactStore) SaveCleanup(ctx context.Context, runID int64, cleanup cleanupArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedProviderCleanup, cleanup, "")
}

type outputValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type outputValidationArtifact struct {
	Valid          bool                    `json:"valid"`
	Errors         []outputValidationIssue `json:"errors"`
	Warnings       []outputValidationIssue `json:"warnings"`
	SchemaVersion  string                  `json:"schema_version"`
	SchemaHash     string                  `json:"schema_hash,omitempty"`
	PromptVersion  string                  `json:"prompt_version,omitempty"`
	PromptHash     string                  `json:"prompt_hash,omitempty"`
	PackID         string                  `json:"pack_id,omitempty"`
	PackHash       string                  `json:"pack_hash,omitempty"`
	OutputHash     string                  `json:"output_hash,omitempty"`
	Outcome        string                  `json:"outcome,omitempty"`
	ActionCount    int                     `json:"action_count"`
	TopActionCount int                     `json:"top_action_count"`
	DurationMs     int64                   `json:"duration_ms,omitempty"`
	ErrorCode      string                  `json:"error_code,omitempty"`
}

type outputSummaryArtifact struct {
	Outcome        string `json:"outcome,omitempty"`
	ActionCount    int    `json:"action_count"`
	TopActionCount int    `json:"top_action_count"`
	OutputHash     string `json:"output_hash,omitempty"`
	Bytes          int    `json:"bytes,omitempty"`
	Valid          bool   `json:"valid"`
	ErrorCode      string `json:"error_code,omitempty"`
}

func (s *ArtifactStore) SaveOutputRaw(ctx context.Context, runID int64, raw []byte) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("artifact store is not configured")
	}
	hash := hashBytes(raw)
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		payload = map[string]string{"raw_text": string(raw)}
	}
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisOutputRaw, payload, hash)
}

func (s *ArtifactStore) SaveOutputValidation(ctx context.Context, runID int64, report outputValidationArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisOutputValidation, report, "")
}

func (s *ArtifactStore) SaveOutputValidated(ctx context.Context, runID int64, canonical []byte, outputHash string) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("artifact store is not configured")
	}
	var payload any
	if err := json.Unmarshal(canonical, &payload); err != nil {
		return fmt.Errorf("validated output is not json: %w", err)
	}
	if outputHash == "" {
		outputHash = hashBytes(canonical)
	}
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisOutputValidated, payload, outputHash)
}

func (s *ArtifactStore) SaveOutputSummary(ctx context.Context, runID int64, summary outputSummaryArtifact) error {
	return s.saveJSON(ctx, runID, ArtifactUnifiedAnalysisOutputSummary, summary, summary.OutputHash)
}

func (s *ArtifactStore) LoadProviderResponseRaw(ctx context.Context, runID int64) ([]byte, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedProviderResponseRaw)
	if !ok {
		return nil, false
	}
	return art.PayloadJson, true
}

func (s *ArtifactStore) LoadValidatedMeta(ctx context.Context, runID int64) (storedValidatedMeta, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedAnalysisOutputValidated)
	if !ok {
		return storedValidatedMeta{}, false
	}
	meta := storedValidatedMeta{OutputHash: textValue(art.Hash)}
	if summary, ok := s.LoadOutputSummary(ctx, runID); ok {
		meta.Outcome = summary.Outcome
		meta.ActionCount = summary.ActionCount
		meta.TopActionCount = summary.TopActionCount
		if summary.OutputHash != "" {
			meta.OutputHash = summary.OutputHash
		}
	}
	return meta, meta.OutputHash != ""
}

// LoadValidatedOutputJSON returns the raw JSON payload stored for
// ArtifactUnifiedAnalysisOutputValidated (the canonical UnifiedAnalysisOutputV1
// produced by Stage 4 validation) plus its content hash, as originally
// computed by ValidateOutput from outputvalidator.CanonicalJSON/OutputHash.
// Callers (Stage 5 publication) should trust the returned hash as the run's
// output_hash rather than re-hashing PayloadJson themselves, since the
// generic JSON roundtrip performed by saveJSON does not guarantee
// byte-identical re-serialization (e.g. numeric formatting).
func (s *ArtifactStore) LoadValidatedOutputJSON(ctx context.Context, runID int64) ([]byte, string, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedAnalysisOutputValidated)
	if !ok || len(art.PayloadJson) == 0 {
		return nil, "", false
	}
	return art.PayloadJson, textValue(art.Hash), true
}

// LoadOutputValidationIdentity returns the prompt/schema identity stamped on
// the Stage 4 output validation artifact for a unified analysis run (see
// outputValidationArtifact). Stage 5 PublicationPolicy uses this to verify a
// run was produced under the currently-accepted prompt/schema before
// publishing it.
func (s *ArtifactStore) LoadOutputValidationIdentity(ctx context.Context, runID int64) (promptVersion, promptHash, schemaVersion, schemaHash string, ok bool) {
	art, found := s.latestArtifact(ctx, runID, ArtifactUnifiedAnalysisOutputValidation)
	if !found || len(art.PayloadJson) == 0 {
		return "", "", "", "", false
	}
	var report outputValidationArtifact
	if err := json.Unmarshal(art.PayloadJson, &report); err != nil {
		return "", "", "", "", false
	}
	return report.PromptVersion, report.PromptHash, report.SchemaVersion, report.SchemaHash, true
}

func (s *ArtifactStore) LoadOutputSummary(ctx context.Context, runID int64) (outputSummaryArtifact, bool) {
	art, ok := s.latestArtifact(ctx, runID, ArtifactUnifiedAnalysisOutputSummary)
	if !ok {
		return outputSummaryArtifact{}, false
	}
	var summary outputSummaryArtifact
	if err := json.Unmarshal(art.PayloadJson, &summary); err != nil {
		return outputSummaryArtifact{}, false
	}
	return summary, true
}

func (s *ArtifactStore) latestArtifact(ctx context.Context, runID int64, kind string) (dbgen.AiPipelineArtifact, bool) {
	if s == nil || s.repo == nil {
		return dbgen.AiPipelineArtifact{}, false
	}
	arts, err := s.repo.ListArtifacts(ctx, runID)
	if err != nil {
		return dbgen.AiPipelineArtifact{}, false
	}
	var latest dbgen.AiPipelineArtifact
	found := false
	for _, art := range arts {
		if art.Kind != kind {
			continue
		}
		latest = art
		found = true
	}
	return latest, found
}

func (s *ArtifactStore) saveJSON(ctx context.Context, runID int64, kind string, payload any, hash string) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("artifact store is not configured")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", kind, err)
	}
	if hash == "" {
		hash = hashBytes(raw)
	}
	_, err = s.repo.CreateArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runID,
		Kind:          kind,
		Hash:          nullableText(hash),
		Storage:       "db",
		PayloadJson:   raw,
	})
	return err
}

func hashBytes(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
