package main

import (
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
	base := "http://localhost:18081"
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 180 * time.Second}
	must(login(c, base))
	fixtureDir := filepath.Join("debug", "ai-chat-audit", "fixtures")
	files := []string{"control.pdf", "control.xlsx"}
	var ids []string
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(fixtureDir, name))
		must(err)
		id, err := upload(c, base, name, data)
		must(err)
		ids = append(ids, id)
	}
	q := "Сравни контрольные маркеры в PDF и XLSX. Назови оба кода PDF_CONTROL и XLSX_CONTROL и разницу источников."
	ans, sid, err := ask(c, base, q, ids)
	must(err)
	out := map[string]any{
		"session_id":     sid,
		"answer":         ans,
		"pass":           strings.Contains(ans, "PDF_CONTROL_92741") && strings.Contains(ans, "XLSX_CONTROL_51283"),
		"attachment_ids": ids,
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	_ = os.WriteFile(filepath.Join("debug", "ai-chat-audit", "acceptance", "multifile-pdf-xlsx.json"), raw, 0o644)
	fmt.Println(string(raw))
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
		return "", fmt.Errorf("upload: %s", b)
	}
	return parsed.Attachment.ID, nil
}

func ask(c *http.Client, base, q string, ids []string) (string, int64, error) {
	payload, _ := json.Marshal(map[string]any{
		"question": q, "attachment_ids": ids,
		"client_request_id": fmt.Sprintf("multi-%d", time.Now().UnixNano()),
		"creation_source":   "acceptance",
	})
	res, err := c.Post(base+"/api/v1/chat/ask", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", 0, fmt.Errorf("%d %s", res.StatusCode, b)
	}
	var parsed struct {
		SessionID int64  `json:"session_id"`
		Answer    string `json:"answer"`
	}
	_ = json.Unmarshal(b, &parsed)
	return parsed.Answer, parsed.SessionID, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
