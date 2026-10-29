package lifecycle

import (
	"os"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
)

func TestLifecycleSourceContractsAreDeterministicAndCurrent(t *testing.T) {
	again := collection.NewSourceContract("POSTINGS_FBS", "OZON:POSTINGS:FBS_LIST_V4", "1.0.0", "POST /v4/posting/fbs/list", "canonical-postings-fbs-list-v1", "seller+posting_number; explicit-delivery-schema; explicit-parent-lineage; cursor+has_next; no-finance")
	if again.SemanticMappingHash != FBSListContract.SemanticMappingHash {
		t.Fatal("non-deterministic hash")
	}
	for _, c := range []collection.SourceContract{FBOListContract, FBODetailContract, FBSListContract, FBSDetailContract, ReturnsListContract} {
		if len(c.SemanticMappingHash) != 64 || c.ContractKey == "" {
			t.Fatalf("contract=%+v", c)
		}
	}
}
func TestCanonicalLifecyclePackageDoesNotUseDeprecatedFBSV3List(t *testing.T) {
	files := []string{"runner.go", "contracts.go", "repository.go", "types.go"}
	for _, name := range files {
		body, e := os.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(string(body), "/v3/posting/fbs/list") {
			t.Fatalf("deprecated endpoint in %s", name)
		}
	}
}
