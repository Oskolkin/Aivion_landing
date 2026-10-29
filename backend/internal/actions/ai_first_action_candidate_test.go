package actions

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAIFirstActionCandidateJSONRoundTrip(t *testing.T) {
	candidate := AIFirstActionCandidate{
		CandidateID:       "candidate-1",
		ChunkID:           "chunk-1",
		Type:              ActionTypeStockReplenishment,
		Category:          ActionCategoryStock,
		LocalPriority:     ActionPriorityCritical,
		Problem:           "У нескольких SKU низкий остаток",
		RecommendedAction: "Проверить поставки и пополнить остаток",
		WhyActionable:     "Товары могут закончиться",
		AffectedEntities:  []AIFirstAffectedEntityRef{},
		EvidenceFactIDs:   []string{},
		SourceSignalIDs:   []string{},
		Limitations:       []string{},
	}

	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AIFirstActionCandidate
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.CandidateID != candidate.CandidateID {
		t.Fatalf("candidate_id=%q", decoded.CandidateID)
	}
	if decoded.ChunkID != candidate.ChunkID {
		t.Fatalf("chunk_id=%q", decoded.ChunkID)
	}
	if decoded.Type != candidate.Type {
		t.Fatalf("type=%q", decoded.Type)
	}
	if decoded.Category != candidate.Category {
		t.Fatalf("category=%q", decoded.Category)
	}
	if decoded.LocalPriority != candidate.LocalPriority {
		t.Fatalf("local_priority=%q", decoded.LocalPriority)
	}
	if decoded.Problem != candidate.Problem {
		t.Fatalf("problem=%q", decoded.Problem)
	}
	if decoded.RecommendedAction != candidate.RecommendedAction {
		t.Fatalf("recommended_action=%q", decoded.RecommendedAction)
	}
	if decoded.WhyActionable != candidate.WhyActionable {
		t.Fatalf("why_actionable=%q", decoded.WhyActionable)
	}
	if decoded.AffectedEntities == nil {
		t.Fatal("affected_entities should not be nil")
	}
	if decoded.EvidenceFactIDs == nil {
		t.Fatal("evidence_fact_ids should not be nil")
	}
	if decoded.SourceSignalIDs == nil {
		t.Fatal("source_signal_ids should not be nil")
	}
}

func TestAIFirstActionCandidateOmitemptyTechnicalFields(t *testing.T) {
	candidate := AIFirstActionCandidate{
		Type:              ActionTypeStockReplenishment,
		Category:          ActionCategoryStock,
		LocalPriority:     ActionPriorityCritical,
		Problem:           "problem",
		RecommendedAction: "action",
		WhyActionable:     "why",
		AffectedEntities:  []AIFirstAffectedEntityRef{},
		EvidenceFactIDs:   []string{},
		SourceSignalIDs:   []string{},
	}

	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	payload := string(raw)
	if strings.Contains(payload, "candidate_id") {
		t.Fatalf("candidate_id should be omitted: %s", payload)
	}
	if strings.Contains(payload, "chunk_id") {
		t.Fatalf("chunk_id should be omitted: %s", payload)
	}
}
