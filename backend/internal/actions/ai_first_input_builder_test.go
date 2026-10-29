package actions

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

type fakeAIDataPackBuilder struct {
	calls int
	pack  *aidatapack.SellerAIDataPack
	err   error
}

func (f *fakeAIDataPackBuilder) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.pack, nil
}

func TestBuildActionsAIFirstInput_BuildsFromDataPackWithoutCandidateBuilder(t *testing.T) {
	pack := testAIFirstDataPack()
	builder := &fakeAIDataPackBuilder{pack: &pack}
	svc := (&GenerationService{candidateBuilder: nil}).WithAIDataPackBuilder(builder)

	input, err := svc.BuildActionsAIFirstInput(context.Background(), 1, testMetricDate(), DefaultGenerateActionsOptions())
	if err != nil {
		t.Fatalf("BuildActionsAIFirstInput: %v", err)
	}
	if builder.calls != 1 {
		t.Fatalf("data pack builder calls=%d want 1", builder.calls)
	}
	if input.DataPack == nil || input.DataPack.SchemaVersion == "" || len(input.DataPack.FactRegistry) == 0 {
		t.Fatal("expected non-empty data pack in AI-first input")
	}
	if containsActionType(input.AllowedActionTypes, ActionTypeDataQualityIssue) {
		t.Fatal("active AI-first allowed types must not include data_quality_issue")
	}
	if containsActionType(input.AllowedActionTypes, ActionTypeKeySKURisk) {
		t.Fatal("active AI-first allowed types must not include key_sku_risk")
	}
	if containsActionType(input.AllowedActionTypes, ActionTypeAdHighDRR) {
		t.Fatal("active AI-first allowed types must not include ad_high_drr")
	}
	if containsActionCategory(input.AllowedCategories, ActionCategoryDataQuality) {
		t.Fatal("active AI-first allowed categories must not include data_quality")
	}
}
