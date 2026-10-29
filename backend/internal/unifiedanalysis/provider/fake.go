package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	fakeContainerID     = "cntr_test_001"
	fakeContainerFileID = "cfile_test_001"
	fakeResponseID      = "resp_test_001"
)

// FakeScenario configures deterministic fake provider behavior.
type FakeScenario string

const (
	FakeScenarioCompletedWithCodeInterpreter    FakeScenario = "completed_with_code_interpreter"
	FakeScenarioCompletedWithoutCodeInterpreter FakeScenario = "completed_without_code_interpreter"
	FakeScenarioQueuedThenCompleted             FakeScenario = "queued_then_completed"
	FakeScenarioInProgressThenCompleted         FakeScenario = "in_progress_then_completed"
	FakeScenarioFailed                          FakeScenario = "failed"
	FakeScenarioIncomplete                      FakeScenario = "incomplete"
	FakeScenarioCancelled                       FakeScenario = "cancelled"
	FakeScenarioRetrieveTransportError          FakeScenario = "retrieve_transport_error"
	FakeScenarioContainerCreateError            FakeScenario = "container_create_error"
	FakeScenarioFileUploadError                 FakeScenario = "file_upload_error"
	FakeScenarioResponseStartError              FakeScenario = "response_start_error"
	FakeScenarioCleanupError                    FakeScenario = "cleanup_error"
	FakeScenarioExpiredContainer                FakeScenario = "expired_container"
	FakeScenarioInProgressUntilTimeout          FakeScenario = "in_progress_until_timeout"
	FakeScenarioRetrieveThenComplete            FakeScenario = "retrieve_then_complete"
)

// FakeAttemptScript describes a single scripted provider attempt for
// multi-attempt (retry) tests. When FakeProvider.AttemptScripts is non-empty,
// each StartBackgroundResponse consumes the next script in order and the
// matching RetrieveResponse returns that script's terminal state immediately.
type FakeAttemptScript struct {
	// Status is the terminal status the retrieve returns (completed, incomplete,
	// failed, cancelled). Empty defaults to completed.
	Status ProviderResponseStatus

	// IncompleteReason is used when Status == incomplete (default max_output_tokens).
	IncompleteReason string

	// ToolOnly reproduces the tool-only completed bug: a Code Interpreter call is
	// present, AssistantMessageCount == 0, OutputText == "" and OutputItemTypes ==
	// ["code_interpreter_call"] (no assistant message item).
	ToolOnly bool

	// Refusal, when set on a completed attempt, is surfaced both in the raw output
	// and on ProviderResponse.Refusal so terminal handling treats it as a refusal.
	Refusal string

	// OutputText overrides the completed assistant message text.
	OutputText string

	// AnalysisJSON, when set, is returned as the completed assistant message text
	// (structured analysis output). Takes precedence over OutputText.
	AnalysisJSON []byte

	// ErrorCode/ErrorMessage populate the provider error for Status == failed.
	ErrorCode    string
	ErrorMessage string
}

// FakeProvider is a deterministic in-memory Provider for tests.
type FakeProvider struct {
	mu sync.Mutex

	Scenario FakeScenario

	// AttemptScripts drives deterministic multi-attempt (retry) tests. When set,
	// each StartBackgroundResponse uses the next script and RetrieveResponse
	// returns its terminal state immediately (no queued/in_progress step).
	AttemptScripts []FakeAttemptScript

	// LastStartRequest records the most recent StartBackgroundResponse request so
	// tests can assert the effective model, tool_choice and max_output_tokens.
	LastStartRequest StartBackgroundResponseRequest

	CreateContainerCount int
	UploadCount          int
	StartResponseCount   int
	RetrieveCount        int
	CancelCount          int
	DeleteContainerCount int
	DeleteFileCount      int

	CleanupFailRemaining int

	smokePackID string

	// analysisOutputJSON, when set, is returned as OutputText for completed responses
	// (Stage 4 structured output scenarios).
	analysisOutputJSON []byte

	// refusalInOutput is embedded in RawJSON output items for Stage 4 refusal tests.
	// It is intentionally not copied to ProviderResponse.Refusal so poll-time smoke
	// checks pass while validate_output loads refusal from the saved artifact.
	refusalInOutput string

	containers    map[string]*Container
	files         map[string]*ContainerFile
	responses     map[string]*fakeResponseState
	seq           int
	attemptCursor int
}

type fakeResponseState struct {
	response        *ProviderResponse
	containerID     string
	retrieveStep    int
	cancelled       bool
	requestID       string
	maxOutputTokens int
	script          *FakeAttemptScript
}

// NewFakeProvider returns a fake provider with the default completed scenario.
func NewFakeProvider(scenario FakeScenario) *FakeProvider {
	if scenario == "" {
		scenario = FakeScenarioCompletedWithCodeInterpreter
	}
	return &FakeProvider{
		Scenario:    scenario,
		smokePackID: "pack:1:2026-07-08:c2f97b1999675dcd",
		containers:  make(map[string]*Container),
		files:       make(map[string]*ContainerFile),
		responses:   make(map[string]*fakeResponseState),
	}
}

// SetSmokePackID configures pack_id included in completed smoke output.
func (f *FakeProvider) SetSmokePackID(packID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.smokePackID = strings.TrimSpace(packID)
}

// SetAnalysisOutputJSON configures structured output JSON for completed analysis responses.
func (f *FakeProvider) SetAnalysisOutputJSON(raw []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.analysisOutputJSON = append([]byte(nil), raw...)
}

// SetRefusal configures a provider refusal embedded in RawJSON output (not ProviderResponse.Refusal).
func (f *FakeProvider) SetRefusal(message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refusalInOutput = strings.TrimSpace(message)
}

// AdvanceResponse advances multi-step response scenarios by one retrieve step.
func (f *FakeProvider) AdvanceResponse(responseID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.responses[strings.TrimSpace(responseID)]
	if !ok {
		return
	}
	state.retrieveStep++
}

func (f *FakeProvider) CreateContainer(ctx context.Context, req CreateContainerRequest) (*Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CreateContainerCount++

	if f.Scenario == FakeScenarioContainerCreateError {
		return nil, newProviderError(CodeProviderContainerCreateFailed, "fake container create failed", 500, "req_fake_create", nil)
	}
	if f.Scenario == FakeScenarioExpiredContainer {
		return nil, newProviderError(CodeProviderContainerExpired, "fake container expired", 410, "req_fake_expired", nil)
	}

	now := time.Now().UTC()
	f.seq++
	id := fmt.Sprintf("%s_%d_%d", fakeContainerID, time.Now().UnixNano(), f.seq)
	container := &Container{
		ID:          id,
		Name:        strings.TrimSpace(req.Name),
		Status:      "active",
		MemoryLimit: strings.TrimSpace(req.MemoryLimit),
		CreatedAt:   now,
	}
	f.containers[container.ID] = container
	return cloneContainer(container), nil
}

func (f *FakeProvider) RetrieveContainer(ctx context.Context, containerID string) (*Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	containerID = strings.TrimSpace(containerID)
	container, ok := f.containers[containerID]
	if !ok {
		if f.Scenario == FakeScenarioExpiredContainer {
			return nil, newProviderError(CodeProviderContainerExpired, "fake container expired", 410, "req_fake_expired", nil)
		}
		return nil, notFoundError(CodeProviderContainerRetrieveFailed, "fake container not found", "req_fake_retrieve")
	}
	return cloneContainer(container), nil
}

func (f *FakeProvider) UploadContainerFile(ctx context.Context, req UploadContainerFileRequest) (*ContainerFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.UploadCount++

	if f.Scenario == FakeScenarioFileUploadError {
		return nil, newProviderError(CodeProviderFileUploadFailed, "fake file upload failed", 500, "req_fake_upload", nil)
	}

	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "analysis_pack.zip"
	}
	now := time.Now().UTC()
	f.seq++
	file := &ContainerFile{
		ID:          fmt.Sprintf("%s_%d_%d", fakeContainerFileID, time.Now().UnixNano(), f.seq),
		ContainerID: strings.TrimSpace(req.ContainerID),
		Path:        "/" + filename,
		Bytes:       int64(len(req.Content)),
		Source:      "upload",
		CreatedAt:   now,
	}
	key := file.ContainerID + ":" + file.ID
	f.files[key] = file
	return cloneContainerFile(file), nil
}

func (f *FakeProvider) StartBackgroundResponse(ctx context.Context, req StartBackgroundResponseRequest) (*ProviderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.StartResponseCount++
	f.LastStartRequest = req

	if f.Scenario == FakeScenarioResponseStartError {
		return nil, newProviderError(CodeProviderResponseStartFailed, "fake response start failed", 500, "req_fake_start", nil)
	}

	f.seq++
	requestID := fmt.Sprintf("req_fake_%d", f.seq)

	// Multi-attempt scripted mode: each start consumes the next script and the
	// response begins queued; the matching retrieve returns the terminal state.
	if len(f.AttemptScripts) > 0 {
		idx := f.attemptCursor
		if idx >= len(f.AttemptScripts) {
			idx = len(f.AttemptScripts) - 1
		}
		script := f.AttemptScripts[idx]
		f.attemptCursor++

		resp := &ProviderResponse{
			ID:        fmt.Sprintf("%s_%d_%d", fakeResponseID, time.Now().UnixNano(), f.seq),
			Status:    ProviderResponseStatusQueued,
			Model:     firstNonEmpty(req.Model, "fake-model"),
			CreatedAt: time.Now().UTC(),
			RequestID: requestID,
		}
		scriptCopy := script
		state := &fakeResponseState{
			response:        resp,
			containerID:     strings.TrimSpace(req.ContainerID),
			requestID:       requestID,
			maxOutputTokens: req.MaxOutputTokens,
			script:          &scriptCopy,
		}
		f.responses[resp.ID] = state
		return cloneProviderResponse(resp), nil
	}

	status := f.initialResponseStatus()
	resp := f.buildResponse(status, req.ContainerID)
	resp.ID = fmt.Sprintf("%s_%d_%d", fakeResponseID, time.Now().UnixNano(), f.seq)
	resp.RequestID = requestID
	state := &fakeResponseState{
		response:        resp,
		containerID:     strings.TrimSpace(req.ContainerID),
		requestID:       requestID,
		maxOutputTokens: req.MaxOutputTokens,
	}
	f.responses[resp.ID] = state
	return cloneProviderResponse(resp), nil
}

func (f *FakeProvider) RetrieveResponse(ctx context.Context, responseID string) (*ProviderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RetrieveCount++

	if f.Scenario == FakeScenarioRetrieveTransportError {
		return nil, newProviderError(CodeProviderResponseRetrieveFailed, "fake retrieve transport error", 503, "req_fake_retrieve", nil)
	}

	responseID = strings.TrimSpace(responseID)
	state, ok := f.responses[responseID]
	if !ok {
		return nil, notFoundError(CodeProviderResponseRetrieveFailed, "fake response not found", "req_fake_retrieve")
	}

	// Scripted multi-attempt mode: return the terminal state immediately.
	if state.script != nil {
		resp := f.buildScriptedResponse(*state.script, state.containerID, state.maxOutputTokens)
		resp.ID = responseID
		resp.RequestID = state.requestID
		if state.cancelled {
			resp = f.buildResponse(ProviderResponseStatusCancelled, state.containerID)
			resp.ID = responseID
			resp.RequestID = state.requestID
		}
		state.response = resp
		return cloneProviderResponse(resp), nil
	}

	status := state.response.Status
	switch f.Scenario {
	case FakeScenarioQueuedThenCompleted:
		if state.retrieveStep == 0 {
			status = ProviderResponseStatusQueued
		} else {
			status = ProviderResponseStatusCompleted
		}
		state.retrieveStep++
	case FakeScenarioInProgressThenCompleted:
		if state.retrieveStep == 0 {
			status = ProviderResponseStatusInProgress
		} else {
			status = ProviderResponseStatusCompleted
		}
		state.retrieveStep++
	case FakeScenarioInProgressUntilTimeout:
		status = ProviderResponseStatusInProgress
		state.retrieveStep++
	case FakeScenarioFailed:
		status = ProviderResponseStatusFailed
	case FakeScenarioIncomplete:
		status = ProviderResponseStatusIncomplete
	case FakeScenarioCancelled:
		status = ProviderResponseStatusCancelled
	case FakeScenarioCompletedWithoutCodeInterpreter:
		status = ProviderResponseStatusCompleted
	case FakeScenarioCompletedWithCodeInterpreter:
		status = ProviderResponseStatusCompleted
	case FakeScenarioRetrieveThenComplete:
		// First retrieve fails with transport error; subsequent succeed completed.
		if state.retrieveStep == 0 {
			state.retrieveStep++
			return nil, newProviderError(CodeProviderResponseRetrieveFailed, "fake retrieve transport error", 503, "req_fake_retrieve", nil)
		}
		status = ProviderResponseStatusCompleted
		state.retrieveStep++
	}

	if state.cancelled {
		status = ProviderResponseStatusCancelled
	}

	resp := f.buildResponse(status, firstNonEmpty(state.containerID, firstContainerID(state.response)))
	resp.ID = responseID
	resp.RequestID = state.requestID
	state.response = resp
	return cloneProviderResponse(resp), nil
}

func (f *FakeProvider) CancelResponse(ctx context.Context, responseID string) (*ProviderResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CancelCount++

	responseID = strings.TrimSpace(responseID)
	state, ok := f.responses[responseID]
	if !ok {
		return nil, notFoundError(CodeProviderResponseCancelFailed, "fake response not found", "req_fake_cancel")
	}
	state.cancelled = true
	resp := f.buildResponse(ProviderResponseStatusCancelled, firstNonEmpty(state.containerID, firstContainerID(state.response)))
	resp.ID = responseID
	resp.RequestID = state.requestID
	state.response = resp
	return cloneProviderResponse(resp), nil
}

func (f *FakeProvider) DeleteContainerFile(ctx context.Context, containerID, fileID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeleteFileCount++

	if f.CleanupFailRemaining > 0 {
		f.CleanupFailRemaining--
		return newProviderError(CodeProviderFileDeleteFailed, "fake delete file failed", 500, "req_fake_delete_file", nil)
	}
	if f.Scenario == FakeScenarioCleanupError {
		return newProviderError(CodeProviderFileDeleteFailed, "fake delete file failed", 500, "req_fake_delete_file", nil)
	}

	key := strings.TrimSpace(containerID) + ":" + strings.TrimSpace(fileID)
	if _, ok := f.files[key]; !ok {
		return ErrNotFound
	}
	delete(f.files, key)
	return nil
}

func (f *FakeProvider) DeleteContainer(ctx context.Context, containerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DeleteContainerCount++

	if f.CleanupFailRemaining > 0 {
		f.CleanupFailRemaining--
		return newProviderError(CodeProviderContainerDeleteFailed, "fake delete container failed", 500, "req_fake_delete_container", nil)
	}
	if f.Scenario == FakeScenarioCleanupError {
		return newProviderError(CodeProviderContainerDeleteFailed, "fake delete container failed", 500, "req_fake_delete_container", nil)
	}

	containerID = strings.TrimSpace(containerID)
	if _, ok := f.containers[containerID]; !ok {
		return ErrNotFound
	}
	delete(f.containers, containerID)
	return nil
}

func (f *FakeProvider) initialResponseStatus() ProviderResponseStatus {
	switch f.Scenario {
	case FakeScenarioQueuedThenCompleted:
		return ProviderResponseStatusQueued
	case FakeScenarioInProgressThenCompleted, FakeScenarioInProgressUntilTimeout:
		return ProviderResponseStatusInProgress
	case FakeScenarioRetrieveThenComplete:
		return ProviderResponseStatusQueued
	case FakeScenarioFailed:
		return ProviderResponseStatusFailed
	case FakeScenarioIncomplete:
		return ProviderResponseStatusIncomplete
	case FakeScenarioCancelled:
		return ProviderResponseStatusCancelled
	default:
		return ProviderResponseStatusQueued
	}
}

func (f *FakeProvider) buildResponse(status ProviderResponseStatus, containerID string) *ProviderResponse {
	now := time.Now().UTC()
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		containerID = fakeContainerID
	}

	resp := &ProviderResponse{
		ID:        fakeResponseID,
		Status:    status,
		Model:     "fake-model",
		CreatedAt: now,
		Usage: ProviderUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	switch status {
	case ProviderResponseStatusCompleted:
		includeCI := f.Scenario != FakeScenarioCompletedWithoutCodeInterpreter
		if includeCI {
			resp.CodeInterpreterCalls = []CodeInterpreterCall{
				{
					ID:          "ci_call_001",
					ContainerID: containerID,
					Status:      "completed",
					Code:        "import zipfile\nprint('smoke')",
				},
			}
		}
		resp.OutputText = fmt.Sprintf(
			`{"pack_id":"%s","schema_version":"aivion_analysis_pack_v1","archive_entry_count":12,"manifest_files_present":true}`,
			f.smokePackID,
		)
		if len(f.analysisOutputJSON) > 0 {
			resp.OutputText = string(f.analysisOutputJSON)
			resp.AssistantMessageCount = 1
		}
		completed := now
		resp.CompletedAt = &completed
	case ProviderResponseStatusFailed:
		resp.Error = &ProviderResponseError{
			Code:    "invalid_request",
			Message: "fake response failed",
		}
	case ProviderResponseStatusIncomplete:
		resp.IncompleteDetails = &IncompleteDetails{Reason: "max_output_tokens"}
	case ProviderResponseStatusCancelled:
		resp.OutputText = ""
	}

	refusalInOutput := f.refusalInOutput
	resp.OutputItemTypes = fakeOutputItemTypes(resp, true, refusalInOutput)
	resp.RawJSON = marshalFakeRaw(resp, true, refusalInOutput)
	return resp
}

// buildScriptedResponse produces a terminal ProviderResponse for a scripted
// multi-attempt test. containerID is the container used at start; maxOutputTokens
// is the effective max_output_tokens requested for this attempt (used to make
// incomplete usage diagnostics realistic).
func (f *FakeProvider) buildScriptedResponse(script FakeAttemptScript, containerID string, maxOutputTokens int) *ProviderResponse {
	now := time.Now().UTC()
	containerID = firstNonEmpty(containerID, fakeContainerID)
	status := script.Status
	if status == "" {
		status = ProviderResponseStatusCompleted
	}

	resp := &ProviderResponse{
		ID:        fakeResponseID,
		Status:    status,
		Model:     "fake-model",
		CreatedAt: now,
		Usage: ProviderUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}

	includeMessage := false
	switch status {
	case ProviderResponseStatusCompleted:
		// Analysis always performs Code Interpreter work; a completed response
		// therefore has at least one CI call whether or not it emitted a message.
		resp.CodeInterpreterCalls = []CodeInterpreterCall{
			{ID: "ci_call_001", ContainerID: containerID, Status: "completed", Code: "import zipfile\nprint('analysis')"},
		}
		if script.ToolOnly {
			// Tool-only completed bug: CI present, no assistant message.
			resp.OutputText = ""
			resp.AssistantMessageCount = 0
			includeMessage = false
		} else {
			text := script.OutputText
			if len(script.AnalysisJSON) > 0 {
				text = string(script.AnalysisJSON)
			}
			resp.OutputText = text
			resp.AssistantMessageCount = 1
			includeMessage = true
		}
		if strings.TrimSpace(script.Refusal) != "" {
			resp.Refusal = strings.TrimSpace(script.Refusal)
		}
		completed := now
		resp.CompletedAt = &completed
	case ProviderResponseStatusIncomplete:
		reason := firstNonEmpty(script.IncompleteReason, "max_output_tokens")
		resp.IncompleteDetails = &IncompleteDetails{Reason: reason}
		outTokens := int64(16000)
		if maxOutputTokens > 0 {
			outTokens = int64(maxOutputTokens)
		}
		resp.Usage.OutputTokens = outTokens
		resp.Usage.TotalTokens = resp.Usage.InputTokens + outTokens
		// An incomplete response may still have run Code Interpreter before it
		// was truncated; include one call for realistic diagnostics.
		resp.CodeInterpreterCalls = []CodeInterpreterCall{
			{ID: "ci_call_001", ContainerID: containerID, Status: "completed", Code: "import zipfile"},
		}
	case ProviderResponseStatusFailed:
		resp.Error = &ProviderResponseError{
			Code:    firstNonEmpty(script.ErrorCode, "server_error"),
			Message: firstNonEmpty(script.ErrorMessage, "fake response failed"),
		}
	case ProviderResponseStatusCancelled:
		resp.OutputText = ""
	}

	refusalInOutput := strings.TrimSpace(script.Refusal)
	resp.OutputItemTypes = fakeOutputItemTypes(resp, includeMessage, refusalInOutput)
	resp.RawJSON = marshalFakeRaw(resp, includeMessage, refusalInOutput)
	return resp
}

func marshalFakeRaw(resp *ProviderResponse, includeMessage bool, refusalInOutput string) []byte {
	raw, _ := json.Marshal(map[string]any{
		"id":     resp.ID,
		"status": string(resp.Status),
		"model":  resp.Model,
		"output": buildFakeRawOutput(resp, includeMessage, refusalInOutput),
		"usage": map[string]any{
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
			"total_tokens":  resp.Usage.TotalTokens,
		},
	})
	return raw
}

// fakeOutputItemTypes returns the output item "type" values in raw payload order
// so ProviderResponse.OutputItemTypes matches buildFakeRawOutput.
func fakeOutputItemTypes(resp *ProviderResponse, includeMessage bool, refusalInOutput string) []string {
	var types []string
	if includeMessage {
		types = append(types, "message")
	}
	for range resp.CodeInterpreterCalls {
		types = append(types, "code_interpreter_call")
	}
	if strings.TrimSpace(refusalInOutput) != "" {
		types = append(types, "message")
	}
	return types
}

func buildFakeRawOutput(resp *ProviderResponse, includeMessage bool, refusalInOutput string) []any {
	items := []any{}
	if includeMessage {
		items = append(items, map[string]any{
			"type": "message",
			"content": []any{
				map[string]any{
					"type": "output_text",
					"text": resp.OutputText,
				},
			},
		})
	}
	for _, call := range resp.CodeInterpreterCalls {
		items = append(items, map[string]any{
			"type":         "code_interpreter_call",
			"id":           call.ID,
			"container_id": call.ContainerID,
			"status":       call.Status,
			"code":         call.Code,
		})
	}
	if strings.TrimSpace(refusalInOutput) != "" {
		items = append(items, map[string]any{
			"type": "message",
			"content": []any{
				map[string]any{
					"type":    "refusal",
					"refusal": refusalInOutput,
				},
			},
		})
	}
	return items
}

func firstContainerID(resp *ProviderResponse) string {
	if resp == nil {
		return fakeContainerID
	}
	for _, call := range resp.CodeInterpreterCalls {
		if strings.TrimSpace(call.ContainerID) != "" {
			return call.ContainerID
		}
	}
	return fakeContainerID
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func cloneContainer(in *Container) *Container {
	if in == nil {
		return nil
	}
	out := *in
	if in.LastActiveAt != nil {
		t := *in.LastActiveAt
		out.LastActiveAt = &t
	}
	if in.ExpiresAt != nil {
		t := *in.ExpiresAt
		out.ExpiresAt = &t
	}
	return &out
}

func cloneContainerFile(in *ContainerFile) *ContainerFile {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneProviderResponse(in *ProviderResponse) *ProviderResponse {
	if in == nil {
		return nil
	}
	out := *in
	out.RawJSON = append([]byte(nil), in.RawJSON...)
	out.CodeInterpreterCalls = append([]CodeInterpreterCall(nil), in.CodeInterpreterCalls...)
	out.OutputItemTypes = append([]string(nil), in.OutputItemTypes...)
	if in.CompletedAt != nil {
		t := *in.CompletedAt
		out.CompletedAt = &t
	}
	if in.Error != nil {
		errCopy := *in.Error
		out.Error = &errCopy
	}
	if in.IncompleteDetails != nil {
		detailsCopy := *in.IncompleteDetails
		out.IncompleteDetails = &detailsCopy
	}
	return &out
}
