package chatv2

import (
	"encoding/json"
	"strings"
	"unicode"
)

type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string { return e.Code + ": " + e.Message }

func ValidateResult(frozen FrozenTurnContext, reg *GroundingRegistry, res TurnResult) error {
	if strings.TrimSpace(res.Answer) == "" {
		return ValidationError{Code: "EMPTY_ANSWER", Message: "answer required"}
	}
	if !validMode(res.AnswerMode) {
		return ValidationError{Code: "INVALID_MODE", Message: string(res.AnswerMode)}
	}
	for _, h := range res.GroundingHandles {
		if _, ok := reg.Get(h); !ok {
			return ValidationError{Code: "PHANTOM_HANDLE", Message: h}
		}
	}
	if err := modeGrounding(frozen, res, reg); err != nil {
		return err
	}
	if err := semanticContract(res); err != nil {
		return err
	}
	if sellerSpecificUngrounded(res, frozen) {
		return ValidationError{Code: "UNGROUNDED_SELLER_CLAIM", Message: "seller-specific claim without grounding"}
	}
	return nil
}

func validMode(m AnswerMode) bool {
	switch m {
	case ModePublishedExplanation, ModeFactualQuery, ModeAdHocAnalysis, ModeActionAssistance,
		ModeAttachmentAnalysis, ModeMixedAnalysis, ModeExport, ModeGeneralGuidance:
		return true
	}
	return false
}

func hasType(res TurnResult, reg *GroundingRegistry, t RefType) bool {
	for _, h := range res.GroundingHandles {
		if e, ok := reg.Get(h); ok && e.Type == t {
			return true
		}
	}
	return false
}

func modeGrounding(frozen FrozenTurnContext, res TurnResult, reg *GroundingRegistry) error {
	switch res.AnswerMode {
	case ModePublishedExplanation:
		if frozen.ContextMode != ContextPublicationBound || (!hasType(res, reg, RefPublication) && !hasType(res, reg, RefFinding) && !hasType(res, reg, RefAction)) {
			return ValidationError{Code: "MODE_GROUNDING", Message: "PUBLISHED_EXPLANATION requires publication grounding"}
		}
	case ModeFactualQuery, ModeAdHocAnalysis:
		if !hasType(res, reg, RefFact) {
			return ValidationError{Code: "MODE_GROUNDING", Message: "factual modes require fact handles"}
		}
	case ModeActionAssistance:
		if frozen.Action == nil || (!hasType(res, reg, RefAction) && !hasType(res, reg, RefActionCase)) {
			return ValidationError{Code: "MODE_GROUNDING", Message: "ACTION_ASSISTANCE requires action grounding"}
		}
	case ModeAttachmentAnalysis:
		if !hasType(res, reg, RefAttachment) {
			return ValidationError{Code: "MODE_GROUNDING", Message: "ATTACHMENT_ANALYSIS requires attachment grounding"}
		}
	case ModeMixedAnalysis:
		need := 0
		if hasType(res, reg, RefAttachment) {
			need++
		}
		if hasType(res, reg, RefFact) || hasType(res, reg, RefPublication) || hasType(res, reg, RefFinding) {
			need++
		}
		if need < 2 {
			return ValidationError{Code: "MODE_GROUNDING", Message: "MIXED_ANALYSIS requires multiple source classes"}
		}
	case ModeExport:
		if res.ExportDataset == nil && !hasType(res, reg, RefFact) && !hasType(res, reg, RefAttachment) {
			return ValidationError{Code: "MODE_GROUNDING", Message: "EXPORT requires dataset grounding"}
		}
	}
	return nil
}

func semanticContract(res TurnResult) error {
	low := strings.ToLower(res.Answer)
	if (strings.Contains(low, "revenue") || strings.Contains(low, "выручка")) && containsSellerish(res.Answer) && res.AnswerMode != ModeGeneralGuidance {
		return ValidationError{Code: "REVENUE_MISLABEL", Message: "ordered_amount must not be labeled revenue/выручка"}
	}
	if strings.Contains(strings.ToUpper(res.Answer), "UNKNOWN") && (strings.Contains(low, "остаток 0") || strings.Contains(low, "stock = 0") || strings.Contains(low, "= 0")) {
		return ValidationError{Code: "UNKNOWN_STOCK_ZERO", Message: "UNKNOWN stock must not be rendered as 0"}
	}
	return nil
}

func sellerSpecificUngrounded(res TurnResult, frozen FrozenTurnContext) bool {
	if res.AnswerMode == ModeGeneralGuidance {
		return containsSellerish(res.Answer)
	}
	if len(res.GroundingHandles) > 0 {
		return false
	}
	return containsSellerish(res.Answer)
}

func containsSellerish(s string) bool {
	low := strings.ToLower(s)
	needles := []string{"у вас", "ваших", "ваш ", "sku", "заказано", "продаж", "остат", "дrr", "дrr", "кампан"}
	for _, n := range needles {
		if strings.Contains(low, n) {
			return true
		}
	}
	digit := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			digit = true
			break
		}
	}
	return digit && (strings.Contains(low, "%") || strings.Contains(low, "шт"))
}

func GroundingStatusFor(res TurnResult, limitations []string) GroundingStatus {
	if len(res.GroundingHandles) == 0 {
		if res.AnswerMode == ModeGeneralGuidance {
			return GroundingNone
		}
		return GroundingNone
	}
	if len(res.Limitations) > 0 || len(limitations) > 0 {
		return GroundingLimited
	}
	return GroundingGrounded
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
