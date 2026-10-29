package dashboardhome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const dashboardNarrativeDebugPromptVersion = DashboardNarrativePromptVersion

// NarrativeDebugRecorder persists intermediate AI outputs for devtools inspection.
type NarrativeDebugRecorder struct {
	Dir        string
	chunkIndex int
	runSummary map[string]any
	enabled    bool
}

func NewNarrativeDebugRecorder(dir string) *NarrativeDebugRecorder {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return &NarrativeDebugRecorder{}
	}
	return &NarrativeDebugRecorder{
		Dir:        dir,
		enabled:    true,
		runSummary: map[string]any{"prompt_version": dashboardNarrativeDebugPromptVersion},
	}
}

func (r *NarrativeDebugRecorder) Enabled() bool {
	return r != nil && r.enabled
}

func (r *NarrativeDebugRecorder) ensureDirs() error {
	if !r.Enabled() {
		return nil
	}
	for _, sub := range []string{"", "chunks", "synthesis"} {
		path := filepath.Join(r.Dir, sub)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

type chunkDebugRecord struct {
	ChunkID                   string   `json:"chunk_id"`
	ChunkType                 string   `json:"chunk_type"`
	ChunkHash                 string   `json:"chunk_hash"`
	PromptVersion             string   `json:"prompt_version"`
	InputApproxTokens         int      `json:"input_approx_tokens"`
	RawAIResponse             string   `json:"raw_ai_response,omitempty"`
	JSONExtractResult         string   `json:"json_extract_result,omitempty"`
	RepairAttemptResult       string   `json:"repair_attempt_result,omitempty"`
	ParseError                string   `json:"parse_error,omitempty"`
	ValidatedAnalysis         any      `json:"validated_analysis,omitempty"`
	ValidationErrors          []string `json:"validation_errors,omitempty"`
	ValidationWarnings        []string `json:"validation_warnings,omitempty"`
	CandidateValidationErrors []string `json:"candidate_validation_errors,omitempty"`
	RejectedSignals           int      `json:"rejected_signals"`
	RejectedCandidates        int      `json:"rejected_candidates"`
	AcceptedSignals           int      `json:"accepted_signals"`
	AcceptedCandidates        int      `json:"accepted_candidates"`
	ParsedCandidateCount      int      `json:"parsed_candidate_count"`
	InventedFactRejected      int      `json:"invented_fact_rejected,omitempty"`
}

func (r *NarrativeDebugRecorder) RecordChunk(
	chunk aidatapack.DataPackChunk,
	rawResponse string,
	parseErr error,
	repairedJSON string,
	analysis *DashboardChunkAnalysis,
	validation ChunkValidationResult,
) {
	if !r.Enabled() {
		return
	}
	_ = r.ensureDirs()
	r.chunkIndex++
	prefix := fmt.Sprintf("%03d_%s", r.chunkIndex, chunk.ChunkID)
	record := chunkDebugRecord{
		ChunkID:              chunk.ChunkID,
		ChunkType:            chunk.ChunkType,
		ChunkHash:            chunk.ChunkHash,
		PromptVersion:        dashboardNarrativeDebugPromptVersion,
		InputApproxTokens:    chunk.ApproxTokens,
		RawAIResponse:        rawResponse,
		RepairAttemptResult:  repairedJSON,
		ValidationErrors:     append([]string(nil), validation.HardErrors...),
		ValidationWarnings:   append([]string(nil), validation.RepairableErrors...),
		InventedFactRejected: validation.InventedFactRejected,
		RejectedCandidates:   validation.RejectedCandidateCount,
		AcceptedCandidates:   len(validation.ActionCandidates),
	}
	if parseErr != nil {
		record.ParseError = parseErr.Error()
	}
	if analysis != nil {
		record.JSONExtractResult = marshalDebugJSON(analysis)
		record.ParsedCandidateCount = len(analysis.ActionCandidates)
		record.ValidatedAnalysis = map[string]any{
			"accepted_signals":         validation.Signals,
			"accepted_candidates":      validation.ActionCandidates,
			"original_signal_count":    len(analysis.Signals),
			"original_candidate_count": len(analysis.ActionCandidates),
		}
		record.AcceptedSignals = len(validation.Signals)
		record.RejectedSignals = validation.RejectedSignalCount
		record.CandidateValidationErrors = filterCandidateValidationErrors(validation.RepairableErrors)
	}
	_ = os.WriteFile(filepath.Join(r.Dir, "chunks", prefix+".raw_response.txt"), []byte(rawResponse), 0o644)
	_ = writeDebugJSON(filepath.Join(r.Dir, "chunks", prefix+".validation.json"), record)
	if chunk.Payload != nil {
		_ = writeDebugJSON(filepath.Join(r.Dir, "chunks", prefix+".input.json"), chunkAnalysisContextForAI(chunk))
	}
	if analysis != nil {
		_ = writeDebugJSON(filepath.Join(r.Dir, "chunks", prefix+".parsed.json"), analysis)
	}
}

type synthesisDebugRecord struct {
	SynthesisInputSummary      any      `json:"synthesis_input_summary,omitempty"`
	SynthesisInputApproxTokens int      `json:"synthesis_input_approx_tokens,omitempty"`
	RawSynthesisResponse       string   `json:"raw_synthesis_response,omitempty"`
	JSONExtractResult          string   `json:"json_extract_result,omitempty"`
	RepairAttemptResult        string   `json:"repair_attempt_result,omitempty"`
	ParseError                 string   `json:"parse_error,omitempty"`
	RepairError                string   `json:"repair_error,omitempty"`
	ValidationErrors           []string `json:"validation_errors,omitempty"`
	ValidationWarnings         []string `json:"validation_warnings,omitempty"`
	AcceptedFinalPayload       any      `json:"accepted_final_payload,omitempty"`
}

func (r *NarrativeDebugRecorder) RecordSynthesis(
	input any,
	inputApproxTokens int,
	rawResponse string,
	parseResult synthesisParseResult,
	validation []string,
	warnings []string,
	payload *dashboardNarrativePayload,
) {
	if !r.Enabled() {
		return
	}
	_ = r.ensureDirs()
	record := synthesisDebugRecord{
		SynthesisInputSummary:      input,
		SynthesisInputApproxTokens: inputApproxTokens,
		RawSynthesisResponse:       rawResponse,
		RepairAttemptResult:        parseResult.RepairedJSON,
		ValidationErrors:           validation,
		ValidationWarnings:         warnings,
		AcceptedFinalPayload:       payload,
	}
	if parseResult.ParseError != nil {
		record.ParseError = parseResult.ParseError.Error()
	}
	if parseResult.Payload != nil {
		record.JSONExtractResult = marshalDebugJSON(parseResult.Payload)
	}
	_ = os.WriteFile(filepath.Join(r.Dir, "synthesis", "synthesis_raw_response.txt"), []byte(rawResponse), 0o644)
	_ = writeDebugJSON(filepath.Join(r.Dir, "synthesis", "synthesis_input.json"), input)
	_ = writeDebugJSON(filepath.Join(r.Dir, "synthesis", "synthesis_validation.json"), record)
	if parseResult.Payload != nil {
		_ = writeDebugJSON(filepath.Join(r.Dir, "synthesis", "synthesis_parsed.json"), parseResult.Payload)
	}
}

func (r *NarrativeDebugRecorder) WriteRunSummary(result *NarrativeGenerationResult, pipeline *ChunkPipelineOutcome) error {
	if !r.Enabled() {
		return nil
	}
	_ = r.ensureDirs()
	summary := map[string]any{
		"recorded_at":    time.Now().UTC().Format(time.RFC3339),
		"prompt_version": dashboardNarrativeDebugPromptVersion,
	}
	if result != nil {
		summary["generation_result"] = result
	}
	if pipeline != nil {
		summary["pipeline"] = pipeline
	}
	for k, v := range r.runSummary {
		summary[k] = v
	}
	return writeDebugJSON(filepath.Join(r.Dir, "run.json"), summary)
}

func writeDebugJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func marshalDebugJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func filterCandidateValidationErrors(errors []string) []string {
	out := make([]string, 0)
	for _, err := range errors {
		if strings.Contains(err, "candidate_") {
			out = append(out, err)
		}
	}
	return out
}

func approxJSONTokens(v any) int {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return (len(raw) + 3) / 4
}
