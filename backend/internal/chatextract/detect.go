package chatextract

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
)

// DetectFormat identifies a supported attachment format from name, MIME, and bytes.
func DetectFormat(filename, mime string, data []byte) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "txt", "md", "json", "csv", "tsv", "xlsx", "pdf", "docx", "pptx", "png", "jpg", "jpeg", "webp", "zip":
		return ext
	}
	mime = strings.ToLower(strings.Split(mime, ";")[0])
	switch mime {
	case "text/plain":
		return "txt"
	case "text/markdown":
		return "md"
	case "application/json":
		return "json"
	case "text/csv":
		return "csv"
	case "text/tab-separated-values":
		return "tsv"
	case "application/pdf":
		return "pdf"
	case "application/zip":
		return "zip"
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return "pdf"
	}
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return "zip"
	}
	switch http.DetectContentType(data) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	}
	return ""
}
