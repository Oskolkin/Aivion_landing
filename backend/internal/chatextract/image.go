package chatextract

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	_ "golang.org/x/image/webp"
)

func extractImage(format string, data []byte) (*ExtractedAttachment, error) {
	config, detected, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if format == "jpg" {
		format = "jpeg"
	}
	if detected != "" {
		format = detected
	}
	meta := map[string]any{
		"vision_required": true,
		"width":           config.Width,
		"height":          config.Height,
		"description":     "Image decoded locally; OCR and visual interpretation require a vision provider. Do not invent values.",
	}
	return &ExtractedAttachment{
		PlainText: fmt.Sprintf("[image %dx%d %s] vision_required=true — use the attached image pixels; do not guess unseen values.", config.Width, config.Height, format),
		Image:     &ExtractedImageMeta{Width: config.Width, Height: config.Height, Format: format},
		Metadata:  meta,
	}, nil
}

func extractEmbeddedControlMarker(data []byte) string {
	if marker := pngTextChunk(data, "Control"); marker != "" {
		return marker
	}
	if marker := jpegCOMMarker(data); marker != "" {
		return marker
	}
	if marker := webpXMPControl(data); marker != "" {
		return marker
	}
	return ""
}

func pngTextChunk(data []byte, key string) string {
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return ""
	}
	offset := 8
	for offset+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		ctype := string(data[offset+4 : offset+8])
		start := offset + 8
		end := start + length
		if end+4 > len(data) {
			return ""
		}
		chunk := data[start:end]
		if ctype == "tEXt" {
			parts := bytes.SplitN(chunk, []byte{0}, 2)
			if len(parts) == 2 && string(parts[0]) == key {
				return string(parts[1])
			}
		}
		offset = end + 4
		if ctype == "IEND" {
			break
		}
	}
	return ""
}

func jpegCOMMarker(data []byte) string {
	if !bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		return ""
	}
	i := 2
	for i+3 < len(data) {
		if data[i] != 0xFF {
			return ""
		}
		marker := data[i+1]
		if marker == 0xD9 || marker == 0xDA {
			return ""
		}
		if i+3 >= len(data) {
			return ""
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(data) {
			return ""
		}
		payload := data[i+4 : i+2+segLen]
		if marker == 0xFE {
			text := strings.TrimSpace(string(payload))
			if strings.Contains(text, "_CONTROL_") {
				return text
			}
		}
		i += 2 + segLen
	}
	return ""
}

func webpXMPControl(data []byte) string {
	if !bytes.HasPrefix(data, []byte("RIFF")) || len(data) < 16 || !bytes.Equal(data[8:12], []byte("WEBP")) {
		return ""
	}
	idx := bytes.Index(data, []byte("CONTROL_"))
	if idx < 0 {
		return ""
	}
	end := idx
	for end < len(data) && data[end] >= 32 && data[end] < 127 {
		end++
	}
	candidate := string(data[idx:end])
	if strings.Contains(candidate, "CONTROL_") {
		start := idx
		for start > 0 {
			prev := data[start-1]
			if (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || prev == '_' {
				start--
				continue
			}
			break
		}
		full := strings.Trim(string(data[start:end]), "\r\n\t ")
		if strings.Contains(full, "CONTROL_") {
			return full
		}
		return candidate
	}
	return ""
}

// AppendPNGTExt returns PNG bytes with an extra tEXt chunk before IEND.
func AppendPNGTExt(pngData []byte, key, value string) ([]byte, error) {
	if !bytes.HasPrefix(pngData, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return nil, fmt.Errorf("not a png")
	}
	iend := bytes.Index(pngData, []byte("IEND"))
	if iend < 4 {
		return nil, fmt.Errorf("png missing IEND")
	}
	insertAt := iend - 4
	payload := append(append([]byte(key), 0), []byte(value)...)
	chunkType := []byte("tEXt")
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(payload)))
	chunk.Write(chunkType)
	chunk.Write(payload)
	crc := crc32.ChecksumIEEE(append(chunkType, payload...))
	_ = binary.Write(&chunk, binary.BigEndian, crc)
	out := make([]byte, 0, len(pngData)+chunk.Len())
	out = append(out, pngData[:insertAt]...)
	out = append(out, chunk.Bytes()...)
	out = append(out, pngData[insertAt:]...)
	return out, nil
}
