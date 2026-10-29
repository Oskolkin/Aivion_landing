package ingestion

import (
	"sync"

	"github.com/hibiken/asynq"
)

type fakeEnqueuer struct {
	mu    sync.Mutex
	tasks []*asynq.Task
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "fake"}, nil
}
