package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strings"
)

type openAIContainerFileResponse struct {
	ID          string          `json:"id"`
	ContainerID string          `json:"container_id"`
	Path        string          `json:"path"`
	Bytes       int64           `json:"bytes"`
	Source      string          `json:"source"`
	CreatedAt   json.RawMessage `json:"created_at"`
}

func parseContainerFile(raw openAIContainerFileResponse) *ContainerFile {
	return &ContainerFile{
		ID:          strings.TrimSpace(raw.ID),
		ContainerID: strings.TrimSpace(raw.ContainerID),
		Path:        strings.TrimSpace(raw.Path),
		Bytes:       raw.Bytes,
		Source:      strings.TrimSpace(raw.Source),
		CreatedAt:   firstTime(raw.CreatedAt),
	}
}

// UploadContainerFile uploads a raw ZIP into a container via multipart form.
func (c *OpenAIClient) UploadContainerFile(ctx context.Context, req UploadContainerFileRequest) (*ContainerFile, error) {
	containerID := strings.TrimSpace(req.ContainerID)
	if containerID == "" {
		return nil, newProviderError(CodeProviderFileUploadFailed, "upload container file: empty container id", 0, "", nil)
	}
	if len(req.Content) == 0 {
		return nil, newProviderError(CodeProviderFileUploadFailed, "upload container file: empty content", 0, "", nil)
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = "analysis_pack.zip"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := part.Write(req.Content); err != nil {
		return nil, fmt.Errorf("write multipart file bytes: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	respBody, _, err := c.doRaw(
		ctx,
		"POST",
		"/containers/"+containerID+"/files",
		writer.FormDataContentType(),
		bytes.NewReader(body.Bytes()),
		CodeProviderFileUploadFailed,
		"upload container file",
		false,
	)
	if err != nil {
		return nil, err
	}
	var decoded openAIContainerFileResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode upload container file response: %w", err)
	}
	file := parseContainerFile(decoded)
	if file.ID == "" {
		return nil, newProviderError(CodeProviderFileUploadFailed, "upload container file: empty id", 0, "", nil)
	}
	if file.ContainerID == "" {
		file.ContainerID = containerID
	}
	return file, nil
}

// DeleteContainerFile deletes a file from a container. 404 is treated as success.
func (c *OpenAIClient) DeleteContainerFile(ctx context.Context, containerID, fileID string) error {
	containerID = strings.TrimSpace(containerID)
	fileID = strings.TrimSpace(fileID)
	if containerID == "" || fileID == "" {
		return nil
	}
	_, _, err := c.doRaw(
		ctx,
		"DELETE",
		"/containers/"+containerID+"/files/"+fileID,
		"",
		nil,
		CodeProviderFileDeleteFailed,
		"delete container file",
		true,
	)
	if IsNotFound(err) {
		return nil
	}
	return err
}
