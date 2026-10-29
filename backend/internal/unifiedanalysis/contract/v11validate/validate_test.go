package v11validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract/v11validate"
	policyv2 "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/policy/v2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
)

func TestV11SchemaStrictMode(t *testing.T) {
	if err := v11validate.ValidateSchemaStrict(); err != nil {
		t.Fatalf("v11 strict mode: %v", err)
	}
}

func TestV11SchemaHashPinned(t *testing.T) {
	h, err := contract.SchemaHashV11()
	if err != nil {
		t.Fatal(err)
	}
	if h != contract.CanonicalSchemaHashV11 {
		t.Fatalf("SchemaHashV11=%s want %s", h, contract.CanonicalSchemaHashV11)
	}
}

func TestProductionVersionsUnchanged(t *testing.T) {
	if contract.UnifiedAnalysisOutputSchemaVersionCurrent != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("Current schema=%s want v10", contract.UnifiedAnalysisOutputSchemaVersionCurrent)
	}
	if prompt.CurrentAnalystVersion != prompt.VersionAnalystV1_31 {
		t.Fatalf("CurrentAnalystVersion=%s want v1_31", prompt.CurrentAnalystVersion)
	}
	if contract.SchemaName() != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("SchemaName=%s want v10", contract.SchemaName())
	}
}

func TestV11Registered(t *testing.T) {
	raw, err := contract.SchemaJSONV11()
	if err != nil || len(raw) == 0 {
		t.Fatalf("SchemaJSONV11: %v len=%d", err, len(raw))
	}
	if _, err := prompt.TemplateByVersion(prompt.VersionAnalystV1_32); err != nil {
		t.Fatalf("v1_32 template: %v", err)
	}
	p, err := policyv2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.PolicyVersion != policyv2.Version {
		t.Fatalf("policy version=%s", p.PolicyVersion)
	}
}

func fixturesDir(t *testing.T, kind string) string {
	t.Helper()
	return filepath.Join("..", "testdata", "v11", kind)
}

type expectMeta struct {
	RejectLayer string `json:"reject_layer"`
	Code        string `json:"code"`
}

func loadExpect(t *testing.T, path string) expectMeta {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m expectMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestValidFixturesPass(t *testing.T) {
	policy, err := policyv2.Load()
	if err != nil {
		t.Fatal(err)
	}
	dir := fixturesDir(t, "valid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".facts.json") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			rep, out := v11validate.ValidateJSON(raw, policy)
			if out == nil {
				t.Fatalf("decode failed: %+v", rep.Issues)
			}
			if !rep.Valid {
				t.Fatalf("semantic invalid: %+v", rep.Issues)
			}
			if out.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV11 {
				t.Fatalf("schema_version=%s", out.SchemaVersion)
			}
		})
	}
	if count < 12 {
		t.Fatalf("expected >=12 valid fixtures, got %d", count)
	}
}

func TestInvalidStructuralFixturesGate(t *testing.T) {
	runInvalidGate(t, "text", "schema")
}

func TestInvalidReferenceFixturesGate(t *testing.T) {
	runInvalidGate(t, "semantic")
}

func TestFactRegistryGroundingFixturesGateDeferred(t *testing.T) {
	assertDeferredExpect(t, "unknown_fact_id", "deferred_part2_fact_registry")
}

func TestEntityGroundingFixturesGateDeferred(t *testing.T) {
	assertDeferredExpect(t, "invalid_sku", "deferred_part2_entity")
}

func assertDeferredExpect(t *testing.T, fixture, layer string) {
	t.Helper()
	expect := loadExpect(t, filepath.Join(fixturesDir(t, "invalid"), fixture+".expect.json"))
	if expect.RejectLayer != layer && expect.RejectLayer != "deferred_part2" {
		t.Fatalf("%s reject_layer=%s want %s", fixture, expect.RejectLayer, layer)
	}
	t.Logf("%s=DEFERRED_PART2 fixture=%s", layer, fixture)
}

func runInvalidGate(t *testing.T, layers ...string) {
	t.Helper()
	policy, err := policyv2.Load()
	if err != nil {
		t.Fatal(err)
	}
	layerSet := map[string]bool{}
	for _, l := range layers {
		layerSet[l] = true
	}
	dir := fixturesDir(t, "invalid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".expect.json") {
			continue
		}
		name := e.Name()
		expect := loadExpect(t, filepath.Join(dir, strings.TrimSuffix(name, ".json")+".expect.json"))
		if !layerSet[expect.RejectLayer] {
			continue
		}
		seen++
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			rep, _ := v11validate.ValidateJSON(raw, policy)
			if rep.Valid {
				t.Fatalf("expected rejection for %s (layer=%s)", name, expect.RejectLayer)
			}
			if len(rep.Issues) == 0 {
				t.Fatal("no issues")
			}
		})
	}
	if seen == 0 {
		t.Fatal("no fixtures matched gate layers")
	}
}

func TestPart1Gates(t *testing.T) {
	t.Run("v11_schema_gate", TestV11SchemaStrictMode)
	t.Run("schema_hash_gate", TestV11SchemaHashPinned)
	t.Run("production_version_unchanged_gate", TestProductionVersionsUnchanged)
	t.Run("analysis_policy_v2_gate", func(t *testing.T) {
		p, err := policyv2.Load()
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		h, err := policyv2.Hash()
		if err != nil {
			t.Fatal(err)
		}
		if h != policyv2.CanonicalPolicyHashV2 {
			t.Fatalf("policy hash mismatch: %s", h)
		}
	})
	t.Run("prompt_v1_32_gate", func(t *testing.T) {
		tpl, err := prompt.TemplateByVersion(prompt.VersionAnalystV1_32)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(tpl.Instructions, "aivion_unified_analysis_v11") {
			t.Fatal("prompt missing v11")
		}
		if strings.Contains(tpl.Instructions, "8-12") || strings.Contains(tpl.Instructions, "8–12") {
			t.Fatal("prompt must not prefer 8-12 Actions")
		}
	})
	t.Run("fixtures_gate", TestValidFixturesPass)
	t.Run("invalid_structural_fixtures_gate", TestInvalidStructuralFixturesGate)
	t.Run("invalid_reference_fixtures_gate", TestInvalidReferenceFixturesGate)
	t.Run("fact_registry_grounding_fixtures_gate", TestFactRegistryGroundingFixturesGateDeferred)
	t.Run("entity_grounding_fixtures_gate", TestEntityGroundingFixturesGateDeferred)
	t.Run("current_publication_unchanged_gate", func(t *testing.T) {
		if contract.UnifiedAnalysisOutputSchemaVersionCurrent != "aivion_unified_analysis_v10" {
			t.Fatal("publication-facing current schema changed")
		}
	})
}
