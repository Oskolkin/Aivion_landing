package contract

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/lineendings"
)

//go:embed schema/unified_analysis_output_v1.schema.json schema/unified_analysis_output_v2.schema.json schema/unified_analysis_output_v3.schema.json schema/unified_analysis_output_v4.schema.json schema/unified_analysis_output_v5.schema.json schema/unified_analysis_output_v6.schema.json schema/unified_analysis_output_v7.schema.json schema/unified_analysis_output_v8.schema.json schema/unified_analysis_output_v9.schema.json schema/unified_analysis_output_v10.schema.json schema/unified_analysis_output_v11.schema.json
var unifiedAnalysisSchemaFS embed.FS

const (
	unifiedAnalysisSchemaFileV1  = "schema/unified_analysis_output_v1.schema.json"
	unifiedAnalysisSchemaFileV2  = "schema/unified_analysis_output_v2.schema.json"
	unifiedAnalysisSchemaFileV3  = "schema/unified_analysis_output_v3.schema.json"
	unifiedAnalysisSchemaFileV4  = "schema/unified_analysis_output_v4.schema.json"
	unifiedAnalysisSchemaFileV5  = "schema/unified_analysis_output_v5.schema.json"
	unifiedAnalysisSchemaFileV6  = "schema/unified_analysis_output_v6.schema.json"
	unifiedAnalysisSchemaFileV7  = "schema/unified_analysis_output_v7.schema.json"
	unifiedAnalysisSchemaFileV8  = "schema/unified_analysis_output_v8.schema.json"
	unifiedAnalysisSchemaFileV9  = "schema/unified_analysis_output_v9.schema.json"
	unifiedAnalysisSchemaFileV10 = "schema/unified_analysis_output_v10.schema.json"
	unifiedAnalysisSchemaFileV11 = "schema/unified_analysis_output_v11.schema.json"
)

// unifiedAnalysisSchemaName is the OpenAI Structured Outputs
// "json_schema.name" identifier for the current Unified Analysis output.
const unifiedAnalysisSchemaName = UnifiedAnalysisOutputSchemaVersionCurrent

// CanonicalSchemaHashV10 is the immutable LF-normalized hash of schema v10.
const CanonicalSchemaHashV10 = "sha256:8c2474226f0de69ec18709df7da9853fa5fb11e00522caf829e270bc3ecc673b"

// CanonicalSchemaHashV11 is the immutable LF-normalized hash of schema v11.
// Not the runtime-accepted schema — Current remains v10 until Part 2 rollout.
const CanonicalSchemaHashV11 = "sha256:ea8ce2ed54699a832eadc42547e71053ce5f2048119df6e388b87a828cbecfb2"

var (
	unifiedAnalysisSchemaOnce  sync.Once
	unifiedAnalysisSchemaBytes []byte
	unifiedAnalysisSchemaHash  string
	unifiedAnalysisSchemaErr   error
)

func loadUnifiedAnalysisSchema() {
	raw, err := unifiedAnalysisSchemaFS.ReadFile(unifiedAnalysisSchemaFileV10)
	if err != nil {
		unifiedAnalysisSchemaErr = fmt.Errorf("read embedded schema: %w", err)
		return
	}
	normalized := lineendings.NormalizeLineEndingsBytes(raw)
	var probe any
	if err := json.Unmarshal(normalized, &probe); err != nil {
		unifiedAnalysisSchemaErr = fmt.Errorf("embedded schema is not valid JSON: %w", err)
		return
	}
	sum := sha256.Sum256(normalized)
	unifiedAnalysisSchemaBytes = normalized
	unifiedAnalysisSchemaHash = "sha256:" + hex.EncodeToString(sum[:])
}

// SchemaJSON returns the OpenAI Structured Outputs compatible JSON Schema
// for the current Unified Analysis output as raw bytes (canonical LF).
func SchemaJSON() (json.RawMessage, error) {
	unifiedAnalysisSchemaOnce.Do(loadUnifiedAnalysisSchema)
	if unifiedAnalysisSchemaErr != nil {
		return nil, unifiedAnalysisSchemaErr
	}
	out := make(json.RawMessage, len(unifiedAnalysisSchemaBytes))
	copy(out, unifiedAnalysisSchemaBytes)
	return out, nil
}

// SchemaHash returns a stable "sha256:<hex>" digest of the embedded
// current Unified Analysis JSON Schema bytes after LF normalization.
func SchemaHash() (string, error) {
	unifiedAnalysisSchemaOnce.Do(loadUnifiedAnalysisSchema)
	if unifiedAnalysisSchemaErr != nil {
		return "", unifiedAnalysisSchemaErr
	}
	return unifiedAnalysisSchemaHash, nil
}

// SchemaName returns the OpenAI Structured Outputs "json_schema.name"
// identifier for the current Unified Analysis output schema.
func SchemaName() string {
	return unifiedAnalysisSchemaName
}

// StructuredOutputFormat returns the raw JSON Schema document for the
// current Unified Analysis output, ready for response_format.json_schema.
func StructuredOutputFormat() (json.RawMessage, error) {
	return SchemaJSON()
}

// SchemaJSONV1 returns the historical v1 schema bytes (read-only compatibility).
func SchemaJSONV1() (json.RawMessage, error) {
	return readNormalizedSchema(unifiedAnalysisSchemaFileV1)
}

// SchemaJSONByVersion returns schema bytes for a historical or current version.
func SchemaJSONByVersion(version string) (json.RawMessage, error) {
	var file string
	switch version {
	case UnifiedAnalysisOutputSchemaVersionV1:
		file = unifiedAnalysisSchemaFileV1
	case UnifiedAnalysisOutputSchemaVersionV2:
		file = unifiedAnalysisSchemaFileV2
	case UnifiedAnalysisOutputSchemaVersionV3:
		file = unifiedAnalysisSchemaFileV3
	case UnifiedAnalysisOutputSchemaVersionV4:
		file = unifiedAnalysisSchemaFileV4
	case UnifiedAnalysisOutputSchemaVersionV5:
		file = unifiedAnalysisSchemaFileV5
	case UnifiedAnalysisOutputSchemaVersionV6:
		file = unifiedAnalysisSchemaFileV6
	case UnifiedAnalysisOutputSchemaVersionV7:
		file = unifiedAnalysisSchemaFileV7
	case UnifiedAnalysisOutputSchemaVersionV8:
		file = unifiedAnalysisSchemaFileV8
	case UnifiedAnalysisOutputSchemaVersionV9:
		file = unifiedAnalysisSchemaFileV9
	case UnifiedAnalysisOutputSchemaVersionV10, "":
		file = unifiedAnalysisSchemaFileV10
	case UnifiedAnalysisOutputSchemaVersionV11:
		file = unifiedAnalysisSchemaFileV11
	default:
		return nil, fmt.Errorf("unknown schema version %q", version)
	}
	return readNormalizedSchema(file)
}

// SchemaJSONV11 returns the v11 schema bytes (read-only; not yet the
// runtime-accepted schema — see UnifiedAnalysisOutputSchemaVersionCurrent).
func SchemaJSONV11() (json.RawMessage, error) {
	return readNormalizedSchema(unifiedAnalysisSchemaFileV11)
}

// SchemaHashV11 returns a stable "sha256:<hex>" digest of the embedded v11
// JSON Schema bytes after LF normalization.
func SchemaHashV11() (string, error) {
	raw, err := unifiedAnalysisSchemaFS.ReadFile(unifiedAnalysisSchemaFileV11)
	if err != nil {
		return "", fmt.Errorf("read embedded schema: %w", err)
	}
	normalized := lineendings.NormalizeLineEndingsBytes(raw)
	sum := sha256.Sum256(normalized)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func readNormalizedSchema(file string) (json.RawMessage, error) {
	raw, err := unifiedAnalysisSchemaFS.ReadFile(file)
	if err != nil {
		return nil, err
	}
	normalized := lineendings.NormalizeLineEndingsBytes(raw)
	out := make(json.RawMessage, len(normalized))
	copy(out, normalized)
	return out, nil
}
