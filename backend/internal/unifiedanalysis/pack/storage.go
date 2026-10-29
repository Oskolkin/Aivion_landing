package pack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/minio/minio-go/v7"
)

// PackStorage persists a built pack ZIP to object storage and returns its object key.
type PackStorage interface {
	Put(ctx context.Context, result *BuildResult) (objectKey string, err error)
}

// S3PackStorage stores pack ZIPs in the artifacts bucket of a shared storage.Client.
type S3PackStorage struct {
	client *storage.Client
}

// NewS3PackStorage wraps a storage.Client for Unified Analysis Pack persistence.
func NewS3PackStorage(client *storage.Client) *S3PackStorage {
	if client == nil {
		panic("unifiedanalysis/pack: storage client must not be nil")
	}
	return &S3PackStorage{client: client}
}

// PackObjectKey returns the frozen v1 object key for a pack:
// ai-analysis-packs/seller-<id>/<metric_date>/<pack_hash>/analysis_pack.zip
func PackObjectKey(sellerAccountID int64, metricDate, packHash string) string {
	return fmt.Sprintf("ai-analysis-packs/seller-%d/%s/%s/analysis_pack.zip", sellerAccountID, metricDate, packHash)
}

// Put uploads result.ZipBytes to the artifacts bucket, tagging the object with
// pack identity metadata, and returns the object key it was stored under.
func (s *S3PackStorage) Put(ctx context.Context, result *BuildResult) (string, error) {
	if s == nil || s.client == nil {
		return "", ErrStorageNotConfigured
	}
	if result == nil {
		return "", ErrNilSnapshot
	}
	key := PackObjectKey(result.SellerAccountID, result.MetricDate, result.PackHash)
	archiveSHA := sha256.Sum256(result.ZipBytes)
	_, err := s.client.Raw.PutObject(
		ctx,
		s.client.Config.BucketArtifacts,
		key,
		bytes.NewReader(result.ZipBytes),
		int64(len(result.ZipBytes)),
		minio.PutObjectOptions{
			ContentType: "application/zip",
			UserMetadata: map[string]string{
				"pack-id":            result.PackID,
				"pack-hash":          result.PackHash,
				"schema-version":     result.SchemaVersion,
				"seller-account-id":  fmt.Sprintf("%d", result.SellerAccountID),
				"metric-date":        result.MetricDate,
				"archive-sha256":     hex.EncodeToString(archiveSHA[:]),
				"compressed-bytes":   fmt.Sprintf("%d", len(result.ZipBytes)),
				"uncompressed-bytes": fmt.Sprintf("%d", result.Validation.UncompressedBytes),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("unifiedanalysis/pack: upload zip: %w", err)
	}
	return key, nil
}
