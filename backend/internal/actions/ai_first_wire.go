package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
)

const (
	ActionsAIFirstSchemaVersionV1 = "actions_ai_first_v1"
	ActionsAIFirstSchemaVersionV2 = "actions_ai_first_v2"

	// ActionsAIFirstSchemaVersion is the current default schema for new shared-flow inputs.
	ActionsAIFirstSchemaVersion = ActionsAIFirstSchemaVersionV2

	// actionsAIFirstMaxOutputTokens avoids truncating large V2 candidate responses.
	actionsAIFirstMaxOutputTokens = 16384
)

type actionsAIFirstInputV1Wire struct {
	SchemaVersion         string                      `json:"schema_version"`
	SellerAccountID       int64                       `json:"seller_account_id"`
	MetricDate            string                      `json:"metric_date"`
	DataPackSchemaVersion string                      `json:"data_pack_schema_version,omitempty"`
	DataPackHash          string                      `json:"data_pack_hash,omitempty"`
	DataPack              aidatapack.SellerAIDataPack `json:"data_pack"`
	GlobalContext         *GlobalSemanticContext      `json:"global_context,omitempty"`
	AllowedActionTypes    []ActionType                `json:"allowed_action_types"`
	AllowedCategories     []ActionCategory            `json:"allowed_categories"`
	AllowedPriorities     []ActionPriority            `json:"allowed_priorities"`
	AllowedCTAKeys        []string                    `json:"allowed_cta_keys,omitempty"`
	ForbiddenClaims       []string                    `json:"forbidden_claims,omitempty"`
	MaxActions            int                         `json:"max_actions,omitempty"`
}

type actionsAIFirstInputV2Wire struct {
	SchemaVersion     string                 `json:"schema_version"`
	SellerAccountID   int64                  `json:"seller_account_id"`
	MetricDate        string                 `json:"metric_date"`
	DataPackHash      string                 `json:"data_pack_hash"`
	GlobalContext     *GlobalSemanticContext `json:"global_context"`
	GlobalContextHash string                 `json:"global_context_hash"`

	ActionCandidates []AIFirstActionCandidate            `json:"action_candidates"`
	FactCatalog      map[string]aidatapack.CompactFact   `json:"fact_catalog"`
	EntityCatalog    map[string]aidatapack.CompactEntity `json:"entity_catalog"`
	DataLimitations  []string                            `json:"data_limitations"`
	FailedChunkIDs   []string                            `json:"failed_chunk_ids"`

	AllowedActionTypes []ActionType     `json:"allowed_action_types"`
	AllowedCategories  []ActionCategory `json:"allowed_categories"`
	AllowedPriorities  []ActionPriority `json:"allowed_priorities"`
	AllowedCTAKeys     []string         `json:"allowed_cta_keys,omitempty"`
	ForbiddenClaims    []string         `json:"forbidden_claims,omitempty"`
	MaxActions         int              `json:"max_actions,omitempty"`
}

// MarshalActionsAIFirstInputWire serializes the provider-facing Actions input.
func MarshalActionsAIFirstInputWire(input ActionsAIFirstInput) ([]byte, error) {
	switch input.SchemaVersion {
	case ActionsAIFirstSchemaVersionV2:
		if input.DataPack == nil {
			return nil, fmt.Errorf("%s: backend data pack is required for mapper/validator", ActionsErrorInputMissingDataPack)
		}
		wire := actionsAIFirstInputV2Wire{
			SchemaVersion:      ActionsAIFirstSchemaVersionV2,
			SellerAccountID:    input.SellerAccountID,
			MetricDate:         input.MetricDate,
			DataPackHash:       input.DataPackHash,
			GlobalContext:      input.GlobalContext,
			GlobalContextHash:  input.GlobalContextHash,
			ActionCandidates:   append([]AIFirstActionCandidate(nil), input.ActionCandidates...),
			FactCatalog:        input.FactCatalog,
			EntityCatalog:      input.EntityCatalog,
			DataLimitations:    append([]string(nil), input.DataLimitations...),
			FailedChunkIDs:     append([]string(nil), input.FailedChunkIDs...),
			AllowedActionTypes: append([]ActionType(nil), input.AllowedActionTypes...),
			AllowedCategories:  append([]ActionCategory(nil), input.AllowedCategories...),
			AllowedPriorities:  append([]ActionPriority(nil), input.AllowedPriorities...),
			AllowedCTAKeys:     append([]string(nil), input.AllowedCTAKeys...),
			ForbiddenClaims:    append([]string(nil), input.ForbiddenClaims...),
			MaxActions:         input.MaxActions,
		}
		return json.Marshal(wire)
	default:
		if input.DataPack == nil {
			return nil, fmt.Errorf("%s: backend data pack is required for legacy wire", ActionsErrorInputMissingDataPack)
		}
		wire := actionsAIFirstInputV1Wire{
			SchemaVersion:         ActionsAIFirstSchemaVersionV1,
			SellerAccountID:       input.SellerAccountID,
			MetricDate:            input.MetricDate,
			DataPackSchemaVersion: input.DataPackSchemaVersion,
			DataPackHash:          input.DataPackHash,
			DataPack:              *input.DataPack,
			GlobalContext:         input.GlobalContext,
			AllowedActionTypes:    append([]ActionType(nil), input.AllowedActionTypes...),
			AllowedCategories:     append([]ActionCategory(nil), input.AllowedCategories...),
			AllowedPriorities:     append([]ActionPriority(nil), input.AllowedPriorities...),
			AllowedCTAKeys:        append([]string(nil), input.AllowedCTAKeys...),
			ForbiddenClaims:       append([]string(nil), input.ForbiddenClaims...),
			MaxActions:            input.MaxActions,
		}
		return json.Marshal(wire)
	}
}

// HashActionsAIFirstInputWire returns a stable hash of the provider-facing payload.
func HashActionsAIFirstInputWire(input ActionsAIFirstInput) (string, error) {
	raw, err := MarshalActionsAIFirstInputWire(input)
	if err != nil {
		return "", err
	}
	var wire any
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", err
	}
	return aihash.HashJSON(wire)
}

func (in ActionsAIFirstInput) IsV2CandidateInput() bool {
	return in.SchemaVersion == ActionsAIFirstSchemaVersionV2
}

// ActionsAIFirstRequestedMaxOutputTokens returns the max_output_tokens budget sent to OpenAI
// for Actions compose calls. Env vars take precedence over the package default.
func ActionsAIFirstRequestedMaxOutputTokens() int {
	if v := actionsEnvInt("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", 0); v > 0 {
		return v
	}
	if v := actionsEnvInt("AI_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return actionsAIFirstMaxOutputTokens
}

func actionsEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
