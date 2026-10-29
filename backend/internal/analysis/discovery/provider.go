package discovery

import (
	"context"
	"sync/atomic"
	"time"
)

type Request struct {
	Model           string
	Instructions    string
	PayloadJSON     []byte
	Schema          map[string]any
	SchemaName      string
	MaxOutputTokens int
}

type Usage struct {
	InputTokens       int64
	OutputTokens      int64
	ReasoningTokens   int64
	CachedInputTokens int64
	TotalTokens       int64
}

type Response struct {
	Status            string
	OutputText        string
	Refusal           string
	IncompleteReason  string
	Usage             Usage
	ProviderRequestID string
	Latency           time.Duration
	RawJSON           []byte
}

type Provider interface {
	Discover(ctx context.Context, req Request) (Response, error)
}

func HashRequest(req Request) string {
	return HashFingerprint(FingerprintFromRequest(req))
}

type FakeProvider struct {
	Responses []Response
	Errs      []error
	Calls     int
	LastReq   Request
}

// TestDiscoverCalls counts FakeProvider Discover invocations process-wide (eval NO_OP tests).
var TestDiscoverCalls atomic.Int64

func (f *FakeProvider) Discover(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	TestDiscoverCalls.Add(1)
	f.LastReq = req
	i := f.Calls
	f.Calls++
	if i < len(f.Errs) && f.Errs[i] != nil {
		return Response{}, f.Errs[i]
	}
	if i < len(f.Responses) {
		return f.Responses[i], nil
	}
	if fakeNonEmptyEnabled() {
		return Response{Status: "completed", OutputText: NonEmptyFixtureJSON(req.PayloadJSON)}, nil
	}
	return Response{Status: "completed", OutputText: `{"discovery_state":"NO_MEANINGFUL_THREADS","threads":[]}`}, nil
}

func EmptyThreadsJSON() string {
	return `{"discovery_state":"NO_MEANINGFUL_THREADS","threads":[]}`
}
