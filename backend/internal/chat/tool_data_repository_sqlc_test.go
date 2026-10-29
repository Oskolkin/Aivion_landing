package chat

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSQLCToolDataRepositoryImplementsInterface(t *testing.T) {
	var _ ToolDataRepository = (*SQLCToolDataRepository)(nil)
}

func TestMapRecommendationOmitsRawResponseFieldByContract(t *testing.T) {
	row := dbgen.Recommendation{
		ID:            42,
		Title:         "Raise price",
		Status:        "open",
		EntityType:    "sku",
		EntitySku:     pgtype.Int8{Int64: 101, Valid: true},
		Horizon:       "short_term",
		PriorityLevel: "high",
	}
	got := mapRecommendation(row)
	if got.ID != 42 {
		t.Fatalf("expected id 42, got %d", got.ID)
	}
	if got.Title != "Raise price" {
		t.Fatalf("unexpected title: %s", got.Title)
	}
	if got.EntitySKU == nil || *got.EntitySKU != 101 {
		t.Fatalf("expected entity sku 101, got %#v", got.EntitySKU)
	}
}

func TestMapAlertCompactsEvidence(t *testing.T) {
	row := dbgen.Alert{
		ID:              7,
		AlertType:       "price_economics",
		AlertGroup:      "price_economics",
		EntityType:      "sku",
		Title:           "Margin risk",
		Message:         "Below min margin",
		Severity:        "high",
		Urgency:         "high",
		Status:          "open",
		EvidencePayload: []byte(`{"current_price":120,"effective_min_price":125,"raw_blob":"skip"}`),
	}
	got := mapAlert(row)
	if got.ID != 7 {
		t.Fatalf("expected id 7, got %d", got.ID)
	}
	if _, ok := got.Evidence["raw_blob"]; ok {
		t.Fatalf("expected evidence to be compacted without raw_blob")
	}
	if _, ok := got.Evidence["current_price"]; !ok {
		t.Fatalf("expected compact evidence to keep current_price")
	}
}
