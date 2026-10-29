package chat

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

const spreadsheetAIRowLimit = 1000

var (
	ErrAttachmentUnsupportedFormat = errors.New("unsupported attachment format")
	ErrAttachmentTooLarge          = errors.New("attachment exceeds size limit")
	ErrAttachmentTooMany           = errors.New("too many attachments")
	ErrAttachmentTotalTooLarge     = errors.New("total attachment size exceeds limit")
	ErrAttachmentNotFound          = errors.New("attachment not found")
	ErrAttachmentNotOwned          = errors.New("attachment not owned by seller")
	ErrAttachmentInvalidState      = errors.New("attachment is not ready for send")
)

func validateInputAttachmentFile(fileName, mimeType string, size int64) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		return fmt.Errorf("%w: missing extension", ErrAttachmentUnsupportedFormat)
	}
	if _, ok := allowedInputExtensions[ext]; !ok {
		if ext == ".xls" {
			return fmt.Errorf("%w: %s", ErrAttachmentUnsupportedFormat, unsupportedXLSMessage())
		}
		return fmt.Errorf("%w: %s", ErrAttachmentUnsupportedFormat, ext)
	}
	if size <= 0 {
		return fmt.Errorf("%w: empty file", ErrAttachmentUnsupportedFormat)
	}
	if size > chatAttachmentMaxFileBytes() {
		return fmt.Errorf("%w: max %d bytes", ErrAttachmentTooLarge, chatAttachmentMaxFileBytes())
	}
	declared := strings.ToLower(strings.TrimSpace(mimeType))
	if declared != "" && declared != "application/octet-stream" {
		if _, ok := allowedInputMIME[declared]; !ok {
			return fmt.Errorf("%w: mime %s", ErrAttachmentUnsupportedFormat, mimeType)
		}
	}
	return nil
}

func validateInputAttachmentBatch(items []ChatAttachment) error {
	if len(items) > chatAttachmentMaxFiles() {
		return fmt.Errorf("%w: max %d", ErrAttachmentTooMany, chatAttachmentMaxFiles())
	}
	var total int64
	for _, item := range items {
		total += item.FileSize
	}
	if total > chatAttachmentMaxTotalBytes() {
		return fmt.Errorf("%w: max %d bytes", ErrAttachmentTotalTooLarge, chatAttachmentMaxTotalBytes())
	}
	return nil
}

func formatFromFileName(fileName string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if ext == "" {
		return "file"
	}
	return ext
}

func spreadsheetRowLimitWarning(fileName string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".csv", ".tsv":
		if count, err := countCSVRows(data); err == nil && count > spreadsheetAIRowLimit {
			return spreadsheetLimitMessage()
		}
	case ".xlsx":
		if over, err := spreadsheetHasLargeSheet(data); err == nil && over {
			return spreadsheetLimitMessage()
		}
	}
	return ""
}

func spreadsheetLimitMessage() string {
	return "Файл содержит больше 1 000 строк на листе. Для прямого AI-анализа может быть использована только часть данных."
}

func countCSVRows(data []byte) (int, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	count := 0
	for {
		_, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func spreadsheetHasLargeSheet(data []byte) (bool, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	defer f.Close()
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		if len(rows) > spreadsheetAIRowLimit {
			return true, nil
		}
	}
	return false, nil
}
