package contracts

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestPrompt98CurrentPoliciesAgreeWithContractBundle(t *testing.T) {
	pol := factualapi.CurrentPolicies()
	if pol.DQ != factualapi.DQPolicyV4 || pol.Capability != factualapi.CapabilityPolicyV4 || pol.API != factualapi.APIVersionV6 {
		t.Fatalf("CurrentPolicies not on Prompt 9.8 versions: %+v", pol)
	}
	if pol.SalesMaturity != factualapi.SalesMaturityV2 || pol.Temporal != factualapi.TemporalPolicyV1 {
		t.Fatalf("unchanged policies drifted: %+v", pol)
	}
	cur, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	v := cur.Versions()
	if v.DQPolicy != pol.DQ || v.CapabilityPolicy != pol.Capability || v.FactualAPI != pol.API {
		t.Fatalf("contract bundle disagrees with CurrentPolicies: bundle=%#v policies=%#v", v, pol)
	}
	if v.OrchestrationPolicy != policy.VersionV7 {
		t.Fatalf("orchestration=%s want %s", v.OrchestrationPolicy, policy.VersionV7)
	}
	if v.SalesMaturityPolicy != factualapi.SalesMaturityV2 {
		t.Fatalf("sales maturity bumped without algorithm change: %s", v.SalesMaturityPolicy)
	}
	wantCore := landscape.LandscapeSchemaV5 + "|" + landscape.CoreProfileVersionSealV5()
	wantDetail := landscape.LandscapeSchemaV5 + "|" + landscape.DetailProfileVersionSealV5()
	if v.LandscapeCore != wantCore {
		t.Fatalf("landscape core seal=%s want %s", v.LandscapeCore, wantCore)
	}
	if v.LandscapeDetail != wantDetail {
		t.Fatalf("landscape detail seal=%s want %s", v.LandscapeDetail, wantDetail)
	}
	if v.DiscoveryPack == NotImplemented || v.DiscoveryPrompt == NotImplemented || v.DiscoveryOutput == NotImplemented || v.DiscoveryValidation == NotImplemented {
		t.Fatal("Discovery contracts must be implemented after Prompt 10")
	}
	if v.EvidenceExpansion == NotImplemented {
		t.Fatal("EvidenceExpansion must be implemented after Prompt 11")
	}
	if v.DeepPack == NotImplemented || v.DeepPrompt == NotImplemented || v.DeepOutput == NotImplemented || v.DeepValidation == NotImplemented {
		t.Fatal("Deep contracts must be implemented after Prompt 12")
	}
	if v.ActionReconciliation == NotImplemented || v.Publication == NotImplemented || v.Publish == NotImplemented {
		t.Fatal("Action/Publication contracts must be implemented after Prompt 13")
	}
	if cur.Hash() == "" {
		t.Fatal("bundle hash empty")
	}
}
