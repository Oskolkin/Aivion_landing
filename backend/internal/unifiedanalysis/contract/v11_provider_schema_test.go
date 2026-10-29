package contract_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract/v11validate"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	policyv2 "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/policy/v2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// TestProviderSchemaCompatibilityV11 runs a live Responses API probe against the
// full v11 JSON Schema via the production OpenAI client adapter.
//
// Credentials: config.LoadSecretFromFileOrEnv("OPENAI_API_KEY_FILE", "OPENAI_API_KEY")
// — never logged. Model: UNIFIED_ANALYSIS_MODEL → OPENAI_MODEL (production: gpt-4.1).
//
// Gate is FAIL (not skip) when RUN_UA_V11_PROVIDER_SCHEMA_PROBE=1 and the probe
// does not fully succeed. Without that flag, Skip keeps the gate PENDING.
func TestProviderSchemaCompatibilityV11(t *testing.T) {
	_ = config.LoadEnvFiles()

	if os.Getenv("RUN_UA_V11_PROVIDER_SCHEMA_PROBE") != "1" {
		t.Skip("provider_schema_compatibility_gate PENDING: set RUN_UA_V11_PROVIDER_SCHEMA_PROBE=1 to run live probe")
	}

	startedAt := time.Now().UTC()
	artifact := map[string]any{
		"gate":       "provider_schema_compatibility_gate",
		"started_at": startedAt.Format(time.RFC3339),
		"status":     "RUNNING",
	}
	defer func() {
		artifact["finished_at"] = time.Now().UTC().Format(time.RFC3339)
		writeProviderArtifact(t, artifact)
	}()

	apiKey, err := config.LoadSecretFromFileOrEnv("OPENAI_API_KEY_FILE", "OPENAI_API_KEY")
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "secret_load_failed"
		t.Fatalf("load openai secret: %v", err)
	}
	if strings.TrimSpace(apiKey) == "" {
		artifact["status"] = "FAIL"
		artifact["error"] = "secret_missing"
		t.Fatal("OPENAI_API_KEY_FILE / OPENAI_API_KEY not configured")
	}
	artifact["secret_source"] = secretSourceLabel()

	model := firstNonEmptyEnv("UNIFIED_ANALYSIS_MODEL", "OPENAI_MODEL")
	if model == "" {
		artifact["status"] = "FAIL"
		artifact["error"] = "model_missing"
		t.Fatal("UNIFIED_ANALYSIS_MODEL / OPENAI_MODEL not configured")
	}
	artifact["model"] = model

	schema, err := contract.SchemaJSONV11()
	if err != nil {
		t.Fatal(err)
	}
	schemaHash, err := contract.SchemaHashV11()
	if err != nil {
		t.Fatal(err)
	}
	promptHash, err := prompt.HashForVersion(prompt.VersionAnalystV1_32)
	if err != nil {
		t.Fatal(err)
	}
	policyHash, err := policyv2.Hash()
	if err != nil {
		t.Fatal(err)
	}
	artifact["schema_version"] = contract.UnifiedAnalysisOutputSchemaVersionV11
	artifact["schema_hash"] = schemaHash
	artifact["prompt_version"] = prompt.VersionAnalystV1_32
	artifact["prompt_hash"] = "sha256:" + promptHash
	artifact["policy_version"] = policyv2.Version
	artifact["policy_hash"] = policyHash
	artifact["schema_bytes"] = len(schema)
	artifact["strict"] = true

	proxy := firstNonEmptyEnv("AI_HTTPS_PROXY", "AI_PROXY_URL")
	if proxy != "" {
		artifact["ai_proxy_configured"] = true
	} else {
		artifact["ai_proxy_configured"] = false
	}

	client, err := provider.NewOpenAIClient(provider.ClientConfig{
		APIKey:         apiKey,
		ProxyURL:       proxy,
		TimeoutSeconds: 120,
		MaxRetries:     1,
	})
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "client_init_failed"
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	container, err := client.CreateContainer(ctx, provider.CreateContainerRequest{
		Name:        "aivion-v11-schema-probe",
		MemoryLimit: "1g",
	})
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "create_container_failed"
		artifact["provider_error_class"] = classifyProviderErr(err)
		t.Fatalf("CreateContainer: %v", err)
	}
	artifact["container_created"] = true
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		_ = provider.CleanupContainer(cctx, client, container.ID, "")
	})

	// Prefer a real minimal zip first.
	zipBytes, err := buildMinimalZip()
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "zip_build_failed"
		t.Fatalf("buildMinimalZip: %v", err)
	}
	sum := sha256.Sum256(zipBytes)
	file, err := client.UploadContainerFile(ctx, provider.UploadContainerFileRequest{
		ContainerID: container.ID,
		Filename:    "analysis_pack.zip",
		ContentType: "application/zip",
		Content:     zipBytes,
		SHA256:      hex.EncodeToString(sum[:]),
	})
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "upload_pack_failed"
		artifact["provider_error_class"] = classifyProviderErr(err)
		t.Fatalf("UploadContainerFile: %v", err)
	}
	_ = file
	artifact["schema_accepted_at_request"] = false

	started, err := client.StartBackgroundResponse(ctx, provider.StartBackgroundResponseRequest{
		Model:       model,
		ContainerID: container.ID,
		Instructions: strings.TrimSpace(`
You are validating OpenAI Structured Outputs acceptance for schema aivion_unified_analysis_v11.
Do NOT use Code Interpreter or any tools.
Immediately return one Structured Output object with:
- schema_version = aivion_unified_analysis_v11
- analysis_outcome = analysis_failed
- analysis_meta.pack_id = pack_probe_v11
- analysis_meta.pack_hash = 0000000000000000000000000000000000000000000000000000000000000000
- analysis_meta.metric_date = 2026-07-08
- metric_claims = []
- threshold_claims = []
- findings = []
- actions = []
- limitations = [{"code":"source_incomplete","scope":"account","affected_blocks":["account_diagnosis"]}]
- every dashboard block status = unavailable, narrative fields null, arrays empty, limitation_codes=["source_incomplete"]
Keep all required keys. No markdown. No prose outside the schema.
`),
		Input: "Emit the minimal valid aivion_unified_analysis_v11 payload for analysis_failed now.",
		TextFormat: &provider.StructuredOutputFormat{
			Type:   "json_schema",
			Name:   contract.UnifiedAnalysisOutputSchemaVersionV11,
			Strict: true,
			Schema: schema,
		},
		ToolChoice:      "none",
		MaxOutputTokens: 8192,
		Metadata: map[string]string{
			"aivion_probe": "v11_schema_compatibility",
		},
	})
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["error"] = "start_response_failed"
		artifact["provider_error_class"] = classifyProviderErr(err)
		// Schema rejection typically surfaces here as 400.
		t.Fatalf("StartBackgroundResponse (schema accept): %v", err)
	}
	artifact["schema_accepted_at_request"] = true
	artifact["response_id_present"] = started.ID != ""
	artifact["initial_status"] = string(started.Status)

	var final *provider.ProviderResponse
	deadline := time.Now().Add(6 * time.Minute)
	for {
		current, pollErr := client.RetrieveResponse(ctx, started.ID)
		if pollErr != nil {
			artifact["status"] = "FAIL"
			artifact["error"] = "retrieve_failed"
			t.Fatalf("RetrieveResponse: %v", pollErr)
		}
		switch current.Status {
		case provider.ProviderResponseStatusCompleted:
			final = current
			goto done
		case provider.ProviderResponseStatusFailed, provider.ProviderResponseStatusIncomplete, provider.ProviderResponseStatusCancelled:
			artifact["status"] = "FAIL"
			artifact["response_status"] = string(current.Status)
			if current.Error != nil {
				artifact["provider_error_code"] = current.Error.Code
			}
			t.Fatalf("terminal provider status %s", current.Status)
		}
		if time.Now().After(deadline) {
			_, _ = client.CancelResponse(ctx, started.ID)
			artifact["status"] = "FAIL"
			artifact["error"] = "timeout"
			t.Fatal("timeout waiting for completed response")
		}
		time.Sleep(3 * time.Second)
	}
done:
	artifact["response_status"] = string(final.Status)
	artifact["response_model"] = final.Model
	artifact["usage_total_tokens"] = final.Usage.TotalTokens

	extracted, err := outputextract.ExtractStructuredOutput(final)
	if err != nil {
		artifact["status"] = "FAIL"
		artifact["parse_result"] = "FAIL"
		artifact["error"] = "extract_failed"
		t.Fatalf("ExtractStructuredOutput: %v", err)
	}
	artifact["parse_result"] = "PASS"
	artifact["output_bytes"] = len(extracted.JSONBytes)

	var out contract.UnifiedAnalysisOutputV11
	if err := json.Unmarshal(extracted.JSONBytes, &out); err != nil {
		artifact["status"] = "FAIL"
		artifact["decode_result"] = "FAIL"
		t.Fatalf("decode UnifiedAnalysisOutputV11: %v", err)
	}
	artifact["decode_result"] = "PASS"
	artifact["decoded_schema_version"] = out.SchemaVersion
	artifact["decoded_outcome"] = out.AnalysisOutcome

	if out.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV11 {
		artifact["status"] = "FAIL"
		artifact["validation_result"] = "FAIL"
		t.Fatalf("schema_version=%q", out.SchemaVersion)
	}

	if err := v11validate.ValidateInstanceAgainstSchemaV11(extracted.JSONBytes); err != nil {
		artifact["status"] = "FAIL"
		artifact["validation_result"] = "FAIL"
		artifact["validation_error"] = err.Error()
		t.Fatalf("structural schema validation: %v", err)
	}
	artifact["validation_result"] = "PASS"
	artifact["status"] = "PASS"
}

func secretSourceLabel() string {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY_FILE")) != "" {
		return "OPENAI_API_KEY_FILE"
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		return "OPENAI_API_KEY"
	}
	return "none"
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func classifyProviderErr(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "invalid_json_schema"), strings.Contains(msg, "json_schema"):
		return "schema_rejected"
	case strings.Contains(msg, "401"), strings.Contains(msg, "auth"):
		return "auth"
	case strings.Contains(msg, "429"):
		return "rate_limit"
	default:
		return "other"
	}
}

func writeProviderArtifact(t *testing.T, artifact map[string]any) {
	t.Helper()
	dir := filepath.Join("testdata", "v11")
	_ = os.MkdirAll(dir, 0o755)
	// Strip any accidental secret-like keys.
	for _, k := range []string{"api_key", "authorization", "OPENAI_API_KEY", "raw_response", "output_json"} {
		delete(artifact, k)
	}
	raw, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	path := filepath.Join(dir, "provider_schema_result.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}

func buildMinimalZip() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte(`{"schema_version":"aivion_analysis_pack_v1","pack_id":"pack_probe_v11","metric_date":"2026-07-08"}`)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
