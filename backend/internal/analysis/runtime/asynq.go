package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
)

const TaskTypeStageExecute = "analysis.stage.execute"

type StageExecutePayload struct {
	RunID    int64  `json:"run_id"`
	StageKey string `json:"stage_key"`
}

func NewStageExecuteTask(runID int64, stageKey string) (*asynq.Task, error) {
	raw, err := json.Marshal(StageExecutePayload{RunID: runID, StageKey: stageKey})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeStageExecute, raw), nil
}

func ParseStageExecutePayload(t *asynq.Task) (StageExecutePayload, error) {
	var p StageExecutePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return StageExecutePayload{}, err
	}
	if p.RunID <= 0 || p.StageKey == "" {
		return StageExecutePayload{}, fmt.Errorf("runtime: invalid analysis.stage.execute payload")
	}
	return p, nil
}

func EnqueueStageExecute(ctx context.Context, client *asynq.Client, runID int64, stageKey string) error {
	if client == nil {
		return fmt.Errorf("runtime: asynq client is not configured")
	}
	task, err := NewStageExecuteTask(runID, stageKey)
	if err != nil {
		return err
	}
	_, err = client.EnqueueContext(ctx, task, asynq.TaskID(fmt.Sprintf("analysis-stage:%d:%s", runID, stageKey)))
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func RegisterStageExecuteHandler(mux *asynq.ServeMux, executor *Executor) {
	if mux == nil || executor == nil {
		return
	}
	mux.HandleFunc(TaskTypeStageExecute, func(ctx context.Context, t *asynq.Task) error {
		p, err := ParseStageExecutePayload(t)
		if err != nil {
			return err
		}
		return executor.ExecuteStage(ctx, p.RunID, p.StageKey)
	})
}

func AsynqEnqueueFunc(client *asynq.Client) EnqueueFunc {
	return func(ctx context.Context, runID int64, stageKey string) error {
		return EnqueueStageExecute(ctx, client, runID, stageKey)
	}
}
