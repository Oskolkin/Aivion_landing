package evidence

import (
	"encoding/json"
	"strings"
	"unicode"
)

var piiKeys = []string{"buyer_name", "phone", "email", "address", "authorization", "api_key", "token", "password", "credential"}

func ValidateItem(item EvidenceItem, req NormalizedEvidenceRequest, seller int64, universeHash string) error {
	if item.EvidenceType != req.EvidenceType {
		return Integrity("evidence type mismatch")
	}
	if item.Subject.Kind != req.Subject.Kind || item.Subject.ID != req.Subject.ID {
		return Integrity("subject mismatch")
	}
	if (item.Related == nil) != (req.Related == nil) {
		return Integrity("related mismatch")
	}
	if item.Related != nil && req.Related != nil && (item.Related.Kind != req.Related.Kind || item.Related.ID != req.Related.ID) {
		return Integrity("related mismatch")
	}
	switch item.MaterializationStatus {
	case StatusComplete, StatusEmptyValid, StatusPartial, StatusFailed:
	default:
		return Integrity("invalid materialization_status")
	}
	return ScanPII(item.Data)
}

func ScanPII(raw json.RawMessage) error {
	s := strings.ToLower(string(raw))
	for _, k := range piiKeys {
		if strings.Contains(s, `"`+k+`"`) {
			return Contract(CodePII, "forbidden field "+k)
		}
	}
	if looksLikePhone(s) && strings.Contains(s, "phone") {
		return Contract(CodePII, "phone")
	}
	_ = unicode.ReplacementChar
	return nil
}

func looksLikePhone(s string) bool { return false }
