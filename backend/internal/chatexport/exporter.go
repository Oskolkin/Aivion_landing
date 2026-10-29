package chatexport

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ExportRequest struct {
	SellerAccountID int64
	UserID          *int64
	SessionID       int64
	MessageID       int64
	ExportType      string
	Format          string
	DateFrom        time.Time
	DateTo          time.Time
	Filters         map[string]any
}

type ExportOutcome struct {
	AttachmentID uuid.UUID
	FileName     string
	MimeType     string
	FileSize     int64
	StorageKey   string
	RowCount     int
}

type Exporter struct {
	data    *DataService
	storage *ObjectStorage
}

func NewExporter(data *DataService, storage *ObjectStorage) *Exporter {
	return &Exporter{data: data, storage: storage}
}

func (e *Exporter) Export(ctx context.Context, req ExportRequest) (ExportOutcome, error) {
	if e == nil || e.data == nil {
		return ExportOutcome{}, fmt.Errorf("exporter is not configured")
	}
	spec, ok := LookupExportType(req.ExportType)
	if !ok || !spec.Implemented {
		return ExportOutcome{}, fmt.Errorf("export type is not allowed: %s", req.ExportType)
	}
	if !IsAllowedFormat(req.Format) {
		return ExportOutcome{}, fmt.Errorf("format is not allowed: %s", req.Format)
	}
	if daysInclusive(req.DateFrom, req.DateTo) > MaxExportDateRange {
		return ExportOutcome{}, fmt.Errorf("%s", ReasonDateRangeTooLarge)
	}

	table, err := e.data.BuildTable(ctx, req.SellerAccountID, req.ExportType, req.DateFrom, req.DateTo)
	if err != nil {
		return ExportOutcome{}, err
	}

	fileName := spec.FileNameStem + fileExtension(req.Format)
	payload, mime, err := WriteFile(req.Format, spec.SheetName, table)
	if err != nil {
		return ExportOutcome{}, err
	}

	attachmentID := uuid.New()
	var storageKey string
	if e.storage != nil {
		storageKey, err = e.storage.UploadExport(ctx, req.SellerAccountID, attachmentID, fileName, payload, mime)
		if err != nil {
			return ExportOutcome{}, wrapStorageError(err)
		}
	} else {
		return ExportOutcome{}, ErrExportStorageUnavailable
	}

	return ExportOutcome{
		AttachmentID: attachmentID,
		FileName:     fileName,
		MimeType:     mime,
		FileSize:     int64(len(payload)),
		StorageKey:   storageKey,
		RowCount:     len(table.Rows),
	}, nil
}

func daysInclusive(from, to time.Time) int {
	from = from.UTC().Truncate(24 * time.Hour)
	to = to.UTC().Truncate(24 * time.Hour)
	if to.Before(from) {
		return 0
	}
	return int(to.Sub(from).Hours()/24) + 1
}
