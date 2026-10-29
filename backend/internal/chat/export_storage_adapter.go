package chat

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

type S3ExportStorage struct {
	client *storage.Client
	bucket string
}

func NewS3ExportStorage(client *storage.Client, bucket string) *S3ExportStorage {
	return &S3ExportStorage{client: client, bucket: bucket}
}

func (s *S3ExportStorage) DownloadExport(ctx context.Context, storageKey string) ([]byte, error) {
	data, err := storage.DownloadBytes(ctx, s.client, s.bucket, storageKey)
	if err != nil {
		return nil, chatexport.WrapStorageDownloadError(err)
	}
	return data, nil
}
