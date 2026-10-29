package aihash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const prefix = "sha256:"

// HashDataPack returns a stable canonical hash for a Seller AI Data Pack.
func HashDataPack(pack *aidatapack.SellerAIDataPack) string {
	if pack == nil {
		return ""
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:])
}

// HashGlobalSemanticContext returns a stable hash for Global Semantic Context.
func HashGlobalSemanticContext(ctx *aicontext.GlobalSemanticContext) string {
	if ctx == nil {
		return ""
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:])
}

// HashJSON returns sha256 hash for arbitrary JSON-marshalable value.
func HashJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal for hash: %w", err)
	}
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:]), nil
}
