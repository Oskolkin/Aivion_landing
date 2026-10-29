// Package lineendings provides canonical LF normalization for immutable
// Unified Analysis prompt/schema identity hashing.
package lineendings

import "bytes"

// NormalizeLineEndingsBytes rewrites CRLF and bare CR to LF.
// It does not alter spaces, JSON structure, encoding, or other characters.
func NormalizeLineEndingsBytes(input []byte) []byte {
	if len(input) == 0 {
		return input
	}
	// Fast path: already LF-only (no CR).
	if !bytes.Contains(input, []byte{'\r'}) {
		out := make([]byte, len(input))
		copy(out, input)
		return out
	}
	out := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == '\r' {
			// CRLF → LF; bare CR → LF.
			if i+1 < len(input) && input[i+1] == '\n' {
				i++
			}
			out = append(out, '\n')
			continue
		}
		out = append(out, c)
	}
	return out
}

// NormalizeLineEndings rewrites CRLF and bare CR to LF for text payloads.
func NormalizeLineEndings(input string) string {
	if input == "" || !containsCR(input) {
		return input
	}
	return string(NormalizeLineEndingsBytes([]byte(input)))
}

func containsCR(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			return true
		}
	}
	return false
}
