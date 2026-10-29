package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type DiscoveryRequestFingerprintV2 struct {
	Model            string `json:"model"`
	InstructionsHash string `json:"instructions_hash"`
	PayloadHash      string `json:"payload_hash"`
	SchemaName       string `json:"schema_name"`
	SchemaHash       string `json:"schema_hash"`
	Strict           bool   `json:"strict"`
	MaxOutputTokens  int    `json:"max_output_tokens"`
	Store            bool   `json:"store"`
	Truncation       string `json:"truncation"`
	ToolsEnabled     bool   `json:"tools_enabled"`
	PreviousResponse bool   `json:"previous_response_enabled"`
}

func FingerprintFromRequest(req Request) DiscoveryRequestFingerprintV2 {
	maxOut := req.MaxOutputTokens
	if maxOut <= 0 {
		maxOut = DefaultMaxOutputTokens()
	}
	return DiscoveryRequestFingerprintV2{
		Model:            req.Model,
		InstructionsHash: canonicaljson.Hash(req.Instructions),
		PayloadHash:      payloadBytesHash(req.PayloadJSON),
		SchemaName:       firstNonEmpty(req.SchemaName, OutputSchemaNameV1),
		SchemaHash:       canonicaljson.Hash(req.Schema),
		Strict:           true,
		MaxOutputTokens:  maxOut,
		Store:            false,
		Truncation:       "disabled",
		ToolsEnabled:     false,
		PreviousResponse: false,
	}
}

func payloadBytesHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func HashFingerprint(fp DiscoveryRequestFingerprintV2) string {
	return canonicaljson.Hash(fp)
}

func BuildDiscoveryResponsesRequest(req Request) openaix.ResponsesRequest {
	maxOut := req.MaxOutputTokens
	if maxOut <= 0 {
		maxOut = DefaultMaxOutputTokens()
	}
	return openaix.ResponsesRequest{
		Model:           req.Model,
		Instructions:    req.Instructions,
		Input:           string(req.PayloadJSON),
		Tools:           []any{},
		MaxOutputTokens: maxOut,
		Truncation:      "disabled",
		Store:           false,
		Text: openaix.TextFormat{
			Format: openaix.JSONSchemaFormat{
				Type:   "json_schema",
				Name:   firstNonEmpty(req.SchemaName, OutputSchemaNameV1),
				Strict: true,
				Schema: req.Schema,
			},
		},
	}
}

func EstimateDiscoveryRequestTokens(req Request) (int, error) {
	wire := BuildDiscoveryResponsesRequest(req)
	serialized, err := json.Marshal(wire)
	if err != nil {
		return 0, err
	}
	return aitokens.EstimateBytesTokens(serialized), nil
}
