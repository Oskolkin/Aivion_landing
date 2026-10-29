package chat

import (
	"strings"
	"unicode"
)

// IsCorruptedChatTitle reports titles that are empty or mostly unreadable replacement glyphs.
func IsCorruptedChatTitle(title string) bool {
	s := strings.TrimSpace(title)
	if s == "" {
		return true
	}

	questionMarks := strings.Count(s, "?")
	lettersAndDigits := 0
	cyrillic := 0
	for _, r := range s {
		if r == '?' {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			lettersAndDigits++
			if r >= '\u0400' && r <= '\u04FF' {
				cyrillic++
			}
		}
	}
	if lettersAndDigits == 0 {
		return true
	}
	if questionMarks >= 3 && cyrillic == 0 {
		return true
	}
	runes := len([]rune(s))
	if questionMarks >= 3 && runes > 0 && float64(questionMarks)/float64(runes) >= 0.2 {
		return true
	}
	return false
}
