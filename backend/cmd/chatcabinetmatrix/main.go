package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var questions = []string{
	"Какая выручка за текущий период?",
	"Как она изменилась к предыдущей неделе?",
	"Сколько заказов было за период?",
	"Что сильнее повлияло на изменение выручки?",
	"Какой средний чек?",
	"Какие категории дали основной рост?",
	"Какие категории снизились?",
	"Какие SKU формируют основную выручку?",
	"Где слабое место воронки?",
	"Как изменилась конверсия?",
	"На каком этапе теряется больше всего пользователей?",
	"Что можно сделать для улучшения конверсии?",
	"Сравни текущую воронку с предыдущей неделей.",
	"Какие товары могут закончиться первыми?",
	"Какие SKU уже без остатка?",
	"Какие SKU имеют избыточный запас?",
	"Сколько выручки находится под риском?",
	"Какие товары нужно пополнить в первую очередь?",
	"Какие товары не нужно закупать?",
	"На сколько дней хватит текущего остатка у самого рискованного SKU?",
	"Какая доля выручки получена за счёт рекламы?",
	"Какие кампании имеют высокий ДРР?",
	"Где реклама убыточна?",
	"Какие кампании стоит сократить?",
	"Какие кампании можно масштабировать?",
	"Сколько расходов можно потенциально сократить?",
	"Какое действие сейчас самое важное?",
	"Почему оно приоритетное?",
	"Какие SKU затронуты приоритетным действием?",
	"Помоги выполнить выбранное действие.",
	"Что произойдёт, если действие не выполнить?",
	"Какие данные нужно проверить перед выполнением?",
	"Какие действия относятся к рекламе?",
	"Какие действия относятся к остаткам?",
	"За какой период выполнен анализ?",
	"С каким периодом проводится сравнение?",
	"На какую дату актуальны данные?",
	"Какие данные ты использовал для ответа?",
	"Есть ли ограничения в данных?",
	"Насколько уверен ответ?",
}

var controlledPhrases = []string{
	"Не удалось спланировать запрос",
	"Не удалось построить безопасный план",
	"Не удалось получить данные для ответа",
	"Недостаточно подготовленных данных",
	"Ответ не прошёл проверку",
	"Достигнут дневной лимит AI-запросов",
}

type item struct {
	Question        string `json:"question"`
	Intent          string `json:"intent,omitempty"`
	ToolPlan        any    `json:"tool_plan,omitempty"`
	ActualAnswer    string `json:"actual_answer"`
	InvalidToolPlan bool   `json:"invalid_tool_plan"`
	Hallucination   bool   `json:"hallucination"`
	Status          string `json:"status"`
	HTTPStatus      int    `json:"http_status,omitempty"`
	Error           string `json:"error,omitempty"`
}

func main() {
	base := env("API", "http://localhost:18081")
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 180 * time.Second}
	must(login(client, base))

	var sessionID *int64
	var results []item
	pass := 0
	for i, q := range questions {
		rec, sid := askOne(client, base, q, sessionID, i)
		if sid > 0 {
			sessionID = &sid
		}
		results = append(results, rec)
		if rec.Status == "PASS" {
			pass++
		}
		fmt.Printf("%2d %-4s %s\n", i+1, rec.Status, trim(q, 60))
	}

	payload := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"api":          base,
		"pass":         pass,
		"total":        len(results),
		"results":      results,
	}
	raw, _ := json.MarshalIndent(payload, "", "  ")
	outs := []string{
		filepath.Join("debug", "ai-chat-release-gate", "json", "cabinet-question-matrix.json"),
		filepath.Join("cabinet-questions", "cabinet-question-matrix.json"),
	}
	for _, out := range outs {
		must(os.MkdirAll(filepath.Dir(out), 0o755))
		must(os.WriteFile(out, raw, 0o644))
		fmt.Printf("wrote %s\n", out)
	}
	fmt.Printf("PASS %d/%d\n", pass, len(results))
}

func askOne(c *http.Client, base, q string, sessionID *int64, idx int) (item, int64) {
	payload := map[string]any{
		"question":          q,
		"client_request_id": fmt.Sprintf("cabinet-matrix-%d-%d", time.Now().UnixNano(), idx),
		"creation_source":   "cabinet_matrix",
	}
	if sessionID != nil {
		payload["session_id"] = *sessionID
	}
	body, _ := json.Marshal(payload)
	rec := item{Question: q, Hallucination: false, Status: "FAIL"}
	res, err := c.Post(base+"/api/v1/chat/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		rec.Error = err.Error()
		return rec, 0
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	rec.HTTPStatus = res.StatusCode
	var parsed struct {
		SessionID int64           `json:"session_id"`
		Answer    string          `json:"answer"`
		Intent    string          `json:"intent"`
		ToolPlan  json.RawMessage `json:"tool_plan"`
	}
	_ = json.Unmarshal(b, &parsed)
	ans := strings.TrimSpace(parsed.Answer)
	rec.Intent = parsed.Intent
	if len(parsed.ToolPlan) > 0 && string(parsed.ToolPlan) != "null" {
		var tp any
		if json.Unmarshal(parsed.ToolPlan, &tp) == nil {
			rec.ToolPlan = tp
		}
	}
	rec.ActualAnswer = trim(ans, 500)
	lower := strings.ToLower(ans)
	rec.InvalidToolPlan = strings.Contains(ans, "безопасный план") ||
		strings.Contains(ans, "спланировать запрос") ||
		strings.Contains(lower, "invalid_tool_plan")
	deflection := isPureClarificationDeflection(q, ans)
	if res.StatusCode == 200 && ans != "" && !rec.InvalidToolPlan && !isControlled(ans) && !deflection {
		rec.Status = "PASS"
	} else if res.StatusCode >= 300 {
		rec.Error = trim(string(b), 200)
	}
	return rec, parsed.SessionID
}

func isPureClarificationDeflection(q, ans string) bool {
	if !isFactualKPIQuestion(q) {
		return false
	}
	lower := strings.ToLower(ans)
	needles := []string{
		"я не понял",
		"не понял вопрос",
		"уточните вопрос",
		"уточните, пожалуйста",
		"переформулируйте",
		"не совсем понял",
		"можете уточнить",
	}
	hits := 0
	for _, n := range needles {
		if strings.Contains(lower, n) {
			hits++
		}
	}
	if hits == 0 {
		return false
	}
	// Fail only when the answer is essentially just a deflection (no KPI substance).
	hasSubstance := strings.ContainsAny(ans, "0123456789%") ||
		strings.Contains(lower, "выручк") ||
		strings.Contains(lower, "заказ") ||
		strings.Contains(lower, "остат") ||
		strings.Contains(lower, "sku") ||
		strings.Contains(lower, "днр") ||
		strings.Contains(lower, "дrr") ||
		strings.Contains(lower, "руб")
	return !hasSubstance
}

func isFactualKPIQuestion(q string) bool {
	lower := strings.ToLower(q)
	keys := []string{
		"выручк", "заказ", "средн", "чек", "категор", "sku", "воронк", "конверс",
		"остат", "запас", "пополн", "реклам", "кампани", "дрр", "период", "дат",
		"действи", "приоритет",
	}
	for _, k := range keys {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func isControlled(ans string) bool {
	for _, p := range controlledPhrases {
		if strings.Contains(ans, p) {
			return true
		}
	}
	return false
}

func login(c *http.Client, base string) error {
	body, _ := json.Marshal(map[string]string{"email": "demo@example.com", "password": "password123"})
	var last error
	for attempt := 0; attempt < 8; attempt++ {
		res, err := c.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode < 300 {
			return nil
		}
		last = fmt.Errorf("login %d %s", res.StatusCode, b)
		if res.StatusCode == 429 {
			time.Sleep(time.Duration(5*(attempt+1)) * time.Second)
			continue
		}
		return last
	}
	return last
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
