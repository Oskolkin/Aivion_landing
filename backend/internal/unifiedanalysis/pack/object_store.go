package pack

import (
	"context"
	"fmt"
	"io"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/minio/minio-go/v7"
)

// S3PackObjectStore downloads pack ZIPs previously stored in the artifacts bucket.
type S3PackObjectStore struct {
	client *storage.Client
}

// NewS3PackObjectStore wraps storage.Client for pack ZIP retrieval.
func NewS3PackObjectStore(client *storage.Client) *S3PackObjectStore {
	if client == nil {
		panic("unifiedanalysis/pack: storage client must not be nil")
	}
	return &S3PackObjectStore{client: client}
}

// Get downloads objectKey from the artifacts bucket and returns the raw bytes.
func (s *S3PackObjectStore) Get(ctx context.Context, objectKey string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, ErrStorageNotConfigured
	}
	obj, err := s.client.Raw.GetObject(ctx, s.client.Config.BucketArtifacts, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("unifiedanalysis/pack: get object: %w", err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("unifiedanalysis/pack: read object: %w", err)
	}
	return data, nil
}
