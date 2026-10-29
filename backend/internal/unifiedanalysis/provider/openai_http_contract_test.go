package provider

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIHTTPContract(t *testing.T) {
	var createBody createContainerBody
	var uploadBytes []byte
	var uploadFilename string
	var responseBody openAIResponseCreateBody
	var cancelCalled bool
	var deleteFileCalled bool
	var deleteContainerCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_http_contract")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/containers":
			if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
				t.Errorf("authorization=%q", auth)
			}
			if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("content-type=%q", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create container: %v", err)
			}
			_ = json.NewEncoder(w).Encode(openAIContainerResponse{
				ID:          "cntr_live_001",
				Name:        createBody.Name,
				Status:      "active",
				MemoryLimit: createBody.MemoryLimit,
				CreatedAt:   json.RawMessage(`1735689600`),
			})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/containers/") && strings.HasSuffix(r.URL.Path, "/files"):
			mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
				t.Fatalf("multipart content-type: %v", err)
			}
			reader := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("next multipart part: %v", err)
				}
				if part.FormName() != "file" {
					continue
				}
				uploadFilename = part.FileName()
				uploadBytes, err = io.ReadAll(part)
				if err != nil {
					t.Fatalf("read upload bytes: %v", err)
				}
			}
			_ = json.NewEncoder(w).Encode(openAIContainerFileResponse{
				ID:          "cfile_live_001",
				ContainerID: "cntr_live_001",
				Path:        "/analysis_pack.zip",
				Bytes:       int64(len(uploadBytes)),
				Source:      "upload",
				CreatedAt:   json.RawMessage(`1735689600`),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/responses":
			if err := json.NewDecoder(r.Body).Decode(&responseBody); err != nil {
				t.Fatalf("decode response create: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "resp_live_001",
				"status": "queued",
				"model":  responseBody.Model,
				"output": []any{},
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
					"total_tokens":  0,
				},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/responses/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "resp_live_001",
				"status": "completed",
				"model":  "gpt-test",
				"output": []any{
					map[string]any{
						"type": "message",
						"content": []any{
							map[string]any{"type": "output_text", "text": "done"},
						},
					},
					map[string]any{
						"type":         "code_interpreter_call",
						"id":           "ci_1",
						"container_id": "cntr_live_001",
						"status":       "completed",
						"code":         "print('ok')",
					},
				},
				"usage": map[string]any{
					"input_tokens":  10,
					"output_tokens": 5,
					"total_tokens":  15,
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel"):
			cancelCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "resp_live_001",
				"status": "cancelled",
				"model":  "gpt-test",
				"output": []any{},
			})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/files/"):
			deleteFileCalled = true
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/containers/"):
			deleteContainerCalled = true
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewOpenAIClient(ClientConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}
	ctx := context.Background()

	container, err := client.CreateContainer(ctx, CreateContainerRequest{
		Name:        "aivion-unified-run-1",
		MemoryLimit: "4g",
	})
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if container.ID != "cntr_live_001" {
		t.Fatalf("container id=%q", container.ID)
	}
	if createBody.Name != "aivion-unified-run-1" || createBody.MemoryLimit != "4g" {
		t.Fatalf("create body=%+v", createBody)
	}

	zipBytes := []byte("PK\x03\x04fake-zip")
	file, err := client.UploadContainerFile(ctx, UploadContainerFileRequest{
		ContainerID: container.ID,
		Filename:    "analysis_pack.zip",
		ContentType: "application/zip",
		Content:     zipBytes,
	})
	if err != nil {
		t.Fatalf("UploadContainerFile: %v", err)
	}
	if file.ID != "cfile_live_001" {
		t.Fatalf("file id=%q", file.ID)
	}
	if uploadFilename != "analysis_pack.zip" {
		t.Fatalf("upload filename=%q", uploadFilename)
	}
	if string(uploadBytes) != string(zipBytes) {
		t.Fatalf("upload bytes mismatch")
	}

	started, err := client.StartBackgroundResponse(ctx, StartBackgroundResponseRequest{
		Model:           "gpt-test",
		ContainerID:     container.ID,
		Instructions:    SmokeInstructions(),
		Input:           SmokeInput(),
		MaxOutputTokens: 2000,
		Metadata: map[string]string{
			"prompt_version": PromptVersionSmokeV1,
		},
	})
	if err != nil {
		t.Fatalf("StartBackgroundResponse: %v", err)
	}
	if started.Status != ProviderResponseStatusQueued {
		t.Fatalf("started status=%q", started.Status)
	}
	if !responseBody.Background {
		t.Fatal("expected background=true")
	}
	if responseBody.ToolChoice != "required" {
		t.Fatalf("tool_choice=%q", responseBody.ToolChoice)
	}
	if len(responseBody.Tools) != 1 || responseBody.Tools[0].Type != "code_interpreter" || responseBody.Tools[0].Container != container.ID {
		t.Fatalf("tools=%+v", responseBody.Tools)
	}
	if responseBody.Metadata["prompt_version"] != PromptVersionSmokeV1 {
		t.Fatalf("metadata=%+v", responseBody.Metadata)
	}

	retrieved, err := client.RetrieveResponse(ctx, "resp_live_001")
	if err != nil {
		t.Fatalf("RetrieveResponse: %v", err)
	}
	if retrieved.Status != ProviderResponseStatusCompleted {
		t.Fatalf("retrieved status=%q", retrieved.Status)
	}
	if !HasCodeInterpreterCall(retrieved) || !CallsMatchContainer(retrieved, container.ID) {
		t.Fatalf("code interpreter calls=%+v", retrieved.CodeInterpreterCalls)
	}

	if _, err := client.CancelResponse(ctx, "resp_live_001"); err != nil {
		t.Fatalf("CancelResponse: %v", err)
	}
	if !cancelCalled {
		t.Fatal("expected cancel endpoint call")
	}

	if err := client.DeleteContainerFile(ctx, container.ID, file.ID); err != nil {
		t.Fatalf("DeleteContainerFile: %v", err)
	}
	if !deleteFileCalled {
		t.Fatal("expected delete file call")
	}

	if err := client.DeleteContainer(ctx, container.ID); err != nil {
		t.Fatalf("DeleteContainer: %v", err)
	}
	if !deleteContainerCalled {
		t.Fatal("expected delete container call")
	}
}
