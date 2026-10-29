package chatv2

import (
	"strings"
	"unicode/utf8"
)

func deriveCompleteness(extraction, stored, mime, excerpt, extrErr string) Completeness {
	if stored != "" {
		return Completeness(stored)
	}
	mime = strings.ToLower(mime)
	if strings.HasPrefix(mime, "image/") {
		return CompleteVisionRequired
	}
	switch strings.ToLower(extraction) {
	case "failed", "unsupported":
		return CompleteUnreadable
	case "skipped":
		if strings.HasPrefix(mime, "image/") {
			return CompleteVisionRequired
		}
		return CompleteUnreadable
	case "ready":
		if strings.TrimSpace(excerpt) == "" {
			return CompletePartial
		}
		return CompleteComplete
	case "pending", "extracting":
		return CompletePartial
	default:
		if strings.TrimSpace(extrErr) != "" {
			return CompleteUnreadable
		}
		if strings.TrimSpace(excerpt) != "" {
			return CompleteComplete
		}
		return CompletePartial
	}
}

func attachmentWarning(c Completeness, warn, extrErr string) string {
	switch c {
	case CompletePartial:
		if warn != "" {
			return "Файл обработан частично. " + warn
		}
		return "Файл обработан частично; часть содержимого недоступна."
	case CompleteUnreadable:
		if extrErr != "" {
			return "Не удалось прочитать файл."
		}
		return "Файл не удалось прочитать."
	case CompleteVisionRequired:
		return "Изображение требует визуального разбора; текстовое извлечение недоступно."
	default:
		return ""
	}
}

func clipExcerpt(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || utf8.RuneCountInString(text) <= max {
		return text
	}
	return string([]rune(text)[:max]) + "…"
}
