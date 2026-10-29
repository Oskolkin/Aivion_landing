package chatv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashJSON(v any) string {
	b, _ := json.Marshal(v)
	return hashBytes(b)
}

func RequestHash(sellerID int64, userID *int64, sessionID *int64, question string, attachmentIDs []string, bindingID *int64) string {
	atts := append([]string(nil), attachmentIDs...)
	sort.Strings(atts)
	uid, sid, bid := "", "", ""
	if userID != nil {
		uid = itoa(*userID)
	}
	if sessionID != nil {
		sid = itoa(*sessionID)
	}
	if bindingID != nil {
		bid = itoa(*bindingID)
	}
	return hashJSON([]any{sellerID, uid, sid, strings.Join(strings.Fields(strings.TrimSpace(question)), " "), atts, bid})
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
