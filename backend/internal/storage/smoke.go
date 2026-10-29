package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

func RunSmokeTest(ctx context.Context, client *Client) error {
	objectName := fmt.Sprintf("smoke-tests/%d.txt", time.Now().UnixNano())
	content := []byte("s3 smoke test ok")

	// upload
	_, err := client.Raw.PutObject(
		ctx,
		client.Config.BucketArtifacts,
		objectName,
		bytes.NewReader(content),
		int64(len(content)),
		minio.PutObjectOptions{
			ContentType: "text/plain",
		},
	)
	if err != nil {
		return fmt.Errorf("upload smoke object: %w", err)
	}

	// read back
	obj, err := client.Raw.GetObject(ctx, client.Config.BucketArtifacts, objectName, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("get smoke object: %w", err)
	}
	defer obj.Close()

	readBytes, err := io.ReadAll(obj)
	if err != nil {
		return fmt.Errorf("read smoke object: %w", err)
	}

	if string(readBytes) != string(content) {
		return fmt.Errorf("smoke object content mismatch")
	}

	// delete
	err = client.Raw.RemoveObject(ctx, client.Config.BucketArtifacts, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("delete smoke object: %w", err)
	}

	return nil
}
