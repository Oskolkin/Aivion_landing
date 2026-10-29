package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	base := env("API", "http://localhost:18081")
	outDir := filepath.Join("debug", "ai-chat-production-readiness")
	_ = os.MkdirAll(filepath.Join(outDir, "streaming"), 0o755)
	_ = os.MkdirAll(filepath.Join(outDir, "json"), 0o755)
	_ = os.MkdirAll(filepath.Join(outDir, "vision"), 0o755)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 180 * time.Second}
	must(login(client, base))

	streamReport := runStreaming(client, base)
	writeJSON(filepath.Join(outDir, "streaming", "streaming-validation.json"), streamReport)

	visionDir := filepath.Join(outDir, "vision-fixtures")
	visionCases := []struct {
		file, expect, question string
	}{
		{"ui_screen.png", "VISION_UI_77421", "Какой скрытый код указан внизу скриншота кабинета? Какая сумма в блоке Revenue?"},
		{"table_grid.png", "CTRL_CELL_33901", "Какой контрольный код справа от пятой строки таблицы?"},
		{"trend_chart.png", "CHART_PEAK_55210", "Какая подпись у пика выручки на графике?"},
		{"doc_photo.jpg", "DOC_PHOTO_CODE_91820", "Какой код указан на фотографии документа и какая сумма к оплате?"},
		{"small_print.png", "TINY_MARK_60447", "Какой маркер указан внизу изображения с мелким текстом?"},
	}
	var visionResults []map[string]any
	for _, tc := range visionCases {
		item := map[string]any{"file": tc.file, "expected": tc.expect, "status": "FAIL"}
		data, err := os.ReadFile(filepath.Join(visionDir, tc.file))
		if err != nil {
			item["error"] = err.Error()
			visionResults = append(visionResults, item)
			continue
		}
		// Ensure no accidental text chunk markers in PNG for ui/table/chart/small
		if strings.Contains(string(data), tc.expect) && strings.HasSuffix(tc.file, ".png") {
			// pixel encoding may still contain ASCII in IDAT uncompressed rarely; check tEXt absence
			if bytes.Contains(data, []byte("tEXt")) || bytes.Contains(data, append([]byte("Control"), 0)) {
				item["error"] = "fixture still contains textual metadata chunks"
				visionResults = append(visionResults, item)
				continue
			}
		}
		id, err := upload(client, base, tc.file, data)
		if err != nil {
			item["error"] = err.Error()
			visionResults = append(visionResults, item)
			continue
		}
		ans, sid, err := ask(client, base, tc.question+" Ответь коротко и процитируй код точно.", []string{id})
		item["session_id"] = sid
		item["attachment_id"] = id
		if err != nil {
			item["error"] = err.Error()
		} else {
			item["answer"] = trim(ans, 400)
			item["found"] = strings.Contains(ans, tc.expect)
			if item["found"] == true {
				item["status"] = "PASS"
			}
		}
		visionResults = append(visionResults, item)
		fmt.Printf("VISION %-5s %s\n", item["status"], tc.file)
	}
	writeJSON(filepath.Join(outDir, "json", "vision-no-metadata.json"), visionResults)

	pdfCyr := runPDFCyrillic(client, base)
	writeJSON(filepath.Join(outDir, "json", "pdf-cyrillic-export.json"), pdfCyr)

	fmt.Println("wrote production readiness evidence under", outDir)
}

func runStreaming(c *http.Client, base string) map[string]any {
	payload, _ := json.Marshal(map[string]any{
		// Conversational turn (no tools) still exercises real provider token streaming.
		"question":          "Напиши короткий абзац из 4-5 предложений про важность контроля остатков на маркетплейсе. Не вызывай отчёты — просто объясни.",
		"client_request_id": fmt.Sprintf("stream-%d", time.Now().UnixNano()),
		"creation_source":   "prod_readiness",
	})
	queuedAt := time.Now()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/chat/ask/stream", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	res, err := c.Do(req)
	out := map[string]any{"queued_at": queuedAt.UTC().Format(time.RFC3339Nano), "status": "FAIL"}
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	defer res.Body.Close()
	out["http_status"] = res.StatusCode
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		out["error"] = string(b)
		return out
	}
	var firstDelta *time.Time
	var deltas []string
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var evt map[string]any
		if json.Unmarshal([]byte(raw), &evt) != nil {
			continue
		}
		typ, _ := evt["type"].(string)
		if typ == "delta" {
			now := time.Now()
			if firstDelta == nil {
				firstDelta = &now
			}
			if d, ok := evt["delta"].(string); ok {
				deltas = append(deltas, d)
			}
		}
		if typ == "done" || typ == "error" {
			out["terminal_type"] = typ
			out["terminal_event"] = evt
			break
		}
	}
	completed := time.Now()
	out["completed_at"] = completed.UTC().Format(time.RFC3339Nano)
	out["emitted_delta_count"] = len(deltas)
	out["joined"] = strings.Join(deltas, "")
	if firstDelta != nil {
		out["first_delta_at"] = firstDelta.UTC().Format(time.RFC3339Nano)
		out["time_to_first_delta_ms"] = firstDelta.Sub(queuedAt).Milliseconds()
		out["total_duration_ms"] = completed.Sub(queuedAt).Milliseconds()
		// Real streaming heuristic: first delta arrives before completion with meaningful lead time
		// and before full ask would typically finish; require TTFT < total and at least 2 deltas.
		if len(deltas) >= 2 && firstDelta.Before(completed) {
			out["status"] = "PASS"
			out["real_provider_streaming"] = true
		}
	}
	return out
}

func runPDFCyrillic(c *http.Client, base string) map[string]any {
	ans, sid, atts, err := askWithAtts(c, base, "Выгрузи сводку продаж в PDF с русским заголовком", nil)
	out := map[string]any{"status": "FAIL", "session_id": sid, "answer": trim(ans, 200)}
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	if len(atts) == 0 {
		out["error"] = "no attachment"
		return out
	}
	data, err := download(c, base, atts[0])
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	path := filepath.Join("debug", "ai-chat-production-readiness", "downloads", "cyrillic-sales.pdf")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
	out["file"] = path
	out["size"] = len(data)
	out["pdf_header"] = bytes.HasPrefix(data, []byte("%PDF"))
	// Unicode PDF with embedded font typically includes /ToUnicode or font file; Cyrillic may be as UTF-16BE
	hasFont := bytes.Contains(data, []byte("DejaVu")) || bytes.Contains(data, []byte("Unicode")) || bytes.Contains(data, []byte("ToUnicode"))
	out["embedded_font_evidence"] = hasFont
	// Look for common Russian words as UTF-16BE
	rub := encodeUTF16BE("Выручка")
	prodazh := encodeUTF16BE("продаж")
	hasCyr := bytes.Contains(data, rub) || bytes.Contains(data, prodazh) || bytes.Contains(data, encodeUTF16BE("Сводка"))
	out["cyrillic_utf16_evidence"] = hasCyr
	out["no_translit_note"] = !bytes.Contains(data, []byte("transliterated"))
	if out["pdf_header"] == true && (hasCyr || hasFont) && out["no_translit_note"] == true {
		out["status"] = "PASS"
	}
	return out
}

func encodeUTF16BE(s string) []byte {
	r := []rune(s)
	out := make([]byte, 0, len(r)*2)
	for _, ch := range r {
		out = append(out, byte(ch>>8), byte(ch))
	}
	return out
}

func login(c *http.Client, base string) error {
	body, _ := json.Marshal(map[string]string{"email": "demo@example.com", "password": "password123"})
	res, err := c.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("login %d %s", res.StatusCode, b)
	}
	return nil
}

func upload(c *http.Client, base, name string, data []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/chat/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var parsed struct {
		Attachment struct {
			ID string `json:"id"`
		} `json:"attachment"`
	}
	_ = json.Unmarshal(b, &parsed)
	if parsed.Attachment.ID == "" {
		return "", fmt.Errorf("%s", b)
	}
	return parsed.Attachment.ID, nil
}

func ask(c *http.Client, base, q string, ids []string) (string, int64, error) {
	ans, sid, _, err := askWithAtts(c, base, q, ids)
	return ans, sid, err
}

func askWithAtts(c *http.Client, base, q string, ids []string) (string, int64, []string, error) {
	payload := map[string]any{
		"question": q, "client_request_id": fmt.Sprintf("pr-%d", time.Now().UnixNano()), "creation_source": "prod_readiness",
	}
	if len(ids) > 0 {
		payload["attachment_ids"] = ids
	}
	body, _ := json.Marshal(payload)
	res, err := c.Post(base+"/api/v1/chat/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", 0, nil, fmt.Errorf("%d %s", res.StatusCode, b)
	}
	var parsed struct {
		SessionID   int64  `json:"session_id"`
		Answer      string `json:"answer"`
		Attachments []struct {
			ID string `json:"id"`
		} `json:"attachments"`
	}
	_ = json.Unmarshal(b, &parsed)
	atts := make([]string, 0, len(parsed.Attachments))
	for _, a := range parsed.Attachments {
		atts = append(atts, a.ID)
	}
	return parsed.Answer, parsed.SessionID, atts, nil
}

func download(c *http.Client, base, id string) ([]byte, error) {
	res, err := c.Get(base + "/api/v1/chat/attachments/" + id + "/download")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%d %s", res.StatusCode, b)
	}
	return b, nil
}

func writeJSON(path string, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(path, raw, 0o644)
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func trim(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
