// Command _fetch_provider_resp retrieves OpenAI Responses by ID and prints a
// redacted diagnostic summary. It never prints API keys.
//
// Usage:
//
//	set OPENAI_API_KEY=...
//	go run ./debug/_fetch_provider_resp resp_abc resp_def
//
// Optional: -out-dir path writes redacted JSON diagnostics (no secrets).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type diag struct {
	ResponseID            string   `json:"response_id"`
	FetchOK               bool     `json:"fetch_ok"`
	FetchError            string   `json:"fetch_error,omitempty"`
	Status                string   `json:"status,omitempty"`
	Model                 string   `json:"model,omitempty"`
	IncompleteReason      string   `json:"incomplete_details_reason,omitempty"`
	ErrorCode             string   `json:"error_code,omitempty"`
	ErrorMessage          string   `json:"error_message,omitempty"`
	InputTokens           int64    `json:"usage_input_tokens,omitempty"`
	OutputTokens          int64    `json:"usage_output_tokens,omitempty"`
	TotalTokens           int64    `json:"usage_total_tokens,omitempty"`
	AssistantMessageCount int      `json:"assistant_message_count"`
	OutputTextBytes       int      `json:"output_text_bytes"`
	OutputItemTypes       []string `json:"output_item_types"`
	CodeInterpreterCalls  int      `json:"code_interpreter_call_count"`
	RefusalPresent        bool     `json:"refusal_present"`
	CompletedAt           any      `json:"completed_at,omitempty"`
	RequestID             string   `json:"x_request_id,omitempty"`
}

func main() {
	outDir := flag.String("out-dir", "", "optional directory for redacted diagnostic JSON files")
	flag.Parse()
	ids := flag.Args()
	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./debug/_fetch_provider_resp [-out-dir DIR] resp_id...")
		os.Exit(2)
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is not set; cannot fetch live responses")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	if *outDir != "" {
		_ = os.MkdirAll(*outDir, 0o755)
	}

	for _, id := range ids {
		d := fetchOne(client, apiKey, strings.TrimSpace(id))
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(d)
		if *outDir != "" && d.FetchOK {
			path := filepath.Join(*outDir, id+"-diag.json")
			raw, _ := json.MarshalIndent(d, "", "  ")
			_ = os.WriteFile(path, raw, 0o644)
			fmt.Fprintf(os.Stderr, "wrote %s\n", path)
		}
	}
}

func fetchOne(client *http.Client, apiKey, id string) diag {
	out := diag{ResponseID: id}
	if id == "" {
		out.FetchError = "empty response id"
		return out
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.openai.com/v1/responses/"+id, nil)
	if err != nil {
		out.FetchError = err.Error()
		return out
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		out.FetchError = err.Error()
		return out
	}
	defer resp.Body.Close()
	out.RequestID = strings.TrimSpace(resp.Header.Get("x-request-id"))
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		out.FetchError = err.Error()
		return out
	}
	if resp.StatusCode >= 300 {
		out.FetchError = fmt.Sprintf("http %d: %s", resp.StatusCode, truncate(string(body), 200))
		return out
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		out.FetchError = "json unmarshal: " + err.Error()
		return out
	}
	out.FetchOK = true
	out.Status, _ = payload["status"].(string)
	out.Model, _ = payload["model"].(string)
	out.CompletedAt = payload["completed_at"]
	if inc, ok := payload["incomplete_details"].(map[string]any); ok {
		out.IncompleteReason, _ = inc["reason"].(string)
	}
	if errObj, ok := payload["error"].(map[string]any); ok {
		out.ErrorCode, _ = errObj["code"].(string)
		out.ErrorMessage, _ = errObj["message"].(string)
	}
	if usage, ok := payload["usage"].(map[string]any); ok {
		out.InputTokens = asInt64(usage["input_tokens"])
		out.OutputTokens = asInt64(usage["output_tokens"])
		out.TotalTokens = asInt64(usage["total_tokens"])
	}
	if output, ok := payload["output"].([]any); ok {
		for _, item := range output {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			typ, _ := m["type"].(string)
			if typ != "" {
				out.OutputItemTypes = append(out.OutputItemTypes, typ)
			}
			switch typ {
			case "message":
				out.AssistantMessageCount++
				if ref, _ := m["refusal"].(string); strings.TrimSpace(ref) != "" {
					out.RefusalPresent = true
				}
				if content, ok := m["content"].([]any); ok {
					for _, c := range content {
						cm, _ := c.(map[string]any)
						if cm == nil {
							continue
						}
						ct, _ := cm["type"].(string)
						if ct == "output_text" || ct == "text" {
							if text, _ := cm["text"].(string); text != "" {
								out.OutputTextBytes += len(text)
							}
						}
						if ct == "refusal" {
							out.RefusalPresent = true
						}
					}
				}
			case "code_interpreter_call":
				out.CodeInterpreterCalls++
			}
		}
	}
	if text, _ := payload["output_text"].(string); text != "" && out.OutputTextBytes == 0 {
		out.OutputTextBytes = len(text)
	}
	if ref, _ := payload["refusal"].(string); strings.TrimSpace(ref) != "" {
		out.RefusalPresent = true
	}
	return out
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
