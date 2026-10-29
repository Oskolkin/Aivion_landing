package rawpayloads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	queries *dbgen.Queries
	s3      *storage.Client
	bucket  string
}

func NewService(db *pgxpool.Pool, s3 *storage.Client, bucket string) *Service {
	return &Service{
		queries: dbgen.New(db),
		s3:      s3,
		bucket:  bucket,
	}
}

type SaveInput struct {
	SellerAccountID int64
	ImportJobID     int64
	Domain          string
	Source          string
	RequestKey      string
	Body            []byte
}

func (s *Service) Save(ctx context.Context, input SaveInput) (dbgen.RawPayload, error) {
	hash := sha256.Sum256(input.Body)
	payloadHash := hex.EncodeToString(hash[:])

	now := time.Now().UTC()
	objectKey := path.Join(
		"raw",
		"ozon",
		fmt.Sprintf("%d", input.SellerAccountID),
		input.Domain,
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		fmt.Sprintf("%d", input.ImportJobID),
		uuid.NewString()+".json",
	)

	if err := storage.UploadBytes(
		ctx,
		s.s3,
		s.bucket,
		objectKey,
		input.Body,
		"application/json",
	); err != nil {
		return dbgen.RawPayload{}, fmt.Errorf("upload raw payload: %w", err)
	}

	row, err := s.queries.CreateRawPayload(ctx, dbgen.CreateRawPayloadParams{
		SellerAccountID:  input.SellerAccountID,
		ImportJobID:      input.ImportJobID,
		Domain:           input.Domain,
		Source:           input.Source,
		RequestKey:       nullableText(input.RequestKey),
		StorageBucket:    s.bucket,
		StorageObjectKey: objectKey,
		PayloadHash:      payloadHash,
	})
	if err != nil {
		if delErr := storage.DeleteObject(ctx, s.s3, s.bucket, objectKey); delErr != nil {
			return dbgen.RawPayload{}, fmt.Errorf("create raw payload row: %w (cleanup failed: %v)", err, delErr)
		}
		return dbgen.RawPayload{}, fmt.Errorf("create raw payload row: %w", err)
	}

	return row, nil
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}
