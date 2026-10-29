// Package healthstatus maps cabinet health_score to deterministic presentation.
package healthstatus

import "strings"

// Canonical score bands (single source of truth for backend + API).
// Frontend must not invent alternate ranges.
//
// Bands (seller-facing):
//
//	80–100 — устойчивое состояние
//	65–79  — хорошее состояние, есть локальные риски
//	50–64  — требует внимания
//	30–49  — высокий риск
//	0–29   — критическое состояние
const (
	BandStableMin    = 80
	BandGoodMin      = 65
	BandAttentionMin = 50
	BandHighRiskMin  = 30
)

// Status is the seller-facing index status derived from health_score.
type Status struct {
	Code        string `json:"code"`        // stable | good | attention | high_risk | critical
	Severity    string `json:"severity"`    // low | medium | high | critical (API enum)
	Label       string `json:"label"`       // Russian badge text
	Explanation string `json:"explanation"` // short Russian explanation
	Tone        string `json:"tone"`        // UI tone hint
}

// FromScore returns deterministic status for a numeric health index.
// When score is nil/invalid, Code is empty (caller must not invent a badge).
func FromScore(score *int32) Status {
	if score == nil {
		return Status{}
	}
	s := int(*score)
	if s < 0 {
		s = 0
	}
	if s > 100 {
		s = 100
	}
	switch {
	case s >= BandStableMin:
		return Status{
			Code:        "stable",
			Severity:    "low",
			Label:       "Устойчивое состояние",
			Explanation: "Индекс в комфортном диапазоне: ключевые показатели кабинета без критических отклонений.",
			Tone:        "low",
		}
	case s >= BandGoodMin:
		return Status{
			Code:        "good",
			Severity:    "medium",
			Label:       "Хорошее состояние, есть локальные риски",
			Explanation: "Кабинет в целом стабилен, но отдельные факторы уже требуют точечной приоритизации.",
			Tone:        "medium",
		}
	case s >= BandAttentionMin:
		return Status{
			Code:        "attention",
			Severity:    "high",
			Label:       "Требует внимания",
			Explanation: "Индекс снижен: есть материальные риски, которые нужно закрыть в первую очередь.",
			Tone:        "high",
		}
	case s >= BandHighRiskMin:
		return Status{
			Code:        "high_risk",
			Severity:    "high",
			Label:       "Высокий риск",
			Explanation: "Негативные факторы заметно давят на кабинет и требуют срочных действий.",
			Tone:        "high",
		}
	default:
		return Status{
			Code:        "critical",
			Severity:    "critical",
			Label:       "Критическое состояние",
			Explanation: "Индекс критический: необходимо срочно устранить основные драйверы снижения.",
			Tone:        "critical",
		}
	}
}

// NormalizeSeverity maps legacy/AI severity strings onto the canonical set.
// Prefer FromScore when a numeric score is available.
func NormalizeSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "critical":
		return "critical"
	case "high", "risk", "warning", "attention", "high_risk":
		return "high"
	case "medium", "good":
		return "medium"
	case "low", "stable", "ok", "healthy":
		return "low"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}
