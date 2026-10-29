package publication

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
)

// Frozen fallback identifiers used only if the live prompt/schema hash
// computation below fails (should not happen in practice — kept as a safety
// net so policy construction never panics on a broken embed/build).
const (
	fallbackAcceptedPromptVersion = prompt.VersionAnalystV1_31
	// Fallback hash is only used if prompt.Render fails; kept non-empty so
	// policy construction never publishes with a blank accepted hash.
	fallbackAcceptedPromptHash    = "0000000000000000000000000000000000000000000000000000000000000000"
	fallbackAcceptedSchemaVersion = contract.UnifiedAnalysisOutputSchemaVersionCurrent
	fallbackAcceptedSchemaHash    = "sha256:ae192829"
)

// AdapterVersion identifies the Stage 5 dashboard/actions projection logic.
// Kept here (rather than only in adapter) so policy.go has no import on the
// adapter package (adapter may want to import publication-level constants
// in the future without creating a cycle).
const AdapterVersion = "unified_publication_adapter_v1"

// PublicationPolicy pins the exact prompt/schema/adapter identity that Stage 5
// is allowed to publish from a unified analysis run. Any run stamped with a
// different prompt_version/prompt_hash or schema_version/schema_hash is
// rejected unless the caller explicitly opts in via allowNonCurrent (used by
// admin/devtools republish tooling only).
type PublicationPolicy struct {
	AcceptedPromptVersion string
	AcceptedPromptHash    string
	AcceptedSchemaVersion string
	AcceptedSchemaHash    string
	AdapterVersion        string
}

// DefaultPublicationPolicy returns the current production policy: the v1_11
// analyst prompt, the current UnifiedAnalysisOutputV1 JSON schema, and the
// Stage 5 v1 dashboard/actions adapter. Prompt and schema hashes are computed
// from the same source of truth the Stage 4 validator stamps on artifacts
// (unifiedanalysis/prompt and unifiedanalysis/contract), falling back to
// frozen constants only if that computation unexpectedly fails.
func DefaultPublicationPolicy() PublicationPolicy {
	promptHash := fallbackAcceptedPromptHash
	if rendered, err := prompt.Render(prompt.PromptProfileAnalysis); err == nil && strings.TrimSpace(rendered.Hash) != "" {
		promptHash = rendered.Hash
	}
	schemaHash := fallbackAcceptedSchemaHash
	if hash, err := contract.SchemaHash(); err == nil && strings.TrimSpace(hash) != "" {
		schemaHash = hash
	}
	return PublicationPolicy{
		AcceptedPromptVersion: firstNonEmpty(renderedVersion(), fallbackAcceptedPromptVersion),
		AcceptedPromptHash:    promptHash,
		AcceptedSchemaVersion: fallbackAcceptedSchemaVersion,
		AcceptedSchemaHash:    schemaHash,
		AdapterVersion:        AdapterVersion,
	}
}

func renderedVersion() string {
	if rendered, err := prompt.Render(prompt.PromptProfileAnalysis); err == nil {
		return strings.TrimSpace(rendered.Version)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Allows checks whether a run stamped with the given promptVersion/promptHash
// and schemaVersion/schemaHash may be published under this policy.
//
// When allowNonCurrent is true, prompt/schema mismatches are permitted (used
// for admin-triggered republish of historical runs); the run must still carry
// non-empty identifiers.
func (p PublicationPolicy) Allows(promptVersion, promptHash, schemaVersion, schemaHash string, allowNonCurrent bool) error {
	promptVersion = strings.TrimSpace(promptVersion)
	promptHash = strings.TrimSpace(promptHash)
	schemaVersion = strings.TrimSpace(schemaVersion)
	schemaHash = strings.TrimSpace(schemaHash)

	if promptVersion == "" {
		return NewError(CodePromptNotAllowed, "run has no prompt_version")
	}
	if schemaVersion == "" {
		return NewError(CodeSchemaNotAllowed, "run has no schema_version")
	}

	if allowNonCurrent {
		return nil
	}

	if promptVersion != p.AcceptedPromptVersion {
		return Errorf(CodePromptNotAllowed, "prompt_version %q is not the accepted publication prompt %q", promptVersion, p.AcceptedPromptVersion)
	}
	if p.AcceptedPromptHash != "" && promptHash != "" && promptHash != p.AcceptedPromptHash {
		return Errorf(CodePromptNotAllowed, "prompt_hash %q does not match accepted publication prompt hash", promptHash)
	}
	if schemaVersion != p.AcceptedSchemaVersion {
		return Errorf(CodeSchemaNotAllowed, "schema_version %q is not the accepted publication schema %q", schemaVersion, p.AcceptedSchemaVersion)
	}
	if p.AcceptedSchemaHash != "" && schemaHash != "" && schemaHash != p.AcceptedSchemaHash {
		return Errorf(CodeSchemaNotAllowed, "schema_hash %q does not match accepted publication schema hash", schemaHash)
	}
	return nil
}
