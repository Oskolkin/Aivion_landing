package chatexport

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/google/uuid"
)

type ObjectStorage struct {
	client *storage.Client
	bucket string
}

func NewObjectStorage(client *storage.Client, bucket string) *ObjectStorage {
	return &ObjectStorage{client: client, bucket: bucket}
}

func (s *ObjectStorage) UploadExport(
	ctx context.Context,
	sellerAccountID int64,
	attachmentID uuid.UUID,
	fileName string,
	data []byte,
	contentType string,
) (storageKey string, err error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("export storage is not configured")
	}
	safeName := sanitizeFileName(fileName)
	key := path.Join(
		"chat-exports",
		fmt.Sprintf("%d", sellerAccountID),
		attachmentID.String(),
		safeName,
	)
	if err := storage.UploadBytes(ctx, s.client, s.bucket, key, data, contentType); err != nil {
		return "", wrapStorageError(err)
	}
	return key, nil
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" {
		return "export.dat"
	}
	return name
}
