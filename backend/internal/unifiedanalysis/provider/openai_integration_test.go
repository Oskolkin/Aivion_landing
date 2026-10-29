//go:build openai_integration

package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenAIIntegrationSmokeContainerZipResponse(t *testing.T) {
	if os.Getenv("RUN_OPENAI_INTEGRATION_TESTS") != "1" {
		t.Skip("set RUN_OPENAI_INTEGRATION_TESTS=1 to run OpenAI integration tests")
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is required for OpenAI integration tests")
	}
	model := strings.TrimSpace(os.Getenv("UNIFIED_ANALYSIS_MODEL"))
	if model == "" {
		t.Skip("UNIFIED_ANALYSIS_MODEL is required for OpenAI integration tests")
	}

	client, err := NewOpenAIClient(ClientConfig{
		APIKey:         apiKey,
		ProxyURL:       strings.TrimSpace(os.Getenv("AI_PROXY_URL")),
		TimeoutSeconds: 60,
		MaxRetries:     1,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	container, err := client.CreateContainer(ctx, CreateContainerRequest{
		Name:        "aivion-openai-integration-smoke",
		MemoryLimit: "1g",
	})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = CleanupContainer(cleanupCtx, client, container.ID, "")
	})

	zipBytes, err := integrationTinyAnalysisPackZip()
	if err != nil {
		t.Fatal(err)
	}
	archiveSHA := sha256.Sum256(zipBytes)
	file, err := client.UploadContainerFile(ctx, UploadContainerFileRequest{
		ContainerID: container.ID,
		Filename:    "analysis_pack.zip",
		ContentType: "application/zip",
		Content:     zipBytes,
		SHA256:      hex.EncodeToString(archiveSHA[:]),
	})
	if err != nil {
		t.Fatalf("UploadContainerFile: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = CleanupContainer(cleanupCtx, client, container.ID, file.ID)
	})

	resp, err := client.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{
		Model:           model,
		ContainerID:     container.ID,
		Instructions:    SmokeInstructions(),
		Input:           SmokeInput(),
		MaxOutputTokens: 512,
	})
	if err != nil {
		t.Fatalf("StartBackgroundResponse: %v", err)
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		current, pollErr := client.RetrieveResponse(ctx, resp.ID)
		if pollErr != nil {
			t.Fatalf("RetrieveResponse: %v", pollErr)
		}
		switch current.Status {
		case ProviderResponseStatusCompleted:
			if !HasCodeInterpreterCall(current) {
				t.Fatalf("completed response missing code interpreter call: %+v", current.CodeInterpreterCalls)
			}
			if !CallsMatchContainer(current, container.ID) {
				t.Fatalf("code interpreter container mismatch: want %s got %+v", container.ID, current.CodeInterpreterCalls)
			}
			return
		case ProviderResponseStatusFailed, ProviderResponseStatusIncomplete, ProviderResponseStatusCancelled:
			t.Fatalf("terminal provider status %s", current.Status)
		}
		if time.Now().After(deadline) {
			_, _ = client.CancelResponse(ctx, resp.ID)
			t.Fatal("timeout after 5m waiting for completed response")
		}
		time.Sleep(5 * time.Second)
	}
}

func integrationTinyAnalysisPackZip() ([]byte, error) {
	manifest := map[string]any{
		"schema_version":    "aivion_analysis_pack_v1",
		"pack_id":           "pack:0:2026-01-01:integration000001",
		"seller_account_id": 0,
		"metric_date":       "2026-01-01",
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
