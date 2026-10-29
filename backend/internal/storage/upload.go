package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func UploadBytes(
	ctx context.Context,
	client *Client,
	bucket string,
	objectKey string,
	data []byte,
	contentType string,
) error {
	reader := bytes.NewReader(data)

	_, err := client.Raw.PutObject(
		ctx,
		bucket,
		objectKey,
		reader,
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return fmt.Errorf("upload bytes to s3: %w", err)
	}

	return nil
}
