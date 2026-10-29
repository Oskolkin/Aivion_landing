package aisummaries

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MaybeDumpFunnelAIDevFiles writes OpenAI funnel_ai_analysis and validated block when
// AI_DEV_FUNNEL_DUMP_DIR is set (local regeneration proof only).
func MaybeDumpFunnelAIDevFiles(openAIContent string, valOut *ValidationOutcome) {
	dir := os.Getenv("AI_DEV_FUNNEL_DUMP_DIR")
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(openAIContent), &parsed); err == nil {
		if raw, ok := parsed["funnel_ai_analysis"]; ok {
			writeDevJSON(filepath.Join(dir, "openai_raw_funnel_ai_analysis.json"), raw)
		}
	}
	if valOut != nil && valOut.Payload != nil && valOut.Payload.FunnelAIAnalysis != nil {
		writeDevJSON(filepath.Join(dir, "validated_funnel_ai_analysis.json"), valOut.Payload.FunnelAIAnalysis)
	}
}

func writeDevJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}
