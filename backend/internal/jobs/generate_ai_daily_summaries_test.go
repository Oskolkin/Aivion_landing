package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

type stubAISummaries struct {
	called int64
	err    error
}

func (s *stubAISummaries) RunForSellerAfterSync(ctx context.Context, sellerAccountID int64) error {
	s.called++
	return s.err
}

func TestGenerateAIDailySummariesHandler_roundtrip(t *testing.T) {
	t.Parallel()
	st := &stubAISummaries{}
	h := NewGenerateAIDailySummariesHandler(st, nil, zap.NewNop())
	payload, err := json.Marshal(GenerateAIDailySummariesPayload{SellerAccountID: 9, SyncJobID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Handle(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if st.called != 1 {
		t.Fatalf("expected service call, got %d", st.called)
	}
}

func TestNewGenerateAIDailySummariesTask_type(t *testing.T) {
	t.Parallel()
	task, err := NewGenerateAIDailySummariesTask(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if task.Type() != TaskTypeGenerateAIDailySummaries {
		t.Fatalf("unexpected type %q", task.Type())
	}
}
