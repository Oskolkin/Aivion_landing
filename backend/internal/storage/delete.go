package storage

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func DeleteObject(ctx context.Context, client *Client, bucket, objectKey string) error {
	if client == nil || client.Raw == nil {
		return fmt.Errorf("s3 client is not configured")
	}
	if err := client.Raw.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object from s3: %w", err)
	}
	return nil
}
