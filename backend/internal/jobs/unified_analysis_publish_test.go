package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestUnifiedAnalysisPublishTaskType(t *testing.T) {
	if TaskTypeUnifiedAnalysisPublish != "unified_analysis.publish" {
		t.Fatalf("publish task type=%q", TaskTypeUnifiedAnalysisPublish)
	}
}

func TestUnifiedAnalysisPublishPayloadMarshal(t *testing.T) {
	task, err := NewUnifiedAnalysisPublishTask(128)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"run_id":128}`
	if string(task.Payload()) != want {
		t.Fatalf("payload=%s want=%s", task.Payload(), want)
	}
}

func TestUnifiedAnalysisPublishTaskOptionDefaults(t *testing.T) {
	if UnifiedAnalysisPublishTimeout < time.Minute {
		t.Fatalf("publish timeout too small: %s", UnifiedAnalysisPublishTimeout)
	}
	if UnifiedAnalysisPublishUniqueTTL < UnifiedAnalysisPublishTimeout {
		t.Fatal("publish unique TTL should be at least as long as the task timeout")
	}
	if UnifiedAnalysisMaxPublishRetries < 1 {
		t.Fatal("expected positive max publish retries")
	}
}

func TestRegisterUnifiedAnalysisPublishHandler(t *testing.T) {
	mux := asynq.NewServeMux()
	handler := NewUnifiedAnalysisPublishHandler(nil, nil)
	RegisterUnifiedAnalysisPublishHandler(mux, handler)

	task := asynq.NewTask(TaskTypeUnifiedAnalysisPublish, []byte(`{"run_id":1}`))
	err := mux.ProcessTask(context.Background(), task)
	// Handler returns an error because the publication service is nil, but
	// routing must succeed (unknown task types return ErrHandlerNotFound).
	if err == nil {
		t.Fatal("expected handler error with nil publication service")
	}
	if err == asynq.ErrHandlerNotFound {
		t.Fatal("unified_analysis.publish handler not registered")
	}
}

func TestUnifiedAnalysisPublishHandler_Handle_RejectsInvalidRunID(t *testing.T) {
	handler := NewUnifiedAnalysisPublishHandler(nil, nil)
	if err := handler.Handle(context.Background(), []byte(`{"run_id":0}`)); err == nil {
		t.Fatal("expected error for run_id=0")
	}
	if err := handler.Handle(context.Background(), []byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid payload JSON")
	}
}

func TestMaybeEnqueueAutoPublish_NilSafety(t *testing.T) {
	// autoPublishQueries never set via EnableAutoPublish: must no-op without
	// touching the (nil) database or enqueue.
	h := NewUnifiedAnalysisHandler(nil, nil, nil)
	if err := h.maybeEnqueueAutoPublish(context.Background(), nil); err != nil {
		t.Fatalf("nil run must no-op, got err=%v", err)
	}

	var hNil *UnifiedAnalysisHandler
	if err := hNil.maybeEnqueueAutoPublish(context.Background(), nil); err != nil {
		t.Fatalf("nil handler must no-op, got err=%v", err)
	}
}
