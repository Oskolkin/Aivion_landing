package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCoverageProbeSerializationPreservesContractFields(t *testing.T) {
	in := coverageProbe{
		Domain: "Finance", Endpoint: "/v1/finance/accrual/types", HTTPMethod: "POST", HTTPStatus: 403,
		Availability: "SCOPE_BLOCKED", Empty: false, RequiresRealEntity: "", PremiumDependency: "NONE",
		DocumentedStatus: "DOCUMENTED", SanitizedErrorCode: "FORBIDDEN", SanitizedErrorMessage: "access denied",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out coverageProbe
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Domain != in.Domain || out.Endpoint != in.Endpoint || out.HTTPMethod != in.HTTPMethod || out.HTTPStatus != in.HTTPStatus || out.Availability != in.Availability || out.Empty != in.Empty || out.RequiresRealEntity != in.RequiresRealEntity || out.PremiumDependency != in.PremiumDependency || out.DocumentedStatus != in.DocumentedStatus || out.SanitizedErrorCode != in.SanitizedErrorCode || out.SanitizedErrorMessage != in.SanitizedErrorMessage {
		t.Fatalf("coverage probe changed after round trip: %#v", out)
	}
}

func TestClassifyCoverageRateLimitIsNotAuthFailure(t *testing.T) {
	availability, _, _ := classifyCoverageResponse(429, []byte(`{"code":"TooManyRequests","message":"slow down"}`))
	if availability != "RATE_LIMITED" {
		t.Fatalf("availability=%q", availability)
	}
}

func TestBusinessRecordCountingDoesNotTreatWrappersAsRecords(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"result":{"items":[]},"pagination":{"total":0}}`),
		[]byte(`{"data":{},"metadata":{"cursor":""}}`),
	} {
		if got := countBusinessRecords(body); got != 0 {
			t.Fatalf("wrapper-only response has %d records", got)
		}
	}
	if got := countBusinessRecords([]byte(`{"result":{"items":[{"sku":1}]}}`)); got != 1 {
		t.Fatalf("records=%d, want 1", got)
	}
}

func TestInventoryKeepsMutationsOutOfSafeWhitelist(t *testing.T) {
	var mutationSeen bool
	for _, method := range coverageInventory() {
		if method.Classification == "MUTATING_EXCLUDED" {
			mutationSeen = true
			if method.SafeToProbe {
				t.Fatalf("mutating method %s is probeable", method.Path)
			}
		}
	}
	if !mutationSeen {
		t.Fatal("mutating denylist is empty")
	}
}

func Test403ClassificationIsEvidenceBased(t *testing.T) {
	cases := []struct{ body, want string }{
		{`{"message":"Premium subscription required"}`, "PREMIUM_REQUIRED"},
		{`{"message":"required oauth scope is missing"}`, "SCOPE_BLOCKED"},
		{`{"message":"access denied"}`, "SCOPE_OR_PERMISSION_BLOCKED"},
	}
	for _, tc := range cases {
		got, _, _ := classifyCoverageResponse(403, []byte(tc.body), true)
		if got != tc.want {
			t.Fatalf("%s: got %s", tc.body, got)
		}
	}
}

func TestEntityDiscoveryArtifactDoesNotExposeBusinessIDs(t *testing.T) {
	artifact := entityDiscoveryArtifact(discoveredEntities{SKU: "123456", ProductID: "42", Counts: map[string]int{"SKU/Product": 1}})
	b, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "123456") || strings.Contains(string(b), `"42"`) {
		t.Fatalf("raw identifiers leaked: %s", b)
	}
}

func TestProviderErrorCaptureKeepsNumericCodeAndRawBody(t *testing.T) {
	body := []byte(`{"code":400,"message":"field required","details":[{"field":"limit"}]}`)
	code, message := providerError(400, body)
	if code != "400" || message != "field required" {
		t.Fatalf("code=%q message=%q", code, message)
	}
	value := sanitizeDiagnosticBody(body)
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "field required") {
		t.Fatalf("missing raw body: %s", b)
	}
}

func TestDiagnosticSanitizerRedactsTokens(t *testing.T) {
	b, _ := json.Marshal(sanitizeDiagnosticBody([]byte(`{"access_token":"secret","message":"safe"}`)))
	if strings.Contains(string(b), "secret") || !strings.Contains(string(b), "[REDACTED]") {
		t.Fatalf("unsafe artifact: %s", b)
	}
}

func TestRepairV21BUsesExactSafeRequestBodies(t *testing.T) {
	probes := repairSpecsV21B()
	if len(probes) != 6 {
		t.Fatalf("probes=%d", len(probes))
	}
	for _, probe := range probes {
		var body map[string]any
		if err := json.Unmarshal([]byte(probe.body), &body); err != nil {
			t.Fatalf("%s: %v", probe.path, err)
		}
		if probe.path == "/v1/analytics/data" && body["metrics"] == nil {
			t.Fatal("analytics metrics missing")
		}
		if probe.path == "/v1/finance/accrual/by-day" && body["last_id"] != "" {
			t.Fatal("first finance page must use empty last_id")
		}
	}
}
