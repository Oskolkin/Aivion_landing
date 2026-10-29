package storage

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	UseSSL          bool
	BucketRaw       string
	BucketExports   string
	BucketArtifacts string
}

type Client struct {
	Raw    *minio.Client
	Config S3Config
}

func New(ctx context.Context, cfg S3Config) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	// Простейшая проверка — запрос к bucket location
	for _, bucket := range []string{cfg.BucketRaw, cfg.BucketExports, cfg.BucketArtifacts} {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return nil, fmt.Errorf("check bucket %s: %w", bucket, err)
		}
		if !exists {
			return nil, fmt.Errorf("bucket %s does not exist", bucket)
		}
	}

	return &Client{
		Raw:    client,
		Config: cfg,
	}, nil
}
