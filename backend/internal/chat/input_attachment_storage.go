package chat

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/minio/minio-go/v7"
)

type InputAttachmentStorage interface {
	UploadInputAttachment(ctx context.Context, bucket, objectKey string, data []byte, contentType string) error
	DownloadInputAttachment(ctx context.Context, bucket, objectKey string) ([]byte, error)
	DeleteInputAttachment(ctx context.Context, bucket, objectKey string) error
}

type S3InputAttachmentStorage struct {
	client *storage.Client
}

func NewS3InputAttachmentStorage(client *storage.Client) *S3InputAttachmentStorage {
	return &S3InputAttachmentStorage{client: client}
}

func (s *S3InputAttachmentStorage) UploadInputAttachment(
	ctx context.Context,
	bucket, objectKey string,
	data []byte,
	contentType string,
) error {
	if err := storage.UploadBytes(ctx, s.client, bucket, objectKey, data, contentType); err != nil {
		return fmt.Errorf("upload input attachment: %w", err)
	}
	return nil
}

func (s *S3InputAttachmentStorage) DownloadInputAttachment(ctx context.Context, bucket, objectKey string) ([]byte, error) {
	data, err := storage.DownloadBytes(ctx, s.client, bucket, objectKey)
	if err != nil {
		return nil, fmt.Errorf("download input attachment: %w", err)
	}
	return data, nil
}

func (s *S3InputAttachmentStorage) DeleteInputAttachment(ctx context.Context, bucket, objectKey string) error {
	if err := s.client.Raw.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete input attachment: %w", err)
	}
	return nil
}
