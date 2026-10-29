package landscape

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrompt96NoV3InProductionPath(t *testing.T) {
	files := []string{"core.go", "detail.go", "factual_source.go", "detail_source.go"}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)
	forbidden := []string{"DefaultCoreProfilesV3()", "DefaultDetailProfilesV3()", "LandscapeSchemaV3"}
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		src := string(body)
		for _, tok := range forbidden {
			if strings.Contains(src, tok) {
				t.Errorf("%s contains production V3 token %s", name, tok)
			}
		}
	}
}

func TestPrompt96NoMixedAdMethodsInV4Source(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)
	for _, name := range []string{"factual_source.go", "detail_source.go", "factual_source_ads_v4.go"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		src := string(body)
		if strings.Contains(src, "BatchGetCampaignPeriodPerformance") || strings.Contains(src, "BatchGetSKUAdvertising(") {
			t.Errorf("%s still calls mixed V3 advertising methods", name)
		}
	}
}

func TestPrompt96NoCategoryDetailNPlusOne(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "detail_source.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Contains(src, "GetCategoryFunnelPeriod") || strings.Contains(src, "GetCategorySearchSummary") {
		t.Fatal("detail_source.go must not issue per-category GetCategory Factual calls")
	}
}
