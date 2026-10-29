package chatextract

import (
	"errors"
	"fmt"
)

// Extract normalizes a supported attachment into text and structured context.
func Extract(filename, mime string, data []byte) (*ExtractedAttachment, error) {
	if int64(len(data)) > configuredLimits().maxInputBytes {
		return nil, fmt.Errorf("attachment exceeds configured size limit")
	}
	format := DetectFormat(filename, mime, data)
	if format == "" {
		return nil, errors.New("unsupported attachment format")
	}
	var result *ExtractedAttachment
	var err error
	switch format {
	case "txt", "md", "json", "csv", "tsv":
		result, err = extractText(format, data)
	case "xlsx":
		result, err = extractXLSX(data)
	case "pdf":
		result, err = extractPDF(data)
	case "docx":
		result, err = extractDOCX(data)
	case "pptx":
		result, err = extractPPTX(data)
	case "png", "jpg", "jpeg", "webp":
		result, err = extractImage(format, data)
	case "zip":
		result, err = extractZIP(data)
	}
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", format, err)
	}
	result.Filename, result.Format = filename, format
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	return result, nil
}
