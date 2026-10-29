package graders

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type LLMGrader struct {
	Model         string
	RubricVersion string
	Prompt        string
}

func (g LLMGrader) Identity() (key, version, promptHash, model, rubric string) {
	sum := sha256.Sum256([]byte(g.Prompt))
	return LLMGraderKey, LLMGraderVersion, hex.EncodeToString(sum[:]), g.Model, g.RubricVersion
}

func (g LLMGrader) Grade(artifact []byte) error {
	if len(artifact) == 0 {
		return fmt.Errorf("empty artifact")
	}
	_ = hex.EncodeToString(sumSHA(artifact))
	return nil
}

func sumSHA(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
