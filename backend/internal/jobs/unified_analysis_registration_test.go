package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestRegisterUnifiedAnalysisHandlers(t *testing.T) {
	mux := asynq.NewServeMux()
	handler := NewUnifiedAnalysisHandler(nil, nil, nil)
	RegisterUnifiedAnalysisHandlers(mux, handler)

	types := []string{
		TaskTypeUnifiedAnalysisStart,
		TaskTypeUnifiedAnalysisPoll,
		TaskTypeUnifiedAnalysisCleanup,
		TaskTypeUnifiedAnalysisCancel,
	}
	for _, typ := range types {
		task := asynq.NewTask(typ, []byte(`{"run_id":1}`))
		err := mux.ProcessTask(context.Background(), task)
		// Handler returns an error because service is nil, but routing must succeed
		// (unknown task types return ErrHandlerNotFound).
		if err == nil {
			t.Fatalf("%s: expected handler error with nil service", typ)
		}
		if err == asynq.ErrHandlerNotFound {
			t.Fatalf("%s: handler not registered", typ)
		}
	}
}

func TestUnifiedAnalysisPollTaskIDIncludesAttempt(t *testing.T) {
	a := UnifiedAnalysisPollTaskID(42, 1)
	b := UnifiedAnalysisPollTaskID(42, 2)
	if a == b {
		t.Fatalf("poll task ids must differ by attempt: %s", a)
	}
	if a != "unified-analysis-poll:42:1" {
		t.Fatalf("task id=%q", a)
	}
}

func TestUnifiedAnalysisTaskOptionDefaults(t *testing.T) {
	if UnifiedAnalysisStartTimeout < time.Minute {
		t.Fatalf("start timeout too small: %s", UnifiedAnalysisStartTimeout)
	}
	if UnifiedAnalysisPollTimeout < time.Second {
		t.Fatalf("poll timeout too small: %s", UnifiedAnalysisPollTimeout)
	}
	if UnifiedAnalysisCleanupTimeout < time.Minute {
		t.Fatalf("cleanup timeout too small: %s", UnifiedAnalysisCleanupTimeout)
	}
	if UnifiedAnalysisMaxStartRetries < 1 || UnifiedAnalysisMaxCleanupRetries < 1 {
		t.Fatal("expected positive max retries")
	}
}
