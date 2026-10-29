package analysisdata

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestDetectEnrichmentSourceNil(t *testing.T) {
	if got := DetectEnrichmentSource(nil); got != "" {
		t.Fatalf("got=%q", got)
	}
}

func TestDetectEnrichmentSourceDevSeed(t *testing.T) {
	run := &dbgen.OzonEnrichmentSyncRun{Scope: "p0_dev_seed"}
	if got := DetectEnrichmentSource(run); got != "dev_seed" {
		t.Fatalf("source=%q", got)
	}
}

func TestDetectEnrichmentSourceFromRawSummary(t *testing.T) {
	run := &dbgen.OzonEnrichmentSyncRun{
		Scope:          "full",
		RawSummaryJson: []byte(`{"source":"ozon_api"}`),
	}
	if got := DetectEnrichmentSource(run); got != "ozon_api" {
		t.Fatalf("source=%q", got)
	}
}

func TestDetectEnrichmentSourceMalformedOrMissing(t *testing.T) {
	cases := []*dbgen.OzonEnrichmentSyncRun{
		{Scope: "full", RawSummaryJson: []byte(`{`)},
		{Scope: "full", RawSummaryJson: []byte(`{"other":"x"}`)},
		{Scope: "full", RawSummaryJson: []byte(`{"source":123}`)},
		{Scope: "full", RawSummaryJson: []byte(`{"source":""}`)},
		{Scope: "full"},
	}
	for i, run := range cases {
		if got := DetectEnrichmentSource(run); got != "" {
			t.Fatalf("case %d source=%q", i, got)
		}
	}
}
