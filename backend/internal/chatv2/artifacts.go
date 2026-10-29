package chatv2

import (
	"context"
	"sync"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

type ArtifactStore interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type MemoryArtifactStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewMemoryArtifactStore() *MemoryArtifactStore {
	return &MemoryArtifactStore{data: map[string][]byte{}}
}

func (m *MemoryArtifactStore) Put(_ context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[key] = cp
	return nil
}

func (m *MemoryArtifactStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.data[key]
	if !ok {
		return nil, ErrAttachmentNotFound
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (m *MemoryArtifactStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemoryArtifactStore) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.data)
}

type FailArtifactStore struct {
	Inner     ArtifactStore
	PutErr    error
	GetErr    error
	DeleteErr error
}

func (f FailArtifactStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if f.PutErr != nil {
		return f.PutErr
	}
	if f.Inner != nil {
		return f.Inner.Put(ctx, key, data, contentType)
	}
	return nil
}

func (f FailArtifactStore) Get(ctx context.Context, key string) ([]byte, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	if f.Inner != nil {
		return f.Inner.Get(ctx, key)
	}
	return nil, ErrAttachmentNotFound
}

func (f FailArtifactStore) Delete(ctx context.Context, key string) error {
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	if f.Inner != nil {
		return f.Inner.Delete(ctx, key)
	}
	return nil
}

type S3ArtifactStore struct {
	Client *storage.Client
	Bucket string
}

func NewS3ArtifactStore(client *storage.Client, bucket string) *S3ArtifactStore {
	if client == nil || bucket == "" {
		return nil
	}
	return &S3ArtifactStore{Client: client, Bucket: bucket}
}

func (s *S3ArtifactStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if s == nil || s.Client == nil {
		return ErrExportFailed
	}
	return storage.UploadBytes(ctx, s.Client, s.Bucket, key, data, contentType)
}

func (s *S3ArtifactStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.Client == nil {
		return nil, ErrExportFailed
	}
	return storage.DownloadBytes(ctx, s.Client, s.Bucket, key)
}

func (s *S3ArtifactStore) Delete(ctx context.Context, key string) error {
	if s == nil || s.Client == nil {
		return ErrExportFailed
	}
	return storage.DeleteObject(ctx, s.Client, s.Bucket, key)
}
