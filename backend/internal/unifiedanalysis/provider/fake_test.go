package provider

import (
	"context"
	"strings"
	"testing"
)

func TestFakeProviderCompletedWithCodeInterpreter(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID("pack:test:001")

	ctx := context.Background()
	container, err := fake.CreateContainer(ctx, CreateContainerRequest{Name: "run-1", MemoryLimit: "4g"})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if !strings.HasPrefix(container.ID, fakeContainerID) {
		t.Fatalf("container id=%q", container.ID)
	}

	file, err := fake.UploadContainerFile(ctx, UploadContainerFileRequest{
		ContainerID: container.ID,
		Filename:    "analysis_pack.zip",
		Content:     []byte("zip"),
	})
	if err != nil {
		t.Fatalf("UploadContainerFile: %v", err)
	}
	if !strings.HasPrefix(file.ID, fakeContainerFileID) {
		t.Fatalf("file id=%q", file.ID)
	}

	started, err := fake.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{
		Model:        "fake-model",
		ContainerID:  container.ID,
		Instructions: SmokeInstructions(),
		Input:        SmokeInput(),
	})
	if err != nil {
		t.Fatalf("StartBackgroundResponse: %v", err)
	}
	if !strings.HasPrefix(started.ID, fakeResponseID) {
		t.Fatalf("response id=%q", started.ID)
	}

	fake.AdvanceResponse(started.ID)
	completed, err := fake.RetrieveResponse(ctx, started.ID)
	if err != nil {
		t.Fatalf("RetrieveResponse: %v", err)
	}
	if completed.Status != ProviderResponseStatusCompleted {
		t.Fatalf("status=%q", completed.Status)
	}
	if !HasCodeInterpreterCall(completed) {
		t.Fatal("expected code interpreter call")
	}
	if !strings.Contains(completed.OutputText, "pack:test:001") || !strings.Contains(completed.OutputText, "aivion_analysis_pack_v1") {
		t.Fatalf("output=%q", completed.OutputText)
	}

	if fake.CreateContainerCount != 1 || fake.UploadCount != 1 || fake.StartResponseCount != 1 || fake.RetrieveCount != 1 {
		t.Fatalf("counts create=%d upload=%d start=%d retrieve=%d", fake.CreateContainerCount, fake.UploadCount, fake.StartResponseCount, fake.RetrieveCount)
	}
}

func TestFakeProviderQueuedThenCompleted(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioQueuedThenCompleted)
	ctx := context.Background()

	started, err := fake.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{
		Model: "fake", ContainerID: fakeContainerID, Input: SmokeInput(),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	first, err := fake.RetrieveResponse(ctx, started.ID)
	if err != nil {
		t.Fatalf("retrieve1: %v", err)
	}
	if first.Status != ProviderResponseStatusQueued {
		t.Fatalf("first status=%q", first.Status)
	}

	fake.AdvanceResponse(started.ID)
	second, err := fake.RetrieveResponse(ctx, started.ID)
	if err != nil {
		t.Fatalf("retrieve2: %v", err)
	}
	if second.Status != ProviderResponseStatusCompleted {
		t.Fatalf("second status=%q", second.Status)
	}
	if fake.RetrieveCount != 2 {
		t.Fatalf("retrieve count=%d", fake.RetrieveCount)
	}
}

func TestFakeProviderErrorScenarios(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		scenario FakeScenario
		action   func(p *FakeProvider) error
		code     string
	}{
		{
			name:     "container_create_error",
			scenario: FakeScenarioContainerCreateError,
			action: func(p *FakeProvider) error {
				_, err := p.CreateContainer(ctx, CreateContainerRequest{Name: "x"})
				return err
			},
			code: CodeProviderContainerCreateFailed,
		},
		{
			name:     "file_upload_error",
			scenario: FakeScenarioFileUploadError,
			action: func(p *FakeProvider) error {
				_, err := p.UploadContainerFile(ctx, UploadContainerFileRequest{ContainerID: "c", Content: []byte("x")})
				return err
			},
			code: CodeProviderFileUploadFailed,
		},
		{
			name:     "response_start_error",
			scenario: FakeScenarioResponseStartError,
			action: func(p *FakeProvider) error {
				_, err := p.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{Model: "m", ContainerID: "c"})
				return err
			},
			code: CodeProviderResponseStartFailed,
		},
		{
			name:     "retrieve_transport_error",
			scenario: FakeScenarioRetrieveTransportError,
			action: func(p *FakeProvider) error {
				p.responses[fakeResponseID] = &fakeResponseState{response: &ProviderResponse{ID: fakeResponseID}}
				_, err := p.RetrieveResponse(ctx, fakeResponseID)
				return err
			},
			code: CodeProviderResponseRetrieveFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewFakeProvider(tc.scenario)
			err := tc.action(p)
			if err == nil {
				t.Fatal("expected error")
			}
			pe, ok := AsError(err)
			if !ok || pe.Code != tc.code {
				t.Fatalf("error=%v code=%q", err, pe.Code)
			}
		})
	}
}

func TestFakeProviderCleanup404Success(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioCompletedWithCodeInterpreter)
	ctx := context.Background()
	if err := CleanupContainer(ctx, fake, "missing", "missing"); err != nil {
		t.Fatalf("CleanupContainer missing resources: %v", err)
	}
	if fake.DeleteFileCount != 1 || fake.DeleteContainerCount != 1 {
		t.Fatalf("delete counts file=%d container=%d", fake.DeleteFileCount, fake.DeleteContainerCount)
	}
}

func TestFakeProviderCleanupError(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioCleanupError)
	ctx := context.Background()
	_, _ = fake.CreateContainer(ctx, CreateContainerRequest{Name: "x"})
	err := CleanupContainer(ctx, fake, fakeContainerID, fakeContainerFileID)
	if err == nil {
		t.Fatal("expected cleanup error")
	}
	pe, ok := AsError(err)
	if !ok || pe.Code != CodeProviderCleanupFailed {
		t.Fatalf("error=%v", err)
	}
}

func TestFakeProviderCancelled(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioCancelled)
	ctx := context.Background()
	started, err := fake.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{Model: "m", ContainerID: fakeContainerID})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	cancelled, err := fake.CancelResponse(ctx, started.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != ProviderResponseStatusCancelled {
		t.Fatalf("status=%q", cancelled.Status)
	}
	if fake.CancelCount != 1 {
		t.Fatalf("cancel count=%d", fake.CancelCount)
	}
}

func TestFakeProviderCompletedWithoutCodeInterpreter(t *testing.T) {
	fake := NewFakeProvider(FakeScenarioCompletedWithoutCodeInterpreter)
	ctx := context.Background()
	started, _ := fake.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{Model: "m", ContainerID: fakeContainerID})
	fake.AdvanceResponse(started.ID)
	resp, err := fake.RetrieveResponse(ctx, started.ID)
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if HasCodeInterpreterCall(resp) {
		t.Fatal("expected no code interpreter call")
	}
}
