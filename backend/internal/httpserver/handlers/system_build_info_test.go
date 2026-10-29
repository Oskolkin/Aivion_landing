package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo"
)

func TestSystemBuildInfo(t *testing.T) {
	provider := buildinfo.NewProvider("marketplace-ai-mvp-api", "test", ":18081")
	handler := NewSystemHandler(provider)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/build-info", nil)
	rr := httptest.NewRecorder()
	handler.GetBuildInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	startedAt, _ := body["started_at"].(string)
	if strings.TrimSpace(startedAt) == "" {
		t.Fatalf("started_at is empty")
	}
	if _, err := time.Parse(time.RFC3339, startedAt); err != nil {
		t.Fatalf("started_at parse: %v", err)
	}

	uptime, ok := body["uptime_seconds"].(float64)
	if !ok {
		t.Fatalf("uptime_seconds missing or invalid type")
	}
	if uptime < 0 {
		t.Fatalf("uptime_seconds negative: %v", uptime)
	}

	buildObj, ok := body["build"].(map[string]any)
	if !ok {
		t.Fatalf("build object missing")
	}
	if strings.TrimSpace(asString(buildObj["version"])) == "" {
		t.Fatalf("build.version missing")
	}

	forbiddenMarkers := []string{
		"database_url", "redis_addr", "redis_password", "s3_secret", "openai_api_key", "jwt_secret", "encryption_key",
	}
	raw := strings.ToLower(rr.Body.String())
	for _, marker := range forbiddenMarkers {
		if strings.Contains(raw, marker) {
			t.Fatalf("response leaked forbidden marker: %s", marker)
		}
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
