package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
)

func TestProductionIsolation_CurrentSchemaRemainsV10(t *testing.T) {
	if contract.UnifiedAnalysisOutputSchemaVersionCurrent != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("Current=%s want v10", contract.UnifiedAnalysisOutputSchemaVersionCurrent)
	}
	if contract.SchemaName() != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("SchemaName=%s want v10", contract.SchemaName())
	}
	raw, err := contract.SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"aivion_unified_analysis_v10"`) {
		t.Fatal("runtime SchemaJSON must embed v10")
	}
	if strings.Contains(string(raw), `"aivion_unified_analysis_v11"`) {
		t.Fatal("runtime SchemaJSON must not be v11")
	}
}

func TestProductionIsolation_CurrentPromptRemainsV1_31(t *testing.T) {
	if prompt.CurrentAnalystVersion != prompt.VersionAnalystV1_31 {
		t.Fatalf("CurrentAnalystVersion=%s want v1_31", prompt.CurrentAnalystVersion)
	}
	tpl, err := prompt.TemplateByVersion(prompt.CurrentAnalystVersion)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tpl.Instructions, "aivion_unified_analysis_v10") {
		t.Fatal("current prompt must target v10")
	}
	if strings.Contains(tpl.Instructions, "aivion_unified_analysis_v11") {
		t.Fatal("current prompt must not target v11")
	}
}

func TestProductionIsolation_V11NotInPackBuilder(t *testing.T) {
	policy := pack.BuildAnalysisPolicy()
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "analysis_policy_v2") {
		t.Fatal("pack builder must not embed analysis_policy_v2")
	}
	if strings.Contains(s, "aivion_unified_analysis_v11") {
		t.Fatal("pack builder must not reference v11 output schema")
	}
}

func TestProductionIsolation_V11NotPublicationAccepted(t *testing.T) {
	pol := publication.DefaultPublicationPolicy()
	if pol.AcceptedSchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("AcceptedSchemaVersion=%s want v10", pol.AcceptedSchemaVersion)
	}
	if pol.AcceptedPromptVersion != prompt.VersionAnalystV1_31 {
		t.Fatalf("AcceptedPromptVersion=%s want v1_31", pol.AcceptedPromptVersion)
	}
	err := pol.Allows(
		prompt.VersionAnalystV1_32,
		"sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		contract.UnifiedAnalysisOutputSchemaVersionV11,
		contract.CanonicalSchemaHashV11,
		false,
	)
	if err == nil {
		t.Fatal("publication policy must reject v11/v1_32")
	}
}

func TestProductionIsolation_ProviderDefaultUsesCurrentSchema(t *testing.T) {
	if contract.SchemaName() == contract.UnifiedAnalysisOutputSchemaVersionV11 {
		t.Fatal("provider default SchemaName must not be v11")
	}
	candidates := []string{
		filepath.Join("..", "start.go"),
		filepath.Join("..", "..", "start.go"),
	}
	var raw []byte
	var err error
	for _, p := range candidates {
		raw, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("read start.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "contract.SchemaJSON()") {
		t.Fatal("start.go must call contract.SchemaJSON() (current schema)")
	}
	if strings.Contains(src, "SchemaJSONV11") {
		t.Fatal("start.go must not call SchemaJSONV11")
	}
}
