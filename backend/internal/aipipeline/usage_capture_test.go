package aipipeline

import (
	"context"
	"sync"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
)

type captureUsageRecorder struct {
	mu     sync.Mutex
	events []aicost.UsageEventInput
}

func newCaptureUsageRecorder() *captureUsageRecorder {
	return &captureUsageRecorder{}
}

func (c *captureUsageRecorder) Record(ctx context.Context, in aicost.UsageEventInput) error {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, in)
	return nil
}

func (c *captureUsageRecorder) Events() []aicost.UsageEventInput {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]aicost.UsageEventInput, len(c.events))
	copy(out, c.events)
	return out
}

func countUsageComponent(events []aicost.UsageEventInput, component string) int {
	n := 0
	for _, ev := range events {
		if ev.Component == component {
			n++
		}
	}
	return n
}

func sumUsageComponentTokens(events []aicost.UsageEventInput, component string) (in, out int) {
	for _, ev := range events {
		if ev.Component != component {
			continue
		}
		in += ev.InputTokens
		out += ev.OutputTokens
	}
	return in, out
}
