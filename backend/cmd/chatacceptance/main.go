package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatartifact"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatextract"
)

type auditItem struct {
	Format           string `json:"format"`
	Filename         string `json:"filename,omitempty"`
	Uploaded         bool   `json:"uploaded,omitempty"`
	ExtractionOK     bool   `json:"extraction_ok,omitempty"`
	ControlExpected  string `json:"control_expected,omitempty"`
	ControlInExtract bool   `json:"control_in_extract,omitempty"`
	Asked            bool   `json:"asked,omitempty"`
	AnswerPreview    string `json:"answer_preview,omitempty"`
	ControlInAnswer  bool   `json:"control_in_answer,omitempty"`
	Generated        bool   `json:"generated,omitempty"`
	Downloaded       bool   `json:"downloaded,omitempty"`
	ParserOK         bool   `json:"parser_ok,omitempty"`
	Size             int    `json:"size,omitempty"`
	Status           string `json:"status"`
	Error            string `json:"error,omitempty"`
	SessionID        int64  `json:"session_id,omitempty"`
	AttachmentID     string `json:"attachment_id,omitempty"`
}

func main() {
	base := env("ACCEPTANCE_API", "http://localhost:18081")
	email := env("ACCEPTANCE_EMAIL", "demo@example.com")
	password := env("ACCEPTANCE_PASSWORD", "password123")
	fixtureDir := filepath.Join("debug", "ai-chat-audit", "fixtures")
	outDir := filepath.Join("debug", "ai-chat-audit", "acceptance")
	_ = os.MkdirAll(outDir, 0o755)
	_ = os.MkdirAll(filepath.Join(outDir, "downloads"), 0o755)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 180 * time.Second}

	must(login(client, base, email, password))

	inputMatrix := []struct {
		file     string
		format   string
		control  string
		question string
	}{
		{"control.txt", "txt", "TXT_CONTROL_16735", "Какой контрольный код указан в файле?"},
		{"control.md", "md", "MD_CONTROL_53892", "Какой контрольный код указан в Markdown?"},
		{"control.json", "json", "JSON_CONTROL_62418", "Какое значение поля control в JSON?"},
		{"control.csv", "csv", "CSV_CONTROL_71934", "Какой контрольный код есть в CSV?"},
		{"control.tsv", "tsv", "TSV_CONTROL_88421", "Какой контрольный код есть в TSV?"},
		{"control.xlsx", "xlsx", "XLSX_CONTROL_51283", "Какое значение в листе Контроль ячейке B7?"},
		{"control.pdf", "pdf", "PDF_CONTROL_92741", "Какой контрольный код указан в PDF?"},
		{"control.docx", "docx", "DOCX_CONTROL_38152", "Какой контрольный код указан в DOCX?"},
		{"control.pptx", "pptx", "PPTX_CONTROL_48216", "Какой текст/код на третьем слайде?"},
		{"control.png", "png", "PNG_CONTROL_55102", "Какой контрольный код связан с изображением? Если не читается OCR, скажи размеры."},
		{"control.jpg", "jpeg", "JPEG_CONTROL_66213", "Опиши изображение и укажи размеры."},
		{"control.webp", "webp", "WEBP_CONTROL_77324", "Опиши изображение WebP и размеры."},
		{"control.zip", "zip", "TXT_CONTROL_16735", "Перечисли файлы в ZIP и какой маркер в TXT внутри архива?"},
	}
	only := map[string]struct{}{}
	for _, part := range strings.Split(env("ACCEPTANCE_ONLY", ""), ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			only[part] = struct{}{}
		}
	}

	var inputResults []auditItem
	for _, tc := range inputMatrix {
		if len(only) > 0 {
			if _, ok := only[tc.format]; !ok {
				continue
			}
		}
		item := auditItem{Format: tc.format, Filename: tc.file, ControlExpected: tc.control, Status: "FAIL"}
		data, err := os.ReadFile(filepath.Join(fixtureDir, tc.file))
		if err != nil {
			item.Error = err.Error()
			inputResults = append(inputResults, item)
			continue
		}
		ext, err := chatextract.Extract(tc.file, "", data)
		if err == nil {
			item.ExtractionOK = true
			item.ControlInExtract = strings.Contains(ext.PlainText, tc.control) || strings.Contains(fmt.Sprint(ext.Metadata), tc.control)
		}
		attID, err := upload(client, base, tc.file, data)
		if err != nil {
			item.Error = "upload: " + err.Error()
			inputResults = append(inputResults, item)
			continue
		}
		item.Uploaded = true
		item.AttachmentID = attID
		q := fmt.Sprintf("Я приложил файл %s. %s Ответь коротко и обязательно процитируй контрольный код, если он есть в содержимом.", tc.file, tc.question)
		ans, sid, _, err := ask(client, base, q, []string{attID})
		item.Asked = err == nil
		item.SessionID = sid
		if err != nil {
			item.Error = "ask: " + err.Error()
		} else {
			item.AnswerPreview = trim(ans, 240)
			item.ControlInAnswer = strings.Contains(ans, tc.control)
			if item.ControlInAnswer && item.Uploaded && item.ExtractionOK {
				item.Status = "PASS"
			} else if item.ControlInExtract && item.Uploaded && item.Asked {
				item.Status = "PARTIAL"
				item.Error = "control present in extraction but missing in AI answer"
			}
		}
		inputResults = append(inputResults, item)
		fmt.Printf("INPUT %-5s %s\n", item.Status, tc.format)
	}

	outputMatrix := []struct {
		format string
		prompt string
	}{
		{"xlsx", "Выгрузи товары с риском остатка в Excel"},
		{"csv", "Выгрузи товары с риском остатка в CSV"},
		{"pdf", "Выгрузи сводку продаж в PDF"},
		{"docx", "Выгрузи сводку продаж в DOCX Word"},
		{"pptx", "Выгрузи сводку продаж в PPTX презентацию"},
		{"txt", "Выгрузи сводку продаж в TXT"},
		{"md", "Выгрузи сводку продаж в Markdown MD"},
		{"json", "Выгрузи сводку продаж в JSON"},
		{"png", "Выгрузи сводку продаж в PNG график"},
		{"jpeg", "Выгрузи сводку продаж в JPEG"},
		{"zip", "Выгрузи сводку продаж в ZIP архив"},
	}

	var outputResults []auditItem
	skipOutput := env("ACCEPTANCE_SKIP_OUTPUT", "") == "1"
	for _, tc := range outputMatrix {
		if skipOutput {
			break
		}
		if len(only) > 0 {
			if _, ok := only[tc.format]; !ok {
				continue
			}
		}
		item := auditItem{Format: tc.format, Status: "FAIL"}
		ans, sid, atts, err := ask(client, base, tc.prompt, nil)
		item.Asked = err == nil
		item.SessionID = sid
		if err != nil {
			item.Error = err.Error()
			outputResults = append(outputResults, item)
			fmt.Printf("OUTPUT %-5s %s (%s)\n", item.Status, tc.format, item.Error)
			continue
		}
		item.AnswerPreview = trim(ans, 200)
		if len(atts) == 0 {
			if fetched, ferr := latestAssistantAttachments(client, base, sid); ferr == nil {
				atts = fetched
			}
		}
		if len(atts) == 0 {
			item.Error = "no attachment in response"
			outputResults = append(outputResults, item)
			fmt.Printf("OUTPUT %-5s %s (no attachment)\n", item.Status, tc.format)
			continue
		}
		att := atts[0]
		item.AttachmentID = att.ID
		item.Filename = att.FileName
		item.Generated = true
		data, err := download(client, base, att.ID)
		if err != nil {
			item.Error = "download: " + err.Error()
			outputResults = append(outputResults, item)
			continue
		}
		item.Downloaded = true
		item.Size = len(data)
		_ = os.WriteFile(filepath.Join(outDir, "downloads", sanitize(att.FileName)), data, 0o644)
		item.ParserOK = validateGenerated(tc.format, data)
		if item.Generated && item.Downloaded && item.ParserOK {
			item.Status = "PASS"
		}
		outputResults = append(outputResults, item)
		fmt.Printf("OUTPUT %-5s %s size=%d\n", item.Status, tc.format, item.Size)
	}

	// Generator self-test for all formats (deterministic, no provider)
	var genSelf []auditItem
	for _, format := range []string{"pdf", "docx", "xlsx", "csv", "tsv", "pptx", "txt", "md", "json", "png", "jpeg", "zip"} {
		spec := chatartifact.ArtifactSpec{
			Type: format, Filename: "self." + format, Title: "Acceptance " + format,
			Sections: []chatartifact.ArtifactSection{{Heading: "H1", Paragraphs: []string{"Hello", "CONTROL_" + strings.ToUpper(format)}}},
			Tables:   []chatartifact.ArtifactTable{{Columns: []string{"A", "B"}, Rows: [][]string{{"1", "2"}}}},
			Workbook: &chatartifact.ArtifactWorkbook{Sheets: []chatartifact.ArtifactSheet{{Name: "S1", Columns: []string{"A"}, Rows: [][]string{{"1"}}}}},
			Slides:   []chatartifact.ArtifactSlide{{Title: "T1", Body: "B1"}, {Title: "T2", Body: "B2"}},
			Text:     "CONTROL_" + strings.ToUpper(format),
			JSON:     map[string]any{"control": "CONTROL_" + strings.ToUpper(format)},
			Chart:    &chatartifact.ArtifactChart{Title: "C", Labels: []string{"a", "b"}, Values: []float64{1, 2}},
		}
		if format == "zip" {
			spec.ZipFiles = []chatartifact.ArtifactSpec{
				{Type: "txt", Filename: "a.txt", Text: "A"},
				{Type: "json", Filename: "b.json", JSON: map[string]string{"ok": "1"}},
			}
		}
		data, _, name, err := chatartifact.Generate(spec)
		item := auditItem{Format: format, Filename: name, Generated: err == nil, Size: len(data), Status: "FAIL"}
		if err != nil {
			item.Error = err.Error()
		} else {
			item.ParserOK = validateGenerated(format, data)
			if item.ParserOK {
				item.Status = "PASS"
			}
			_ = os.WriteFile(filepath.Join(outDir, "downloads", "gen-"+name), data, 0o644)
		}
		genSelf = append(genSelf, item)
	}

	report := map[string]any{
		"generated_at":        time.Now().UTC().Format(time.RFC3339),
		"api":                 base,
		"input_formats":       inputResults,
		"output_formats_live": outputResults,
		"generator_self_test": genSelf,
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	outPath := filepath.Join(outDir, "acceptance-report.json")
	must(os.WriteFile(outPath, raw, 0o644))
	fmt.Println("wrote", outPath)
}

type attDTO struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
}

func login(c *http.Client, base, email, password string) error {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	res, err := c.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("login %d: %s", res.StatusCode, string(b))
	}
	return nil
}

func upload(c *http.Client, base, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		return "", err
	}
	_ = w.Close()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/chat/attachments", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("%d: %s", res.StatusCode, string(b))
	}
	var parsed struct {
		Attachment struct {
			ID string `json:"id"`
		} `json:"attachment"`
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &parsed)
	if parsed.Attachment.ID != "" {
		return parsed.Attachment.ID, nil
	}
	if parsed.ID != "" {
		return parsed.ID, nil
	}
	return "", fmt.Errorf("no attachment id in %s", string(b))
}

func ask(c *http.Client, base, question string, attachmentIDs []string) (string, int64, []attDTO, error) {
	payload := map[string]any{
		"question":          question,
		"client_request_id": fmt.Sprintf("accept-%d", time.Now().UnixNano()),
		"creation_source":   "acceptance",
	}
	if len(attachmentIDs) > 0 {
		payload["attachment_ids"] = attachmentIDs
	}
	body, _ := json.Marshal(payload)
	res, err := c.Post(base+"/api/v1/chat/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", 0, nil, fmt.Errorf("%d: %s", res.StatusCode, string(b))
	}
	var parsed struct {
		SessionID   int64    `json:"session_id"`
		Answer      string   `json:"answer"`
		Attachments []attDTO `json:"attachments"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", 0, nil, err
	}
	return parsed.Answer, parsed.SessionID, parsed.Attachments, nil
}

func latestAssistantAttachments(c *http.Client, base string, sessionID int64) ([]attDTO, error) {
	res, err := c.Get(fmt.Sprintf("%s/api/v1/chat/sessions/%d/messages?limit=50", base, sessionID))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var parsed struct {
		Items []struct {
			Role        string `json:"role"`
			Attachments []struct {
				ID       string `json:"id"`
				FileName string `json:"file_name"`
			} `json:"attachments"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	for i := len(parsed.Items) - 1; i >= 0; i-- {
		if parsed.Items[i].Role == "assistant" && len(parsed.Items[i].Attachments) > 0 {
			out := make([]attDTO, 0, len(parsed.Items[i].Attachments))
			for _, a := range parsed.Items[i].Attachments {
				out = append(out, attDTO{ID: a.ID, FileName: a.FileName})
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("none")
}

func download(c *http.Client, base, id string) ([]byte, error) {
	res, err := c.Get(base + "/api/v1/chat/attachments/" + url.PathEscape(id) + "/download")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("%d: %s", res.StatusCode, string(b))
	}
	return b, nil
}

func validateGenerated(format string, data []byte) bool {
	if len(data) < 8 {
		return false
	}
	switch format {
	case "pdf":
		return bytes.HasPrefix(data, []byte("%PDF"))
	case "png":
		return bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47})
	case "jpeg":
		return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
	case "zip", "docx", "xlsx", "pptx":
		return bytes.HasPrefix(data, []byte("PK"))
	case "json":
		var v any
		return json.Unmarshal(data, &v) == nil
	case "csv", "tsv", "txt", "md":
		return len(bytes.TrimSpace(data)) > 0
	default:
		return false
	}
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func trim(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func sanitize(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "_")
	if name == "" {
		return "file.bin"
	}
	return name
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
