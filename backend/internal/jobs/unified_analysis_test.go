package jobs

import (
	"testing"
)

func TestUnifiedAnalysisPayloadMarshal(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name: "start",
			payload: func() []byte {
				task, err := NewUnifiedAnalysisStartTask(42)
				if err != nil {
					t.Fatal(err)
				}
				return task.Payload()
			}(),
			want: `{"run_id":42}`,
		},
		{
			name: "poll",
			payload: func() []byte {
				task, err := NewUnifiedAnalysisPollTask(7, 3)
				if err != nil {
					t.Fatal(err)
				}
				return task.Payload()
			}(),
			want: `{"run_id":7,"poll_attempt":3}`,
		},
		{
			name: "cleanup",
			payload: func() []byte {
				task, err := NewUnifiedAnalysisCleanupTask(9)
				if err != nil {
					t.Fatal(err)
				}
				return task.Payload()
			}(),
			want: `{"run_id":9}`,
		},
		{
			name: "cancel",
			payload: func() []byte {
				task, err := NewUnifiedAnalysisCancelTask(11)
				if err != nil {
					t.Fatal(err)
				}
				return task.Payload()
			}(),
			want: `{"run_id":11}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.payload) != tc.want {
				t.Fatalf("payload=%s want=%s", tc.payload, tc.want)
			}
		})
	}
}

func TestUnifiedAnalysisTaskTypes(t *testing.T) {
	if TaskTypeUnifiedAnalysisStart != "unified_analysis.start" {
		t.Fatalf("start task type=%q", TaskTypeUnifiedAnalysisStart)
	}
	if TaskTypeUnifiedAnalysisPoll != "unified_analysis.poll" {
		t.Fatalf("poll task type=%q", TaskTypeUnifiedAnalysisPoll)
	}
	if TaskTypeUnifiedAnalysisCleanup != "unified_analysis.cleanup" {
		t.Fatalf("cleanup task type=%q", TaskTypeUnifiedAnalysisCleanup)
	}
	if TaskTypeUnifiedAnalysisCancel != "unified_analysis.cancel" {
		t.Fatalf("cancel task type=%q", TaskTypeUnifiedAnalysisCancel)
	}
}
