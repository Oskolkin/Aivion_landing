package chat

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	warningCodeSpreadsheetTruncated = "spreadsheet_truncated_for_direct_ai_analysis"
	maxDisplayFileNameLen           = 120
)

func sanitizeDisplayFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == ".." {
		return "attachment"
	}
	var b strings.Builder
	for _, r := range base {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "attachment"
	}
	if len(out) > maxDisplayFileNameLen {
		ext := filepath.Ext(out)
		baseName := strings.TrimSuffix(out, ext)
		if len(baseName) > maxDisplayFileNameLen-len(ext) {
			baseName = baseName[:maxDisplayFileNameLen-len(ext)]
		}
		out = baseName + ext
	}
	return out
}

func validateInputAttachmentContent(fileName, mimeType string, data []byte) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if err := validateInputAttachmentFile(fileName, mimeType, int64(len(data))); err != nil {
		return err
	}
	detected := detectContentMIME(data)
	declared := strings.ToLower(strings.TrimSpace(mimeType))
	if declared != "" && declared != "application/octet-stream" {
		if detected != "" && !mimeCompatible(declared, detected, ext) {
			return fmt.Errorf("%w: content does not match declared type", ErrAttachmentUnsupportedFormat)
		}
	}
	switch ext {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return fmt.Errorf("%w: invalid pdf content", ErrAttachmentUnsupportedFormat)
		}
	case ".xlsx":
		if err := validateOfficeOpenXML(data, "xl/"); err != nil {
			return err
		}
	case ".docx":
		if err := validateOfficeOpenXML(data, "word/"); err != nil {
			return err
		}
	case ".pptx":
		if err := validateOfficeOpenXML(data, "ppt/"); err != nil {
			return err
		}
	case ".png":
		if detectContentMIME(data) != "image/png" {
			return fmt.Errorf("%w: invalid png content", ErrAttachmentUnsupportedFormat)
		}
	case ".jpg", ".jpeg":
		if detectContentMIME(data) != "image/jpeg" {
			return fmt.Errorf("%w: invalid jpeg content", ErrAttachmentUnsupportedFormat)
		}
	case ".webp":
		if detectContentMIME(data) != "image/webp" {
			return fmt.Errorf("%w: invalid webp content", ErrAttachmentUnsupportedFormat)
		}
	case ".zip":
		if detected != "application/zip" {
			return fmt.Errorf("%w: invalid zip content", ErrAttachmentUnsupportedFormat)
		}
	case ".csv", ".tsv", ".txt", ".md", ".json":
		if !isMostlyText(data) {
			return fmt.Errorf("%w: file must be plain text", ErrAttachmentUnsupportedFormat)
		}
	}
	if detected == "application/zip" && ext != ".xlsx" && ext != ".docx" && ext != ".pptx" && ext != ".zip" {
		return fmt.Errorf("%w: zip archives are not allowed", ErrAttachmentUnsupportedFormat)
	}
	if detected == "application/x-msdownload" || detected == "application/x-executable" {
		return fmt.Errorf("%w: executable content", ErrAttachmentUnsupportedFormat)
	}
	return nil
}

func detectContentMIME(data []byte) string {
	if len(data) >= 4 && bytes.HasPrefix(data, []byte("%PDF-")) {
		return "application/pdf"
	}
	if len(data) >= 2 && data[0] == 0x50 && data[1] == 0x4B {
		return "application/zip"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	if isMostlyText(data) {
		return "text/plain"
	}
	if len(data) >= 2 && data[0] == 0x4D && data[1] == 0x5A {
		return "application/x-msdownload"
	}
	return ""
}

func mimeCompatible(declared, detected, ext string) bool {
	if declared == detected {
		return true
	}
	switch ext {
	case ".csv", ".tsv", ".txt", ".md", ".json":
		return detected == "text/plain" || strings.HasPrefix(declared, "text/")
	case ".xlsx", ".docx", ".pptx":
		return detected == "application/zip" || strings.Contains(declared, "openxmlformats")
	case ".zip":
		return detected == "application/zip" && isZipDeclaredMIME(declared)
	case ".png":
		return detected == "image/png"
	case ".jpg", ".jpeg":
		return detected == "image/jpeg"
	case ".webp":
		return detected == "image/webp"
	case ".pdf":
		return detected == "application/pdf"
	}
	return false
}

func isZipDeclaredMIME(declared string) bool {
	switch strings.ToLower(strings.TrimSpace(declared)) {
	case "application/zip", "application/x-zip-compressed", "application/zip-compressed", "multipart/x-zip":
		return true
	default:
		return false
	}
}

func validateOfficeOpenXML(data []byte, requiredPrefix string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("%w: invalid office document", ErrAttachmentUnsupportedFormat)
	}
	hasContentTypes := false
	hasRequired := false
	for _, f := range zr.File {
		name := strings.ToLower(strings.TrimPrefix(f.Name, "/"))
		if name == "[content_types].xml" {
			hasContentTypes = true
		}
		if strings.HasPrefix(name, strings.ToLower(requiredPrefix)) {
			hasRequired = true
		}
	}
	if !hasContentTypes || !hasRequired {
		return fmt.Errorf("%w: invalid office document structure", ErrAttachmentUnsupportedFormat)
	}
	return nil
}

func isMostlyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, b := range sample {
		if b == 0 {
			return false
		}
	}
	return true
}

func unsupportedXLSMessage() string {
	return "Формат .xls пока не поддерживается. Сохраните файл как .xlsx или .csv."
}
