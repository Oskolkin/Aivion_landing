package aipipeline

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

const GlobalContextArtifactSchemaVersion = "global_context_artifact_v1"

// GlobalContextArtifact is the compact required final Global Context state.
type GlobalContextArtifact struct {
	SchemaVersion              string                          `json:"schema_version"`
	InputSchemaVersion         string                          `json:"input_schema_version"`
	OutputSchemaVersion        string                          `json:"output_schema_version"`
	SellerAccountID            int64                           `json:"seller_account_id"`
	MetricDate                 string                          `json:"metric_date"`
	DataPackHash               string                          `json:"data_pack_hash"`
	GlobalContextInputHash     string                          `json:"global_context_input_hash"`
	ChunkSetSchemaVersion      string                          `json:"chunk_set_schema_version,omitempty"`
	ChunkHashes                []string                        `json:"chunk_hashes,omitempty"`
	GlobalContext              aicontext.GlobalSemanticContext `json:"global_context"`
	GlobalContextHash          string                          `json:"global_context_hash"`
	SourceSharedAnalysisStatus string                          `json:"source_shared_analysis_status,omitempty"`
	DataQualityLimitations     []string                        `json:"data_quality_limitations,omitempty"`
}

func buildGlobalContextArtifact(
	input GlobalContextGenerationInput,
	ctx *aicontext.GlobalSemanticContext,
	inputHash, contextHash string,
) (GlobalContextArtifact, error) {
	if ctx == nil {
		return GlobalContextArtifact{}, fmt.Errorf("global context is nil")
	}
	limitations := append([]string(nil), input.DataQualityLimitations...)
	if len(limitations) == 0 {
		limitations = []string{}
	}
	chunkHashes := append([]string(nil), input.ChunkHashes...)
	if len(chunkHashes) == 0 {
		chunkHashes = []string{}
	}
	return GlobalContextArtifact{
		SchemaVersion:              GlobalContextArtifactSchemaVersion,
		InputSchemaVersion:         input.SchemaVersion,
		OutputSchemaVersion:        ctx.SchemaVersion,
		SellerAccountID:            input.SellerAccountID,
		MetricDate:                 input.MetricDate,
		DataPackHash:               input.DataPackHash,
		GlobalContextInputHash:     inputHash,
		ChunkSetSchemaVersion:      input.ChunkSetSchemaVersion,
		ChunkHashes:                chunkHashes,
		GlobalContext:              *ctx,
		GlobalContextHash:          contextHash,
		SourceSharedAnalysisStatus: input.SharedAnalysisStatus,
		DataQualityLimitations:     limitations,
	}, nil
}

func decodeGlobalContextArtifactPayload(payload any) (GlobalContextArtifact, error) {
	if payload == nil {
		return GlobalContextArtifact{}, fmt.Errorf("artifact payload is nil")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return GlobalContextArtifact{}, err
	}
	var artifact GlobalContextArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return GlobalContextArtifact{}, err
	}
	return artifact, nil
}

func validateStoredGlobalContextArtifact(
	artifact GlobalContextArtifact,
	input GlobalContextGenerationInput,
	inputHash string,
	storedHash string,
	computedHash string,
) (string, error) {
	if artifact.SchemaVersion != GlobalContextArtifactSchemaVersion {
		return "schema_version_mismatch", nil
	}
	if artifact.InputSchemaVersion != input.SchemaVersion {
		return "input_schema_version_mismatch", nil
	}
	if artifact.SellerAccountID != input.SellerAccountID {
		return "seller_account_id_mismatch", nil
	}
	if artifact.MetricDate != input.MetricDate {
		return "metric_date_mismatch", nil
	}
	if artifact.DataPackHash != input.DataPackHash {
		return "data_pack_hash_mismatch", nil
	}
	if artifact.GlobalContextInputHash != inputHash {
		return "global_context_input_hash_mismatch", nil
	}
	if input.SchemaVersion == GlobalContextInputSchemaVersionV2 {
		if artifact.ChunkSetSchemaVersion != input.ChunkSetSchemaVersion {
			return "chunk_set_schema_version_mismatch", nil
		}
		if !slices.Equal(artifact.ChunkHashes, input.ChunkHashes) {
			return "chunk_hashes_mismatch", nil
		}
	}
	if artifact.OutputSchemaVersion != aicontext.GlobalSemanticContextSchemaVersion {
		return "output_schema_version_mismatch", nil
	}
	if artifact.GlobalContextHash == "" {
		return "global_context_hash_missing", nil
	}
	if storedHash != "" && computedHash != "" && storedHash != computedHash {
		return "artifact_hash_mismatch", nil
	}
	validation := ValidateGlobalSemanticContext(input, artifact.GlobalContext)
	if validation.Status != aicontext.ValidationStatusValid &&
		validation.Status != aicontext.ValidationStatusValidWithWarnings {
		return "restored_context_invalid", nil
	}
	return "", nil
}

func validateStoredGlobalContextArtifactResumeHint(
	artifact GlobalContextArtifact,
	input GlobalContextGenerationInput,
	storedHash string,
	computedHash string,
) (string, error) {
	if artifact.SchemaVersion != GlobalContextArtifactSchemaVersion {
		return "schema_version_mismatch", nil
	}
	if artifact.InputSchemaVersion != input.SchemaVersion {
		return "input_schema_version_mismatch", nil
	}
	if artifact.SellerAccountID != input.SellerAccountID {
		return "seller_account_id_mismatch", nil
	}
	if artifact.MetricDate != input.MetricDate {
		return "metric_date_mismatch", nil
	}
	if artifact.DataPackHash != input.DataPackHash {
		return "data_pack_hash_mismatch", nil
	}
	if artifact.OutputSchemaVersion != aicontext.GlobalSemanticContextSchemaVersion {
		return "output_schema_version_mismatch", nil
	}
	if artifact.GlobalContextHash == "" {
		return "global_context_hash_missing", nil
	}
	if storedHash != "" && computedHash != "" && storedHash != computedHash {
		return "artifact_hash_mismatch", nil
	}
	if strings.TrimSpace(artifact.GlobalContext.SchemaVersion) == "" {
		return "restored_context_invalid", nil
	}
	if strings.TrimSpace(artifact.GlobalContext.OverallStory) == "" {
		return "restored_context_invalid", nil
	}
	return "", nil
}
