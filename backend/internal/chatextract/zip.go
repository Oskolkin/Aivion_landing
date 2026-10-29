package chatextract

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

func extractZIP(data []byte) (*ExtractedAttachment, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	limits := configuredLimits()
	if len(reader.File) > limits.zipMaxFiles {
		return nil, fmt.Errorf("zip contains more than %d files", limits.zipMaxFiles)
	}
	var declaredTotal int64
	var actualTotal int64
	var text strings.Builder
	var sections []ExtractedSection
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if unsafeZipPath(file.Name) {
			return nil, fmt.Errorf("unsafe zip path %q", file.Name)
		}
		if file.Flags&0x1 != 0 {
			return nil, fmt.Errorf("encrypted zip entry %q is not allowed", file.Name)
		}
		if exceedsCompressionRatio(file.CompressedSize64, file.UncompressedSize64, uint64(limits.zipMaxCompressionRatio)) {
			return nil, fmt.Errorf("zip entry %q exceeds compression ratio limit", file.Name)
		}
		if file.UncompressedSize64 > uint64(limits.zipMaxUncompressed) {
			return nil, errors.New("zip entry exceeds uncompressed size limit")
		}
		declaredTotal += int64(file.UncompressedSize64)
		if declaredTotal > limits.zipMaxUncompressed {
			return nil, errors.New("zip exceeds uncompressed size limit")
		}
		format := DetectFormat(file.Name, "", nil)
		if format == "zip" {
			return nil, errors.New("nested zip archives are not allowed")
		}
		if format == "" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		remaining := limits.zipMaxUncompressed - actualTotal
		content, readErr := io.ReadAll(io.LimitReader(stream, remaining+1))
		_ = stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		actualTotal += int64(len(content))
		if actualTotal > limits.zipMaxUncompressed {
			return nil, errors.New("zip exceeds uncompressed size limit")
		}
		attachment, err := Extract(file.Name, "", content)
		if err != nil {
			return nil, err
		}
		if attachment.PlainText == "" {
			continue
		}
		value := fmt.Sprintf("[%s]\n%s", file.Name, attachment.PlainText)
		sections = append(sections, ExtractedSection{Title: file.Name, Text: attachment.PlainText})
		if text.Len() > 0 {
			text.WriteString("\n\n")
		}
		text.WriteString(value)
	}
	return &ExtractedAttachment{PlainText: text.String(), Sections: sections, Metadata: map[string]any{"files": len(sections)}}, nil
}

func exceedsCompressionRatio(compressed, uncompressed, maxRatio uint64) bool {
	if uncompressed == 0 {
		return false
	}
	if compressed == 0 {
		return true
	}
	return uncompressed/compressed > maxRatio || (uncompressed/compressed == maxRatio && uncompressed%compressed != 0)
}

func unsafeZipPath(name string) bool {
	normalized := strings.ReplaceAll(name, `\`, "/")
	cleaned := path.Clean(normalized)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) || filepath.IsAbs(name) {
		return true
	}
	// filepath.VolumeName is OS-specific; detect Windows drive/UNC paths portably
	// so Linux CI rejects the same zip slip shapes Windows would.
	if filepath.VolumeName(name) != "" {
		return true
	}
	if len(name) >= 2 {
		drive := name[0]
		if (drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z') {
			if name[1] == ':' {
				return true
			}
		}
	}
	if strings.HasPrefix(normalized, "//") {
		return true
	}
	return false
}
