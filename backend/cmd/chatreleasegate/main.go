package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("AI chat release gate — run individual harnesses (nothing heavy is started here).")
	fmt.Println()
	fmt.Println("Suggested order (from backend/):")
	cmds := []struct {
		cmd  string
		out  string
		note string
	}{
		{"go run ./cmd/chatsecuritylive", "debug/ai-chat-release-gate/json/security-matrix.json", "live security cases (API env, optional DEMO_B_EMAIL)"},
		{"go run ./cmd/chatstorageaudit", "debug/ai-chat-release-gate/json/storage-integrity.json", "DB + MinIO attachment integrity (DATABASE_URL, S3_* buckets)"},
		{"go run ./cmd/chatstoragefix --dry-run", "debug/ai-chat-release-gate/json/storage-fix-report.json", "classify/reconcile missing keys + orphan objects (--apply to fix)"},
		{"go run ./cmd/chatsecuritymatrix", "debug/ai-chat-production-readiness/json/security-matrix.json", "lighter security matrix"},
		{"go run ./cmd/chatcabinetmatrix", "debug/ai-chat-release-gate/json/cabinet-question-matrix.json", "cabinet question matrix"},
		{"go run ./cmd/chatprodreadiness", "debug/ai-chat-production-readiness/", "streaming / vision / PDF readiness"},
		{"go run ./cmd/chatacceptance", "debug/ai-chat-audit/acceptance/", "input/export acceptance"},
	}
	for i, c := range cmds {
		fmt.Printf("  %d) %s\n     → %s\n     %s\n", i+1, c.cmd, filepath.FromSlash(c.out), c.note)
	}
	fmt.Println()
	fmt.Println("Env defaults: API=http://localhost:18081  SECURITY_EMAIL=demo@example.com")
	if v := strings.TrimSpace(os.Getenv("API")); v != "" {
		fmt.Println("API override:", v)
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		fmt.Println("DATABASE_URL: not set (chatstorageaudit will skip/fail)")
	} else {
		fmt.Println("DATABASE_URL: set")
	}
	fmt.Println()
	fmt.Println("This orchestrator intentionally runs nothing heavy.")
}
