package chatextract

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	pdfStream     = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfTextObject = regexp.MustCompile(`(?s)BT(.*?)ET`)
	pdfLiteral    = regexp.MustCompile(`\((?:\\.|[^\\)])*\)`)
	pdfHexLiteral = regexp.MustCompile(`<([0-9A-Fa-f\s]+)>`)
	pdfPage       = regexp.MustCompile(`/Type\s*/Page[^s]`)
)

// extractPDF decompresses FlateDecode streams when present, then pulls text from BT/ET operators.
func extractPDF(data []byte) (*ExtractedAttachment, error) {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return nil, fmt.Errorf("invalid pdf header")
	}
	expanded := expandPDFStreams(data)
	sections := make([]ExtractedSection, 0)
	pageIdx := 0
	for _, object := range pdfTextObject.FindAllSubmatch(expanded, -1) {
		if !looksLikePDFTextObject(object[1]) {
			continue
		}
		var text strings.Builder
		for _, literal := range pdfLiteral.FindAll(object[1], -1) {
			text.WriteString(normalizePDFDecodedText(pdfUnescape(literal[1 : len(literal)-1])))
			text.WriteByte(' ')
		}
		for _, hex := range pdfHexLiteral.FindAllSubmatch(object[1], -1) {
			decoded := pdfHexDecode(string(hex[1]))
			if decoded == "" {
				continue
			}
			text.WriteString(normalizePDFDecodedText(decoded))
			text.WriteByte(' ')
		}
		if value := cleanPDFTextSegment(text.String()); value != "" {
			pageIdx++
			sections = append(sections, ExtractedSection{Title: "Page " + strconv.Itoa(pageIdx), Text: value})
		}
	}
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.Text)
	}
	pageCount := len(pdfPage.FindAll(data, -1))
	if pageCount == 0 {
		pageCount = len(sections)
	}
	return &ExtractedAttachment{
		PlainText: strings.Join(parts, "\n\n"),
		Sections:  sections,
		Metadata: map[string]any{
			"pages":          pageCount,
			"text_segments":  len(sections),
			"limited_parser": true,
		},
	}, nil
}

func expandPDFStreams(data []byte) []byte {
	var out bytes.Buffer
	// Prefer decompressed content streams only. Searching the raw PDF body
	// pulls false BT/ET matches out of font programs and object dictionaries.
	matches := pdfStream.FindAllSubmatchIndex(data, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		headerStart := m[0]
		lookBehind := data[max(0, headerStart-200):headerStart]
		payload := data[m[2]:m[3]]
		// Skip font/image/ToUnicode binary streams — they create false BT/ET matches
		// and dump CID maps into PlainText.
		if bytes.Contains(lookBehind, []byte("/FontFile")) ||
			bytes.Contains(lookBehind, []byte("/CIDToGIDMap")) ||
			bytes.Contains(lookBehind, []byte("/ToUnicode")) ||
			bytes.Contains(lookBehind, []byte("/FontDescriptor")) ||
			bytes.Contains(lookBehind, []byte("/Image")) ||
			bytes.Contains(lookBehind, []byte("/XObject")) {
			continue
		}
		if bytes.Contains(lookBehind, []byte("/FlateDecode")) || bytes.Contains(lookBehind, []byte("/Fl")) {
			decoded, err := inflatePDF(payload)
			if err == nil && len(decoded) > 0 && looksLikePDFContentStream(decoded) {
				out.Write(decoded)
				out.WriteByte('\n')
			}
		} else if looksLikePDFContentStream(payload) {
			out.Write(payload)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func inflatePDF(payload []byte) ([]byte, error) {
	payload = bytes.TrimLeft(payload, "\r\n")
	r, err := zlib.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 8<<20))
}

func pdfUnescape(value []byte) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			switch value[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '(', ')', '\\':
				b.WriteByte(value[i+1])
			default:
				// octal escape \ddd
				if value[i+1] >= '0' && value[i+1] <= '7' {
					oct := 0
					j := i + 1
					for n := 0; n < 3 && j < len(value) && value[j] >= '0' && value[j] <= '7'; n++ {
						oct = oct*8 + int(value[j]-'0')
						j++
					}
					b.WriteByte(byte(oct))
					i = j - 1
					continue
				}
				b.WriteByte(value[i+1])
			}
			i++
			continue
		}
		b.WriteByte(value[i])
	}
	return b.String()
}

func pdfHexDecode(hex string) string {
	hex = strings.ReplaceAll(hex, " ", "")
	if len(hex)%2 == 1 {
		hex += "0"
	}
	out := make([]byte, 0, len(hex)/2)
	for i := 0; i+1 < len(hex); i += 2 {
		v, err := strconv.ParseUint(hex[i:i+2], 16, 8)
		if err != nil {
			continue
		}
		out = append(out, byte(v))
	}
	if len(out) >= 2 && out[0] == 0xFE && out[1] == 0xFF {
		return decodeUTF16Bytes(out[2:], true)
	}
	if len(out) >= 2 && out[0] == 0xFF && out[1] == 0xFE {
		return decodeUTF16Bytes(out[2:], false)
	}
	// gofpdf Unicode mode often emits UTF-16BE without BOM inside <...>.
	if looksLikeUTF16BE(out) {
		return decodeUTF16Bytes(out, true)
	}
	if looksLikeUTF16LE(out) {
		return decodeUTF16Bytes(out, false)
	}
	return string(out)
}

func looksLikeUTF16BE(b []byte) bool {
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	nullHigh := 0
	for i := 0; i+1 < len(b); i += 2 {
		if b[i] == 0 && b[i+1] != 0 {
			nullHigh++
		}
	}
	return nullHigh*2 >= len(b)/2
}

func looksLikeUTF16LE(b []byte) bool {
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	nullLow := 0
	for i := 0; i+1 < len(b); i += 2 {
		if b[i] != 0 && b[i+1] == 0 {
			nullLow++
		}
	}
	return nullLow*2 >= len(b)/2
}

func decodeUTF16Bytes(b []byte, bigEndian bool) string {
	if len(b) < 2 {
		return ""
	}
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u16 = append(u16, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u16 = append(u16, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	return string(utf16.Decode(u16))
}

func normalizePDFDecodedText(s string) string {
	if s == "" {
		return ""
	}
	// Repair accidental UTF-16BE interpreted as a Go string of raw bytes.
	raw := []byte(s)
	if looksLikeUTF16BE(raw) {
		return decodeUTF16Bytes(raw, true)
	}
	if looksLikeUTF16LE(raw) {
		return decodeUTF16Bytes(raw, false)
	}
	return s
}

func looksLikePDFContentStream(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	// Content streams are mostly ASCII operators; font programs are dense binary.
	printable := 0
	sample := b
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	for _, c := range sample {
		if c == 9 || c == 10 || c == 13 || (c >= 32 && c < 127) {
			printable++
		}
	}
	ratio := float64(printable) / float64(len(sample))
	if ratio < 0.55 {
		return false
	}
	return bytes.Contains(sample, []byte("BT")) || bytes.Contains(sample, []byte("Tj")) ||
		bytes.Contains(sample, []byte("TJ")) || bytes.Contains(sample, []byte("Td")) ||
		bytes.Contains(sample, []byte("Tm"))
}

func looksLikePDFTextObject(b []byte) bool {
	if len(b) == 0 || len(b) > 200_000 {
		return false
	}
	printable := 0
	for _, c := range b {
		if c == 9 || c == 10 || c == 13 || (c >= 32 && c < 127) || c >= 0xC0 {
			printable++
		}
	}
	return float64(printable)/float64(len(b)) >= 0.5
}

func cleanPDFTextSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(' ')
		case unicode.IsPrint(r) || unicode.IsSpace(r):
			b.WriteRune(r)
		case r == 0:
			// drop NULs leftover from bad decoding
		default:
			// drop other controls / unpaired surrogates
		}
	}
	cleaned := strings.Join(strings.Fields(b.String()), " ")
	if cleaned == "" {
		return ""
	}
	lower := strings.ToLower(cleaned)
	if strings.Contains(lower, "endstream") || strings.Contains(lower, "endobj") ||
		strings.Contains(lower, "begincmap") || strings.Contains(lower, "/type /font") ||
		strings.Contains(lower, "cidinit") || strings.Contains(cleaned, "/W [") {
		return ""
	}
	if len(cleaned) > 8000 {
		return ""
	}
	// Reject mostly-garbage segments (e.g. accidental binary BT/ET matches).
	lettersDigits := 0
	for _, r := range cleaned {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			lettersDigits++
		}
	}
	if lettersDigits < 3 {
		return ""
	}
	if !utf8.ValidString(cleaned) {
		return ""
	}
	return cleaned
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
