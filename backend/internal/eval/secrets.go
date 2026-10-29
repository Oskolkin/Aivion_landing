package eval

import (
	"encoding/json"
	"regexp"
)

var (
	openAIKeyRe = regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_-]{10,}\b`)
	bearerRe    = regexp.MustCompile(`(?i)Authorization:\s*Bearer\s+\S+`)
	ozonTokenRe = regexp.MustCompile(`(?i)(ozon[_-]?(oauth|refresh|access)?[_-]?token)["']?\s*[:=]\s*["'][^"']+["']`)
)

type SecretHit struct {
	Kind  string
	Where string
}

func ScanSecrets(name string, body []byte) []SecretHit {
	var hits []SecretHit
	if openAIKeyRe.Find(body) != nil {
		hits = append(hits, SecretHit{Kind: "openai_key", Where: name})
	}
	if bearerRe.Find(body) != nil {
		hits = append(hits, SecretHit{Kind: "authorization_bearer", Where: name})
	}
	if ozonTokenRe.Find(body) != nil {
		hits = append(hits, SecretHit{Kind: "ozon_token", Where: name})
	}
	return hits
}

func HumanReviewPacket(c Case, frozen, baseline, candidate string, det map[string]any) []byte {
	payload := map[string]any{
		"case":                    c.CaseKey,
		"frozen_facts":            frozen,
		"baseline_output":         baseline,
		"candidate_output":        candidate,
		"deterministic":           det,
		"requested_human_ratings": []string{"groundedness", "causal_restraint", "action_warranted", "critical_fail"},
		"note":                    "Codex must not write human_review=PASS. Record reviews via record-human-review CLI.",
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return b
}
