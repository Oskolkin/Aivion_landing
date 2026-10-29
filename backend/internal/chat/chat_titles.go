package chat

import "strings"

const chatTitleMaxRunes = 72

// BuildChatTitle returns a deterministic session title without extra OpenAI calls.
func BuildChatTitle(actionTitle, firstUserMessage, fallback string) string {
	if t := trimTitleCandidate(actionTitle); t != "" {
		return t
	}
	if t := trimTitleCandidate(firstUserMessage); t != "" {
		return t
	}
	if fb := strings.TrimSpace(fallback); fb != "" {
		return fb
	}
	return "Новый чат"
}

func trimTitleCandidate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) > chatTitleMaxRunes {
		return strings.TrimSpace(string(r[:chatTitleMaxRunes])) + "…"
	}
	return s
}
