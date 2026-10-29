package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

func DownloadBytes(ctx context.Context, client *Client, bucket, objectKey string) ([]byte, error) {
	obj, err := client.Raw.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object from s3: %w", err)
	}
	defer obj.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, obj); err != nil {
		return nil, fmt.Errorf("read object from s3: %w", err)
	}
	return buf.Bytes(), nil
}
