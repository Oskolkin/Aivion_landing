package unifiedanalysis

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// Run is the domain view of a unified_analysis ai_pipeline_runs row.
type Run struct {
	ID              int64
	SellerAccountID int64
	MetricDate      time.Time
	Mode            string
	Trigger         string
	Status          string

	DataPackSchemaVersion string
	DataPackHash          string
	PipelineVersion       string
	PromptVersion         string

	PackID                string
	PackHash              string
	PackStorageKey        string
	PackArchiveSHA256     string
	PackCompressedBytes   int64
	PackUncompressedBytes int64

	Provider                  string
	ProviderModel             string
	ProviderContainerID       string
	ProviderContainerFileID   string
	ProviderContainerFilePath string
	ProviderResponseID        string
	ProviderStatus            string
	ProviderStartedAt         *time.Time
	ProviderCompletedAt       *time.Time
	ProviderLatencyMs         int64
	ProviderAttemptCount      int32
	ProviderRequestID         string

	InputTokens          int64
	OutputTokens         int64
	TotalTokens          int64
	CodeInterpreterCalls int32
	CancelRequestedAt    *time.Time
	CleanupStatus        string
	CleanupStartedAt     *time.Time
	CleanupCompletedAt   *time.Time
	CleanupError         string
	ErrorCode            string
	ErrorMessage         string
	StartedAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// RunFromDB maps dbgen.AiPipelineRun to Run.
func RunFromDB(row dbgen.AiPipelineRun) *Run {
	return &Run{
		ID:                        row.ID,
		SellerAccountID:           row.SellerAccountID,
		MetricDate:                dateValue(row.MetricDate),
		Mode:                      row.Mode,
		Trigger:                   row.Trigger,
		Status:                    row.Status,
		DataPackSchemaVersion:     textValue(row.DataPackSchemaVersion),
		DataPackHash:              row.DataPackHash,
		PipelineVersion:           textValue(row.PipelineVersion),
		PromptVersion:             textValue(row.PromptVersion),
		PackID:                    textValue(row.PackID),
		PackHash:                  textValue(row.PackHash),
		PackStorageKey:            textValue(row.PackStorageKey),
		PackArchiveSHA256:         textValue(row.PackArchiveSha256),
		PackCompressedBytes:       int8Value(row.PackCompressedBytes),
		PackUncompressedBytes:     int8Value(row.PackUncompressedBytes),
		Provider:                  textValue(row.Provider),
		ProviderModel:             textValue(row.ProviderModel),
		ProviderContainerID:       textValue(row.ProviderContainerID),
		ProviderContainerFileID:   textValue(row.ProviderContainerFileID),
		ProviderContainerFilePath: textValue(row.ProviderContainerFilePath),
		ProviderResponseID:        textValue(row.ProviderResponseID),
		ProviderStatus:            textValue(row.ProviderStatus),
		ProviderStartedAt:         timePtr(row.ProviderStartedAt),
		ProviderCompletedAt:       timePtr(row.ProviderCompletedAt),
		ProviderLatencyMs:         int8Value(row.ProviderLatencyMs),
		ProviderAttemptCount:      row.ProviderAttemptCount,
		ProviderRequestID:         textValue(row.ProviderRequestID),
		InputTokens:               int8Value(row.InputTokens),
		OutputTokens:              int8Value(row.OutputTokens),
		TotalTokens:               int8Value(row.TotalTokens),
		CodeInterpreterCalls:      int4Value(row.CodeInterpreterCalls),
		CancelRequestedAt:         timePtr(row.CancelRequestedAt),
		CleanupStatus:             textValue(row.CleanupStatus),
		CleanupStartedAt:          timePtr(row.CleanupStartedAt),
		CleanupCompletedAt:        timePtr(row.CleanupCompletedAt),
		CleanupError:              textValue(row.CleanupError),
		ErrorCode:                 textValue(row.ErrorCode),
		ErrorMessage:              textValue(row.ErrorMessage),
		StartedAt:                 timePtr(row.StartedAt),
		CompletedAt:               timePtr(row.CompletedAt),
		CreatedAt:                 timeValue(row.CreatedAt),
		UpdatedAt:                 timeValue(row.UpdatedAt),
	}
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func toDate(t time.Time) pgtype.Date {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: t, Valid: true}
}

func dateValue(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func timeValue(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func timestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func timestamptzNow(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func int8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func int4Value(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func int8Ptr(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func int4Ptr(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}
