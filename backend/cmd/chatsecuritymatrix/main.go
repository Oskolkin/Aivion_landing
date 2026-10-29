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

	"github.com/google/uuid"
)

type result struct {
	Case       string `json:"case"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

func main() {
	base := env("API", "http://localhost:18081")
	client := &http.Client{Timeout: 30 * time.Second}
	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	results := []result{{Case: "login", Status: "FAIL"}}
	if err := login(client, base); err != nil {
		results[0].Detail = err.Error()
		writeReport(base, results)
		return
	}
	results[0].Status = "PASS"

	results = append(results,
		checkRandomDownload(client, base),
		checkUploadRejected(client, base, "spoofed.pdf", []byte("MZ executable bytes"), "PDF magic is required"),
		checkTraversalUpload(client, base),
	)
	writeReport(base, results)
	for _, item := range results {
		fmt.Printf("%-35s %s (%d)\n", item.Case, item.Status, item.HTTPStatus)
	}
}

func login(c *http.Client, base string) error {
	payload, _ := json.Marshal(map[string]string{"email": env("SECURITY_EMAIL", "demo@example.com"), "password": env("SECURITY_PASSWORD", "password123")})
	res, err := c.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("login returned %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func checkRandomDownload(c *http.Client, base string) result {
	res, err := c.Get(base + "/api/v1/chat/attachments/" + uuid.NewString() + "/download")
	if err != nil {
		return result{Case: "random attachment download", Status: "FAIL", Detail: err.Error()}
	}
	defer res.Body.Close()
	item := result{Case: "random attachment download", HTTPStatus: res.StatusCode}
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden {
		item.Status = "PASS"
	} else {
		item.Status = "FAIL"
		item.Detail = "expected 404 or 403"
	}
	return item
}

func checkUploadRejected(c *http.Client, base, filename string, data []byte, expectation string) result {
	status, body, err := upload(c, base, filename, data)
	item := result{Case: "MIME-spoofed PDF upload", HTTPStatus: status, Detail: expectation}
	if err != nil {
		item.Status = "FAIL"
		item.Detail = err.Error()
		return item
	}
	if status >= 400 && status < 500 {
		item.Status = "PASS"
	} else {
		item.Status = "FAIL"
		item.Detail = fmt.Sprintf("%s; unexpected response: %s", expectation, strings.TrimSpace(body))
	}
	return item
}

func checkTraversalUpload(c *http.Client, base string) result {
	status, body, err := upload(c, base, "../../secret.txt", []byte("safe text"))
	item := result{Case: "path traversal upload filename", HTTPStatus: status}
	if err != nil {
		item.Status = "FAIL"
		item.Detail = err.Error()
		return item
	}
	if status >= 400 && status < 500 {
		item.Status, item.Detail = "PASS", "rejected"
		return item
	}
	var payload struct {
		Attachment struct {
			Name string `json:"name"`
		} `json:"attachment"`
	}
	_ = json.Unmarshal([]byte(body), &payload)
	if status < 300 && payload.Attachment.Name != "" && !strings.Contains(payload.Attachment.Name, "..") && !strings.ContainsAny(payload.Attachment.Name, `/\`) {
		item.Status, item.Detail = "PASS", "sanitized to "+payload.Attachment.Name
		return item
	}
	item.Status = "FAIL"
	item.Detail = "filename was neither rejected nor safely sanitized"
	return item
}

func upload(c *http.Client, base, filename string, data []byte) (int, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return 0, "", err
	}
	if _, err := part.Write(data); err != nil {
		return 0, "", err
	}
	if err := writer.Close(); err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chat/attachments", &body)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	response, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(response), nil
}

func writeReport(base string, results []result) {
	report := map[string]any{
		"generated_at":          time.Now().UTC().Format(time.RFC3339),
		"api":                   base,
		"results":               results,
		"zip_security_coverage": "zip-slip, absolute paths, file count, total size, compression ratio, nested archives, and encrypted-entry flags are unit-tested",
		"remaining_limitations": []string{"Encrypted ZIP detection relies on the standard ZIP general-purpose encryption flag; malformed archives that cannot be parsed are rejected by zip.NewReader but not separately classified."},
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	path := filepath.Join("debug", "ai-chat-production-readiness", "json", "security-matrix.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
	fmt.Println("wrote", path)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
