package dashboardhome

import (
	"encoding/json"
	"strings"
)

func unwrapSellerPulseNarrativeRaw(raw json.RawMessage) *SellerPulseNarrative {
	raw = unwrapNestedSellerPulseJSON(raw)
	if len(raw) == 0 {
		return nil
	}
	var narrative SellerPulseNarrative
	if err := json.Unmarshal(raw, &narrative); err != nil {
		return nil
	}
	normalizeSellerPulseNarrativePayload(&narrative)
	return &narrative
}

func unwrapNestedSellerPulseJSON(raw json.RawMessage) json.RawMessage {
	for depth := 0; depth < 4; depth++ {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return raw
		}
		if _, hasTone := envelope["tone"]; hasTone {
			return raw
		}
		if _, hasHeadline := envelope["headline"]; hasHeadline {
			return raw
		}
		inner, ok := envelope["seller_pulse_narrative"]
		if !ok || len(inner) == 0 {
			return raw
		}
		raw = inner
	}
	return raw
}

func sellerPulseNarrativeLooksNested(raw string) bool {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &envelope); err != nil {
		return false
	}
	if _, hasTone := envelope["tone"]; hasTone {
		return false
	}
	if _, hasHeadline := envelope["headline"]; hasHeadline {
		return false
	}
	_, ok := envelope["seller_pulse_narrative"]
	return ok
}
