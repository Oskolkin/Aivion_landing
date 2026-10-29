package cleanup

import (
	"strings"
	"testing"
)

func TestArchiveStaleSessionsQueryIsUpdateOnly(t *testing.T) {
	q := strings.ToLower(ArchiveStaleActiveChatSessionsSQL)
	if strings.Contains(q, "delete") {
		t.Fatal("cleanup must not DELETE rows")
	}
	if !strings.Contains(q, "update chat_sessions") {
		t.Fatal("expected UPDATE chat_sessions")
	}
}
