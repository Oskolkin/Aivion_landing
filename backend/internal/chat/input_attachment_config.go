package chat

import (
	"os"
	"strings"
)

const (
	AttachmentKindExport = "export"
	AttachmentKindInput  = "input"

	InputAttachmentStatusUploaded   = "uploaded"
	InputAttachmentStatusAttached   = "attached"
	InputAttachmentStatusProcessing = "processing"
	InputAttachmentStatusProcessed  = "processed"
	InputAttachmentStatusFailed     = "failed"
	InputAttachmentStatusDeleted    = "deleted"
)

var allowedInputExtensions = map[string]struct{}{
	".pdf": {}, ".xlsx": {}, ".csv": {}, ".docx": {}, ".pptx": {}, ".txt": {}, ".md": {}, ".json": {}, ".tsv": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".zip": {},
}

var allowedInputMIME = map[string]struct{}{
	"application/pdf": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
	"text/csv":                  {},
	"application/csv":           {},
	"text/plain":                {},
	"text/markdown":             {},
	"application/json":          {},
	"text/tab-separated-values": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
	"image/png":       {},
	"image/jpeg":      {},
	"image/webp":      {},
	"application/zip": {},
	// Browsers often declare ZIP with these aliases instead of application/zip.
	"application/x-zip-compressed": {},
	"application/zip-compressed":   {},
	"multipart/x-zip":              {},
}

func chatAttachmentMaxFileBytes() int64 {
	return int64(getEnvInt("CHAT_ATTACHMENT_MAX_FILE_BYTES", 20*1024*1024))
}

func chatAttachmentMaxTotalBytes() int64 {
	return int64(getEnvInt("CHAT_ATTACHMENT_MAX_TOTAL_BYTES", 30*1024*1024))
}

func chatAttachmentMaxFiles() int {
	return getEnvInt("CHAT_ATTACHMENT_MAX_FILES", 5)
}

func chatDraftAttachmentTTLHours() int {
	return getEnvInt("CHAT_DRAFT_ATTACHMENT_TTL_HOURS", 24)
}

func chatOpenAIFileTTLHours() int {
	return getEnvInt("CHAT_OPENAI_FILE_TTL_HOURS", 24)
}

func chatAttachmentCleanupEnabled() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("CHAT_ATTACHMENT_CLEANUP_ENABLED")))
	switch raw {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return true
	}
}

func ChatAttachmentCleanupEnabled() bool {
	return chatAttachmentCleanupEnabled()
}

func chatAttachmentCleanupBatchSize() int {
	v := getEnvInt("CHAT_ATTACHMENT_CLEANUP_BATCH_SIZE", 100)
	if v <= 0 {
		return 100
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var v int
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		v = v*10 + int(ch-'0')
	}
	if v <= 0 {
		return fallback
	}
	return v
}
