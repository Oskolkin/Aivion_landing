package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

func CaseArtifactKey(suiteKey, version, runID, caseKey, name string) string {
	return fmt.Sprintf("eval/%s/%s/%s/%s/%s", suiteKey, version, runID, caseKey, name)
}

type MemoryArtifacts struct {
	mu   sync.Mutex
	Data map[string][]byte
}

func NewMemoryArtifacts() *MemoryArtifacts {
	return &MemoryArtifacts{Data: map[string][]byte{}}
}

func (m *MemoryArtifacts) Put(ctx context.Context, key string, body []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if hits := ScanSecrets(key, body); len(hits) > 0 {
		return "", fmt.Errorf("refusing to store secret in artifact %s", key)
	}
	sum := sha256.Sum256(body)
	h := hex.EncodeToString(sum[:])
	cp := append([]byte(nil), body...)
	m.mu.Lock()
	m.Data[key] = cp
	m.mu.Unlock()
	return h, nil
}
