package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type openAIContainerResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	MemoryLimit  string          `json:"memory_limit"`
	CreatedAt    json.RawMessage `json:"created_at"`
	LastActiveAt json.RawMessage `json:"last_active_at"`
	ExpiresAt    json.RawMessage `json:"expires_at"`
}

type createContainerBody struct {
	Name        string `json:"name"`
	MemoryLimit string `json:"memory_limit"`
}

func parseContainer(raw openAIContainerResponse) *Container {
	return &Container{
		ID:           strings.TrimSpace(raw.ID),
		Name:         strings.TrimSpace(raw.Name),
		Status:       strings.TrimSpace(raw.Status),
		MemoryLimit:  strings.TrimSpace(raw.MemoryLimit),
		CreatedAt:    firstTime(raw.CreatedAt),
		LastActiveAt: optionalUnixTime(raw.LastActiveAt),
		ExpiresAt:    optionalUnixTime(raw.ExpiresAt),
	}
}

func firstTime(raw json.RawMessage) (t time.Time) {
	if parsed, ok := parseUnixTime(raw); ok {
		return parsed
	}
	return time.Time{}
}

// CreateContainer creates an explicit Code Interpreter container.
func (c *OpenAIClient) CreateContainer(ctx context.Context, req CreateContainerRequest) (*Container, error) {
	body := createContainerBody{
		Name:        strings.TrimSpace(req.Name),
		MemoryLimit: strings.TrimSpace(req.MemoryLimit),
	}
	respBody, _, err := c.doJSON(ctx, doJSONOptions{
		method:    "POST",
		path:      "/containers",
		body:      body,
		errorCode: CodeProviderContainerCreateFailed,
		operation: "create container",
	})
	if err != nil {
		return nil, err
	}
	var decoded openAIContainerResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode create container response: %w", err)
	}
	container := parseContainer(decoded)
	if container.ID == "" {
		return nil, newProviderError(CodeProviderContainerCreateFailed, "create container: empty id", 0, "", nil)
	}
	return container, nil
}

// RetrieveContainer fetches container metadata.
func (c *OpenAIClient) RetrieveContainer(ctx context.Context, containerID string) (*Container, error) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return nil, newProviderError(CodeProviderContainerRetrieveFailed, "retrieve container: empty id", 0, "", nil)
	}
	respBody, _, err := c.doJSON(ctx, doJSONOptions{
		method:    "GET",
		path:      "/containers/" + containerID,
		retrySafe: true,
		errorCode: CodeProviderContainerRetrieveFailed,
		operation: "retrieve container",
	})
	if err != nil {
		return nil, err
	}
	var decoded openAIContainerResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode retrieve container response: %w", err)
	}
	container := parseContainer(decoded)
	if container.ID == "" {
		return nil, newProviderError(CodeProviderContainerRetrieveFailed, "retrieve container: empty id", 0, "", nil)
	}
	return container, nil
}

// DeleteContainer deletes a container. 404 is treated as success.
func (c *OpenAIClient) DeleteContainer(ctx context.Context, containerID string) error {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return nil
	}
	_, _, err := c.doJSON(ctx, doJSONOptions{
		method:    "DELETE",
		path:      "/containers/" + containerID,
		retrySafe: true,
		errorCode: CodeProviderContainerDeleteFailed,
		operation: "delete container",
	})
	if IsNotFound(err) {
		return nil
	}
	return err
}
