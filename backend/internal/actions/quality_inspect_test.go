package actions

import "testing"

func TestInspectPublishedActionQuality_DetailedActionPasses(t *testing.T) {
	detail := buildAIFirstOperationalDetail(ValidDetailedPilotDraftFromCandidateForTests(testStockCandidate("c1")))
	raw := MarshalActionItemPayloadJSON(ActionItem{
		OperationalDetail:  detail,
		SourceCandidateIDs: []string{"c1"},
		EvidenceFactIDs:    []string{"fact_1"},
	})
	row := ActionQualityInspectInput{
		ID:                    1,
		SellerAccountID:       1,
		ActionKey:             "k1",
		Type:                  ActionTypeStockReplenishment,
		Category:              ActionCategoryStock,
		Title:                 "Test",
		Reason:                ValidDetailedPilotDraftFromCandidateForTests(testStockCandidate("c1")).Reason,
		SourceCandidateIDs:    []string{"c1"},
		EvidenceFactIDs:       []string{"fact_1"},
		AffectedEntitiesCount: 1,
		PayloadJSON:           raw,
	}
	report := InspectPublishedActionQuality([]ActionQualityInspectInput{row})
	if report.Passed != 1 || report.Failed != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestInspectPublishedActionQuality_LegacyGenericFails(t *testing.T) {
	row := ActionQualityInspectInput{
		ID:                    2,
		SellerAccountID:       1,
		Type:                  ActionTypeNoSalesWithStock,
		Category:              ActionCategoryStock,
		Title:                 "Generic",
		Reason:                "Наличие значительных запасов без продаж увеличивает складские издержки.",
		SourceCandidateIDs:    []string{"c1"},
		EvidenceFactIDs:       []string{"fact_1"},
		AffectedEntitiesCount: 1,
		PayloadJSON:           []byte("{}"),
	}
	report := InspectPublishedActionQuality([]ActionQualityInspectInput{row})
	if report.Passed != 0 || report.Failed != 1 {
		t.Fatalf("report=%+v", report)
	}
}
