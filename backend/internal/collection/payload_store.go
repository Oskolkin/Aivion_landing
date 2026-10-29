package collection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/google/uuid"
)

// ObjectStorage is deliberately small so payload persistence can be tested
// independently while production reuses the existing MinIO client.
type ObjectStorage interface {
	Put(context.Context, string, string, []byte, string) error
	Get(context.Context, string, string) ([]byte, error)
}
type minioStorage struct{ client *storage.Client }

func (m minioStorage) Put(ctx context.Context, bucket, key string, body []byte, contentType string) error {
	return storage.UploadBytes(ctx, m.client, bucket, key, body, contentType)
}
func (m minioStorage) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	return storage.DownloadBytes(ctx, m.client, bucket, key)
}

type SourcePayloadStore struct {
	objects ObjectStorage
	bucket  string
	now     func() time.Time
}

func NewSourcePayloadStore(client *storage.Client, bucket string) *SourcePayloadStore {
	return NewSourcePayloadStoreWithObjects(minioStorage{client: client}, bucket)
}
func NewSourcePayloadStoreWithObjects(objects ObjectStorage, bucket string) *SourcePayloadStore {
	return &SourcePayloadStore{objects: objects, bucket: bucket, now: func() time.Time { return time.Now().UTC() }}
}

type StoredPayload struct {
	Bucket, ObjectKey, SHA256 string
	Size                      int64
}

func (s *SourcePayloadStore) Put(ctx context.Context, sellerID int64, domain CollectionDomain, body []byte, contentType string) (StoredPayload, error) {
	if s == nil || s.objects == nil || s.bucket == "" {
		return StoredPayload{}, fmt.Errorf("source payload store is not configured")
	}
	sum := sha256.Sum256(body)
	now := s.now()
	key := path.Join("collection", "ozon", fmt.Sprintf("seller-%d", sellerID), string(domain), now.Format("2006/01/02"), uuid.NewString()+".bin")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.objects.Put(ctx, s.bucket, key, body, contentType); err != nil {
		return StoredPayload{}, fmt.Errorf("put source payload object: %w", err)
	}
	return StoredPayload{Bucket: s.bucket, ObjectKey: key, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}, nil
}
func (s *SourcePayloadStore) Get(ctx context.Context, bucket, key string) ([]byte, error) {
	if s == nil || s.objects == nil {
		return nil, fmt.Errorf("source payload store is not configured")
	}
	return s.objects.Get(ctx, bucket, key)
}
func (s *SourcePayloadStore) Verify(ctx context.Context, payload SourcePayload) (bool, error) {
	body, err := s.Get(ctx, payload.Bucket, payload.ObjectKey)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]) == payload.SHA256, nil
}
