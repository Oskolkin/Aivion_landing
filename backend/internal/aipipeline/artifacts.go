package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type ArtifactRecorder struct {
	store   PipelineStore
	enabled bool
	runID   int64
	refs    map[string]string
}

func NewArtifactRecorder(store PipelineStore, runID int64, enabled bool) *ArtifactRecorder {
	return &ArtifactRecorder{
		store:   store,
		enabled: enabled,
		runID:   runID,
		refs:    make(map[string]string),
	}
}

func (r *ArtifactRecorder) Refs() map[string]string {
	if r == nil {
		return nil
	}
	out := make(map[string]string, len(r.refs))
	for k, v := range r.refs {
		out[k] = v
	}
	return out
}

// ArtifactDocument is the canonical on-disk shape for ai_pipeline_artifacts.payload_json.
type ArtifactDocument struct {
	Meta    map[string]any `json:"meta"`
	Payload any            `json:"payload"`
}

// HashArtifactPayload returns sha256:<hex> for a stable JSON representation of v.
func HashArtifactPayload(v any) (string, error) {
	return aihash.HashJSON(v)
}

func pipelineArtifactMeta(runID int64, extra map[string]any) map[string]any {
	meta := map[string]any{"pipeline_run_id": runID}
	for k, v := range extra {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		meta[k] = v
	}
	return meta
}

func aiArtifactMeta(runID int64, provider, model string, estimatedTokens, maxTokens int, extra map[string]any) map[string]any {
	meta := pipelineArtifactMeta(runID, extra)
	if provider != "" {
		meta["provider"] = provider
	}
	if model != "" {
		meta["model"] = model
	}
	if estimatedTokens > 0 {
		meta["estimated_input_tokens"] = estimatedTokens
	}
	if maxTokens > 0 {
		meta["max_input_tokens"] = maxTokens
	}
	proxy := openaix.ProxySummaryFromEnv()
	meta["ai_proxy"] = proxyArtifactMeta(proxy)
	return meta
}

func proxyArtifactMeta(proxy openaix.ProxySummary) map[string]any {
	out := map[string]any{
		"configured": proxy.AIProxyConfigured,
		"source":     proxy.EffectiveProxySource,
	}
	if proxy.EffectiveProxyScheme != "" {
		out["scheme"] = proxy.EffectiveProxyScheme
	}
	host := proxy.EffectiveProxyHost
	if host == "" {
		host = proxy.AIProxyHost
	}
	if host != "" {
		out["host"] = host
	}
	if proxy.ProxyError != "" {
		out["error"] = proxy.ProxyError
	}
	return out
}

func (r *ArtifactRecorder) SaveDocument(ctx context.Context, kind string, meta map[string]any, payload any) error {
	if r == nil || !r.enabled || r.store == nil {
		return nil
	}
	return r.saveDocument(ctx, kind, meta, payload)
}

func (r *ArtifactRecorder) SaveRequiredDocument(ctx context.Context, kind string, meta map[string]any, payload any) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("artifact recorder is not configured")
	}
	return r.saveDocument(ctx, kind, meta, payload)
}

func (r *ArtifactRecorder) saveDocument(ctx context.Context, kind string, meta map[string]any, payload any) error {
	doc := ArtifactDocument{
		Meta:    meta,
		Payload: scrubArtifactPayload(payload),
	}
	hash, err := HashArtifactPayload(doc)
	if err != nil {
		return fmt.Errorf("hash artifact %s: %w", kind, err)
	}
	id, err := r.store.CreateArtifact(ctx, CreateArtifactParams{
		PipelineRunID: r.runID,
		Kind:          kind,
		Hash:          hash,
		Payload:       doc,
	})
	if err != nil {
		return err
	}
	r.refs[kind] = fmt.Sprintf("artifact:%d", id)
	return nil
}

// Save persists an artifact using meta derived from pipeline_run_id only.
// Deprecated for new call sites: prefer SaveDocument with explicit meta.
func (r *ArtifactRecorder) Save(ctx context.Context, kind string, payload any, _ string) error {
	return r.SaveDocument(ctx, kind, pipelineArtifactMeta(r.runID, nil), payload)
}

func artifactDocumentPayload(raw []byte) (ArtifactDocument, error) {
	var doc ArtifactDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ArtifactDocument{}, err
	}
	return doc, nil
}

func scrubArtifactPayload(payload any) any {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return payload
	}
	return scrubValue(v)
}

func scrubValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			lk := stringsLower(k)
			if isSensitiveKey(lk) {
				out[k] = "[redacted]"
				continue
			}
			out[k] = scrubValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = scrubValue(val)
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	switch key {
	case "api_key", "token", "access_token", "refresh_token", "client_secret", "password", "authorization":
		return true
	default:
		return false
	}
}

func stringsLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
