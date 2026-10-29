package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ModelAttempt struct {
	No          int
	RequestHash string
	Model       string
	Response    Response
	Output      OutputV1
	Class       AttemptClass
	Report      ValidationReportV1
}

func ClassifyResponse(resp Response, err error, in ValidateInput) (OutputV1, AttemptClass, ValidationReportV1) {
	if err != nil {
		return OutputV1{}, AttemptProvider, ValidationReportV1{Valid: false, Errors: []string{err.Error()}}
	}
	st := strings.ToLower(strings.TrimSpace(resp.Status))
	if resp.Refusal != "" || st == "refused" {
		return OutputV1{}, AttemptRefused, ValidationReportV1{Valid: false, Errors: []string{CodeProviderRefused}}
	}
	if st == "incomplete" || resp.IncompleteReason != "" && st != "completed" {
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

type AttemptRecorder func(ctx context.Context, at ModelAttempt) error

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
		resp, err := p.Discover(ctx, req)
		out, class, rep := ClassifyResponse(resp, err, vin)
		at := ModelAttempt{No: i, RequestHash: reqHash, Model: req.Model, Response: resp, Output: out, Class: class, Report: rep}
		attempts = append(attempts, at)
		if record != nil {
			if perr := record(ctx, at); perr != nil {
				return attempts, OutputV1{}, fmt.Errorf("persist discovery AI attempt: %w", perr)
			}
		}
		if class == AttemptValid {
			return attempts, out, nil
		}
		if class == AttemptProvider {
			return attempts, OutputV1{}, err
		}
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
	return attempts, OutputV1{}, packErr(CodeValidationFailed, "max model attempts exhausted")
}
