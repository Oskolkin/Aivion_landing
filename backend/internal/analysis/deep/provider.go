package deep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
	Analyze(ctx context.Context, req Request) (Response, error)
}

// TestProviderCallCount is incremented on every Fake/OpenAI Analyze call (integration tests).
var TestProviderCallCount int

type FakeProvider struct {
	Responses []Response
	Errs      []error
	Calls     int
	LastReq   Request
}

func (f *FakeProvider) Analyze(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	TestProviderCallCount++
	f.LastReq = req
	i := f.Calls
	f.Calls++
	if i < len(f.Errs) && f.Errs[i] != nil {
		return Response{}, f.Errs[i]
	}
	if i < len(f.Responses) {
		return f.Responses[i], nil
	}
	if f.Responses != nil {
		return Response{Status: "incomplete", IncompleteReason: "exhausted_scripted_responses"}, nil
	}
	return Response{Status: "completed", OutputText: FixtureJSON(req.PayloadJSON)}, nil
}

type AttemptClass string

const (
	AttemptValid      AttemptClass = "VALID"
	AttemptRefused    AttemptClass = "REFUSED"
	AttemptIncomplete AttemptClass = "INCOMPLETE"
	AttemptInvalid    AttemptClass = "INVALID"
	AttemptProvider   AttemptClass = "PROVIDER_ERROR"
)

func ShouldRerun(class AttemptClass, attemptNo, max int) bool {
	if attemptNo >= max {
		return false
	}
	switch class {
	case AttemptRefused, AttemptIncomplete, AttemptInvalid:
		return true
	default:
		return false
	}
}

type ModelAttempt struct {
	No          int
	RequestHash string
	Model       string
	Response    Response
	Output      OutputV1
	Class       AttemptClass
	Report      ValidationReportV1
}

type AttemptRecorder func(ctx context.Context, at ModelAttempt) error

func ClassifyResponse(resp Response, err error, in ValidateInput) (OutputV1, AttemptClass, ValidationReportV1) {
	if err != nil {
		return OutputV1{}, AttemptProvider, ValidationReportV1{Valid: false, Errors: []string{err.Error()}}
	}
	st := strings.ToLower(strings.TrimSpace(resp.Status))
	if resp.Refusal != "" || st == "refused" {
		return OutputV1{}, AttemptRefused, ValidationReportV1{Valid: false, Errors: []string{CodeProviderRefused}}
	}
	if st == "incomplete" || (resp.IncompleteReason != "" && st != "completed") {
		return OutputV1{}, AttemptIncomplete, ValidationReportV1{Valid: false, Errors: []string{CodeProviderIncomplete + ":" + resp.IncompleteReason}}
	}
	if strings.TrimSpace(resp.OutputText) == "" {
		return OutputV1{}, AttemptInvalid, ValidationReportV1{Valid: false, Errors: []string{CodeProviderInvalidResponse}}
	}
	var out OutputV1
	if e := json.Unmarshal([]byte(resp.OutputText), &out); e != nil {
		return OutputV1{}, AttemptInvalid, ValidationReportV1{Valid: false, Errors: []string{"parse: " + e.Error()}}
	}
	in.Output = out
	rep := Validate(in)
	if !rep.Valid {
		return out, AttemptInvalid, rep
	}
	return out, AttemptValid, rep
}

func RunAttempts(ctx context.Context, p Provider, req Request, vin ValidateInput, max int, rec ...AttemptRecorder) ([]ModelAttempt, OutputV1, error) {
	if p == nil {
		return nil, OutputV1{}, packErr(CodeProviderNotConfigured, "provider missing")
	}
	if max <= 0 {
		max = CurrentAttemptPolicy().MaxModelAttempts
	}
	var record AttemptRecorder
	if len(rec) > 0 {
		record = rec[0]
	}
	reqHash := HashRequest(req)
	attempts := make([]ModelAttempt, 0, max)
	for i := 1; i <= max; i++ {
		resp, err := p.Analyze(ctx, req)
		out, class, rep := ClassifyResponse(resp, err, vin)
		at := ModelAttempt{No: i, RequestHash: reqHash, Model: req.Model, Response: resp, Output: out, Class: class, Report: rep}
		attempts = append(attempts, at)
		if record != nil {
			if perr := record(ctx, at); perr != nil {
				return attempts, OutputV1{}, fmt.Errorf("persist deep AI attempt: %w", perr)
			}
		}
		if class == AttemptValid {
			ObserveAI("valid", resp.Latency.Seconds(), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens, resp.Usage.ReasoningTokens)
			return attempts, out, nil
		}
		if class == AttemptProvider {
			ObserveAI("provider_error", resp.Latency.Seconds(), 0, 0, 0, 0)
			return attempts, OutputV1{}, err
		}
		ObserveAI(string(class), resp.Latency.Seconds(), resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens, resp.Usage.ReasoningTokens)
		if !ShouldRerun(class, i, max) {
			code := CodeValidationFailed
			if class == AttemptRefused {
				code = CodeProviderRefused
			}
			if class == AttemptIncomplete {
				code = CodeProviderIncomplete
			}
			return attempts, OutputV1{}, packErr(code, strings.Join(rep.Errors, "; "))
		}
	}
	return attempts, OutputV1{}, packErr(CodeValidationFailed, "exhausted attempts")
}
