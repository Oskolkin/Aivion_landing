package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
)

// ListObjectKeysByPrefix lists object keys under an exact prefix in a bucket.
// Prefix should include a trailing "/" when listing a seller-scoped tree so that
// seller-1 does not match seller-10 / seller-11.
func ListObjectKeysByPrefix(ctx context.Context, client *Client, bucket, prefix string) ([]string, error) {
	if client == nil || client.Raw == nil {
		return nil, fmt.Errorf("s3 client is not configured")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	keys := make([]string, 0)
	for obj := range client.Raw.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects prefix %q: %w", prefix, obj.Err)
		}
		if obj.Key == "" {
			continue
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}
