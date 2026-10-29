package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type openAIFileUploadResponse struct {
	ID string `json:"id"`
}

func (c *OpenAIClient) UploadUserFile(ctx context.Context, fileName string, data []byte, mimeType string) (string, error) {
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return "", ErrOpenAIAPIKeyMissing
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty file payload")
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("purpose", "user_data"); err != nil {
		return "", fmt.Errorf("write purpose field: %w", err)
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("write file bytes: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/files",
		bytes.NewReader(body.Bytes()),
	)
	if err != nil {
		return "", fmt.Errorf("build openai files request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", openaix.WrapIfUnavailable(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read openai files response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var decoded openAIFileUploadResponse
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return "", fmt.Errorf("decode openai files response: %w", err)
		}
		if strings.TrimSpace(decoded.ID) == "" {
			return "", fmt.Errorf("%w: empty file id", ErrOpenAIProvider)
		}
		return decoded.ID, nil
	}
	errBody := strings.TrimSpace(string(respBody))
	if errBody == "" {
		errBody = "<empty>"
	}
	if err := openaix.WrapHTTPOutage(resp.StatusCode, errBody); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: status=%d body=%s", ErrOpenAIProvider, resp.StatusCode, errBody)
}

func (c *OpenAIClient) DeleteUserFile(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return ErrOpenAIAPIKeyMissing
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/files/"+fileID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("build openai delete file request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return openaix.WrapIfUnavailable(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	errBody := strings.TrimSpace(string(respBody))
	if err := openaix.WrapHTTPOutage(resp.StatusCode, errBody); err != nil {
		return err
	}
	return fmt.Errorf("%w: delete file status=%d body=%s", ErrOpenAIProvider, resp.StatusCode, errBody)
}

type fileUploadClient interface {
	UploadUserFile(ctx context.Context, fileName string, data []byte, mimeType string) (string, error)
}

type fileLifecycleClient interface {
	fileUploadClient
	DeleteUserFile(ctx context.Context, fileID string) error
}

func uploadAttachmentsToOpenAI(
	ctx context.Context,
	client fileUploadClient,
	storage InputAttachmentStorage,
	bucket string,
	items []ChatAttachment,
) ([]string, []string, error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	fileIDs := make([]string, 0, len(items))
	warnings := make([]string, 0)
	for _, item := range items {
		if !shouldUploadAttachmentToProvider(item) {
			fileIDs = append(fileIDs, "")
			warnings = append(warnings, "Файл "+item.FileName+" анализируется локально (без OpenAI Files API).")
			continue
		}
		data, err := storage.DownloadInputAttachment(ctx, bucket, item.StorageKey)
		if err != nil {
			return nil, warnings, err
		}
		if w := spreadsheetRowLimitWarning(item.FileName, data); w != "" {
			warnings = append(warnings, w)
		}
		fileID, err := client.UploadUserFile(ctx, item.FileName, data, item.MimeType)
		if err != nil {
			// Fall back to local extraction instead of failing the whole ask.
			warnings = append(warnings, "Не удалось загрузить "+item.FileName+" в OpenAI Files; используем локальное извлечение.")
			fileIDs = append(fileIDs, "")
			continue
		}
		fileIDs = append(fileIDs, fileID)
	}
	return fileIDs, warnings, nil
}

func shouldUploadAttachmentToProvider(item ChatAttachment) bool {
	format := strings.ToLower(strings.TrimSpace(item.Format))
	if format == "" {
		format = strings.ToLower(strings.TrimPrefix(filepath.Ext(item.FileName), "."))
	}
	switch format {
	case "png", "jpg", "jpeg", "webp", "gif":
		// Images are sent as Responses API input_image (base64), not Files API input_file.
		return false
	default:
		// Documents/archives: local extraction is authoritative for chat context.
		return false
	}
}
