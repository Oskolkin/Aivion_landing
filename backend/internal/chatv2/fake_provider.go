package chatv2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FakeProvider is deterministic and scriptable for tests and AI_PROVIDER_MODE=fake.
type FakeProvider struct {
	mu            sync.Mutex
	script        []ProviderResponse
	fail          error
	loopForever   bool
	invalidThenOK bool
	attempt       int
	Delay         time.Duration
	LastRequests  []ProviderRequest
	Generations   int
}

func NewFakeProvider() *FakeProvider { return &FakeProvider{} }

func (f *FakeProvider) Script(resps ...ProviderResponse) *FakeProvider {
	f.script = append(f.script, resps...)
	return f
}

func (f *FakeProvider) Fail(err error) *FakeProvider { f.fail = err; return f }

func (f *FakeProvider) InvalidThenValid() *FakeProvider { f.invalidThenOK = true; return f }

func (f *FakeProvider) InfiniteTools() *FakeProvider { f.loopForever = true; return f }

func (f *FakeProvider) Turn(ctx context.Context, req ProviderRequest) (ProviderResponse, error) {
	if f.Delay > 0 {
		select {
		case <-ctx.Done():
			return ProviderResponse{}, ctx.Err()
		case <-time.After(f.Delay):
		}
	}
	if err := ctx.Err(); err != nil {
		return ProviderResponse{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LastRequests = append(f.LastRequests, req)
	f.Generations++
	if f.fail != nil {
		return ProviderResponse{}, f.fail
	}
	if f.loopForever {
		return ProviderResponse{ID: "fake-loop", ToolCalls: []ProviderToolCall{{CallID: fmt.Sprintf("c%d", f.Generations), Name: ToolGetPublicationOverview, Args: map[string]any{}}}}, nil
	}
	if f.invalidThenOK {
		f.attempt++
		if f.attempt == 1 {
			return ProviderResponse{ID: "fake-invalid", Final: &TurnResult{
				Answer: "У вас продажи выросли на 20%", AnswerMode: ModeFactualQuery, GroundingHandles: []string{"cf_999"}, Limitations: []string{},
			}}, nil
		}
	}
	if len(f.script) > 0 {
		r := f.script[0]
		f.script = f.script[1:]
		return r, nil
	}
	return f.defaultTurn(req), nil
}

func (f *FakeProvider) defaultTurn(req ProviderRequest) ProviderResponse {
	question := ""
	for _, m := range req.Input {
		if s, _ := m["question"].(string); s != "" {
			question = s
		}
		if s, _ := m["content"].(string); s != "" && question == "" {
			question = s
		}
	}
	q := strings.ToLower(question)
	id := "fake-" + strconv.Itoa(f.Generations)

	// If last input has tool results, produce final.
	hasToolResults := false
	for _, m := range req.Input {
		if m["type"] == "tool_result" {
			hasToolResults = true
			break
		}
	}
	if hasToolResults {
		rawCurrent := currentRoundToolJSON(req)
		if strings.Contains(q, "выгруз") || strings.Contains(strings.ToLower(string(mustJSON(req.Input))), "выгруз") {
			filters := exportFiltersFromToolJSON(rawCurrent, req)
			return ProviderResponse{ID: id, Final: &TurnResult{
				Answer: "Файл выгрузки подготовлен.", AnswerMode: ModeExport, GroundingHandles: []string{},
				ExportDataset: &ExportDatasetRef{Kind: "SKU", Period: "last_7d", Filters: filters},
			}}
		}
		if stringsContainsAny(q, "остат", "заказать", "заказа") && !strings.Contains(rawCurrent, "sellable_stock") {
			ids := skuIDsFromInput(req)
			if len(ids) > 0 {
				arr := make([]any, 0, len(ids))
				for _, n := range ids {
					arr = append(arr, n)
				}
				return ProviderResponse{ID: id, ToolCalls: []ProviderToolCall{{CallID: id + "-sku", Name: ToolGetSKUContexts, Args: map[string]any{"sku_ids": arr}}}}
			}
		}
		handles := collectHandles(req)
		mode := ModeGeneralGuidance
		answer := "Готово. Ответ основан на доступном контексте кабинета."
		lim := []string{}
		salesDown := strings.Contains(rawCurrent, `"sales_status":"UNAVAILABLE"`) || strings.Contains(rawCurrent, "данные продаж")
		adsDown := strings.Contains(rawCurrent, `"advertising_status":"UNAVAILABLE"`)
		if salesDown {
			mode = ModeGeneralGuidance
			answer = "Не могу достоверно ответить: необходимый фактический источник сейчас недоступен."
			lim = append(lim, "FACTUAL_SALES_UNAVAILABLE")
			lim = append(lim, "Не могу достоверно ответить: данные продаж для этого запроса сейчас недоступны.")
			return ProviderResponse{ID: id, Final: &TurnResult{Answer: answer, AnswerMode: mode, GroundingHandles: []string{}, Limitations: lim, ReferencedEntities: entityRefsFromRequest(req)}}
		}
		if strings.Contains(q, "реклам") {
			mode = ModeFactualQuery
			answer = "По текущим фактам рекламы по указанному SKU: см. grounded handles. Это не полный интернет-обзор правил Ozon."
			if adsDown {
				mode = ModeFactualQuery
				answer = "По продажам и остаткам могу ответить. Рекламный источник сейчас недоступен, поэтому рекламный вывод ограничен."
				lim = append(lim, "Рекламный контекст сейчас недоступен.")
			}
		} else if hasAttachmentContext(req) && (strings.Contains(q, "сравни") || strings.Contains(q, "файл") || strings.Contains(q, "план") || strings.Contains(q, "продаж") || strings.Contains(q, "остат")) {
			mode = ModeMixedAnalysis
			answer = "Сравнение вложения с фактами кабинета выполнено по обоим источникам."
		} else if strings.Contains(q, "сколько") || strings.Contains(q, "заказа") {
			mode = ModeFactualQuery
			answer = "Заказано на сумму и штуки по запрошенному SKU приведены в фактическом контексте. Не интерпретирую ordered_amount как выручку."
		} else if strings.Contains(q, "что мне сегодня") || strings.Contains(q, "самое важное") {
			mode = ModePublishedExplanation
			answer = "Сейчас самое важное следует из опубликованного анализа, без независимого переранжирования."
		} else if strings.Contains(q, "abc") {
			mode = ModeGeneralGuidance
			answer = "ABC-анализ — группировка ассортимента по вкладу. Это общее определение, не расчёт по вашему кабинету."
		} else if strings.Contains(q, "заказать") {
			mode = ModeFactualQuery
			answer = "Опираюсь на текущий фактический остаток, а не на прошлый ответ в истории чата."
		}
		if adsDown && mode != ModeGeneralGuidance && !strings.Contains(q, "реклам") {
			lim = append(lim, "Рекламный контекст сейчас недоступен.")
		}
		if mode == ModeMixedAnalysis {
			attH, factH := splitHandleClasses(req, handles)
			if len(attH) == 0 || len(factH) == 0 {
				mode = ModeGeneralGuidance
				lim = append(lim, "Недостаточно фактов для смешанного ответа.")
			} else {
				handles = append(append([]string{}, attH...), factH...)
			}
		}
		if len(handles) == 0 && mode != ModeGeneralGuidance {
			handles = []string{}
			mode = ModeGeneralGuidance
			lim = append(lim, "Недостаточно фактов для числового ответа.")
		}
		if stocks := extractSellableValues(rawCurrent); len(stocks) > 0 && mode != ModeMixedAnalysis && stringsContainsAny(q, "остат", "sku417", "polaris", "417", "сколько") {
			answer = fmt.Sprintf("Остаток Polaris 417: %.0f E2E_STOCK=%.0f", stocks[0], stocks[0])
			mode = ModeFactualQuery
		}
		if ac := extractActionContext(req, rawCurrent); ac.bound && mode != ModeMixedAnalysis {
			answer = fmt.Sprintf("%s E2E_ACTION origin=%d current_active=%d", strings.TrimSpace(answer), ac.originMembers, ac.currentActive)
			mode = ModeActionAssistance
			if ac.originPub > 0 && ac.currentPub > 0 && ac.originPub != ac.currentPub {
				answer += fmt.Sprintf(" E2E_HISTORICAL origin_pub=%d current_pub=%d", ac.originPub, ac.currentPub)
			}
			if ac.recommendedPrice > 0 {
				answer += fmt.Sprintf(" recommended_price=%.0f", ac.recommendedPrice)
			}
		}
		if hasAttachmentContext(req) && stringsContainsAny(q, "сравни", "файл", "план") {
			answer += " E2E_MIXED attachment+cabinet"
		}
		return ProviderResponse{ID: id, Final: &TurnResult{Answer: answer, AnswerMode: mode, GroundingHandles: handles, Limitations: lim, ReferencedEntities: entityRefsFromRequest(req)}}
	}

	calls := []ProviderToolCall{}
	if strings.Contains(q, "правила ozon") || strings.Contains(q, "интернет") {
		return ProviderResponse{ID: id, Final: &TurnResult{
			Answer:     "Aivion Chat не ищет информацию в интернете. Могу опереться только на публикацию и факты кабинета, либо дать общую справку без претензии на свежие правила площадки.",
			AnswerMode: ModeGeneralGuidance, GroundingHandles: []string{}, Limitations: []string{"no_web_search"},
		}}
	}
	if stringsContainsAny(q, "важн", "сегодня делать", "приоритет") {
		calls = append(calls, ProviderToolCall{CallID: id + "-ov", Name: ToolGetPublicationOverview, Args: map[string]any{}})
		calls = append(calls, ProviderToolCall{CallID: id + "-ac", Name: ToolListCurrentActions, Args: map[string]any{"limit": nil}})
	}
	if strings.Contains(q, "выгруз") {
		calls = append(calls, ProviderToolCall{CallID: id + "-se", Name: ToolSearchEntities, Args: searchEntitiesArgs(question)})
	}
	if stringsContainsAny(q, "помоги", "осталось", "действ", "рекомендовал", "1990") {
		calls = append(calls, ProviderToolCall{CallID: id + "-act", Name: ToolGetActionContext, Args: map[string]any{}})
	}
	if strings.Contains(q, "polaris") && !strings.Contains(q, "выгруз") && !stringsContainsAny(q, "помоги", "осталось", "действ", "рекомендовал") {
		calls = append(calls, ProviderToolCall{CallID: id + "-se", Name: ToolSearchEntities, Args: searchEntitiesArgs(question)})
		return ProviderResponse{ID: id, ToolCalls: calls}
	}
	if stringsContainsAny(q, "sku", "продаж", "остат", "заказа", "реклам", "сравни", "кампан", "категор", "осень", "leaf", "polaris", "расскажи про", "сколько") {
		ids := skuIDsFromInput(req)
		if len(ids) > 0 && !stringsContainsAny(q, "кампан", "категор") {
			arr := make([]any, 0, len(ids))
			for _, n := range ids {
				arr = append(arr, n)
			}
			calls = append(calls, ProviderToolCall{CallID: id + "-sku", Name: ToolGetSKUContexts, Args: map[string]any{"sku_ids": arr}})
		} else {
			calls = append(calls, ProviderToolCall{CallID: id + "-se", Name: ToolSearchEntities, Args: searchEntitiesArgs(question)})
		}
	}
	if strings.Contains(q, "файл") || strings.Contains(q, "вложен") || strings.Contains(q, "план") {
		calls = append(calls, ProviderToolCall{CallID: id + "-ov", Name: ToolGetPublicationOverview, Args: map[string]any{}})
		if !stringsContainsAny(q, "sku", "продаж", "остат", "заказа", "реклам", "сравни") {
			calls = append(calls, ProviderToolCall{CallID: id + "-met", Name: ToolListSKUMetrics, Args: listSKUMetricsArgs(10)})
		}
	}
	if len(calls) == 0 {
		return ProviderResponse{ID: id, Final: &TurnResult{
			Answer:     "Могу помочь с фактами кабинета, публикацией или вложениями. Уточните товар, период или действие.",
			AnswerMode: ModeGeneralGuidance, GroundingHandles: []string{}, Limitations: []string{},
		}}
	}
	return ProviderResponse{ID: id, ToolCalls: uniqueProviderToolCalls(calls)}
}

func uniqueProviderToolCalls(in []ProviderToolCall) []ProviderToolCall {
	if len(in) <= 1 {
		return in
	}
	seen := map[string]struct{}{}
	out := make([]ProviderToolCall, 0, len(in))
	for _, c := range in {
		key := strings.TrimSpace(c.CallID)
		if key == "" {
			key = c.Name + ":" + hashJSON(c.Args)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

func searchEntitiesArgs(query string) map[string]any {
	return map[string]any{"query": query, "limit": nil}
}

func listSKUMetricsArgs(limit int) map[string]any {
	return map[string]any{"period": nil, "date_from": nil, "date_to": nil, "limit": limit, "sort": nil}
}

func currentRoundToolJSON(req ProviderRequest) string {
	var parts []map[string]any
	for _, m := range req.Input {
		if m["type"] == "tool_result" {
			parts = append(parts, m)
		}
	}
	return string(mustJSON(parts))
}

func hasAttachmentContext(req ProviderRequest) bool {
	for _, m := range req.Input {
		if m["attachments"] == nil {
			continue
		}
		raw, _ := json.Marshal(m["attachments"])
		if len(raw) > 4 && string(raw) != "null" && string(raw) != "[]" {
			return true
		}
	}
	return false
}

func splitHandleClasses(req ProviderRequest, all []string) (att []string, fact []string) {
	attSeen := map[string]struct{}{}
	for _, m := range req.Input {
		if m["attachments"] == nil {
			continue
		}
		for _, h := range handlesIn(m["attachments"]) {
			if _, ok := attSeen[h]; ok {
				continue
			}
			attSeen[h] = struct{}{}
			att = append(att, h)
		}
	}
	factSeen := map[string]struct{}{}
	for _, m := range req.Input {
		if m["type"] != "tool_result" {
			continue
		}
		for _, h := range handlesIn(m) {
			if _, ok := attSeen[h]; ok {
				continue
			}
			if _, ok := factSeen[h]; ok {
				continue
			}
			factSeen[h] = struct{}{}
			fact = append(fact, h)
		}
	}
	if len(att) == 0 || len(fact) == 0 {
		// Fall back to collected handles if JSON shape hid one class.
		_ = all
	}
	return att, fact
}

func handlesIn(v any) []string {
	raw, _ := json.Marshal(v)
	var parsed any
	_ = json.Unmarshal(raw, &parsed)
	var out []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if s, ok := t["handle"].(string); ok && s != "" {
				out = append(out, s)
			}
			for _, y := range t {
				walk(y)
			}
		case []any:
			for _, y := range t {
				walk(y)
			}
		}
	}
	walk(parsed)
	return out
}

func handlesFromToolJSON(raw string) []string {
	seen := map[string]struct{}{}
	var out []string
	var parsed any
	_ = json.Unmarshal([]byte(raw), &parsed)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if k == "handle" {
					if s, ok := val.(string); ok && s != "" {
						if _, ok := seen[s]; !ok {
							seen[s] = struct{}{}
							out = append(out, s)
						}
					}
				}
				walk(val)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(parsed)
	return out
}

func collectHandles(req ProviderRequest) []string {
	seen := map[string]struct{}{}
	var out []string
	raw, _ := json.Marshal(req.Input)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, val := range t {
				if k == "handle" {
					if s, ok := val.(string); ok && s != "" {
						if _, ok := seen[s]; !ok {
							seen[s] = struct{}{}
							out = append(out, s)
						}
					}
				}
				walk(val)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	var parsed any
	_ = json.Unmarshal(raw, &parsed)
	walk(parsed)
	return out
}

func stringsContainsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func entityRefsFromRequest(req ProviderRequest) []EntityRef {
	return collectEntityRefsFromInput(req.Input)
}

func skuIDsFromInput(req ProviderRequest) []int64 {
	var out []int64
	raw, _ := json.Marshal(req.Input)
	var parsed any
	_ = json.Unmarshal(raw, &parsed)
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if id, ok := t["canonical_sku_id"].(float64); ok {
				out = append(out, int64(id))
			}
			if s, ok := t["canonical_sku_id"].(string); ok {
				if n, err := strconv.ParseInt(s, 10, 64); err == nil {
					out = append(out, n)
				}
			}
			if kind, _ := t["kind"].(string); strings.EqualFold(kind, "SKU") {
				switch id := t["id"].(type) {
				case float64:
					out = append(out, int64(id))
				case string:
					if n, err := strconv.ParseInt(id, 10, 64); err == nil {
						out = append(out, n)
					}
				}
			}
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(parsed)
	return out
}

func parseSKUNums(s string) []int64 {
	var out []int64
	cur := ""
	flush := func() {
		if len(cur) >= 3 && len(cur) <= 12 {
			n, _ := strconv.ParseInt(cur, 10, 64)
			if n > 0 {
				out = append(out, n)
			}
		}
		cur = ""
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur += string(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

type fakeActionContext struct {
	bound            bool
	originMembers    int
	currentActive    int
	originPub        int64
	currentPub       int64
	recommendedPrice float64
}

func extractSellableValues(raw string) []float64 {
	var out []float64
	var parsed any
	_ = json.Unmarshal([]byte(raw), &parsed)
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if ss, ok := t["sellable_stock"].(map[string]any); ok {
				if n, ok := ss["value"].(float64); ok {
					out = append(out, n)
				}
			}
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(parsed)
	return out
}

func extractActionContext(req ProviderRequest, rawToolJSON string) fakeActionContext {
	ac := extractActionContextFromToolJSON(rawToolJSON)
	for _, m := range req.Input {
		if act, ok := m["action"].(*ActionTurnContext); ok && act != nil {
			ac.bound = true
			ac.originMembers = act.OriginMemberCount
			ac.currentActive = act.CurrentActiveCount
			ac.originPub = act.OriginPublicationID
			for _, mem := range act.Members {
				params := mem.OriginParameters
				if len(params) == 0 {
					params = mem.Parameters
				}
				if rp, ok := asFakeFloat(params["recommended_price"]); ok && rp > 0 {
					ac.recommendedPrice = rp
					break
				}
			}
		}
		if bc, ok := m["base_context"].(map[string]any); ok {
			if n, ok := asFakeInt64(bc["publication_id"]); ok {
				ac.currentPub = n
			}
		}
	}
	return ac
}

func extractActionContextFromToolJSON(raw string) fakeActionContext {
	var ac fakeActionContext
	var parsed any
	_ = json.Unmarshal([]byte(raw), &parsed)
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if bound, ok := t["bound"].(bool); ok && bound {
				ac.bound = true
			}
			if n, ok := asFakeInt(t["origin_member_count"]); ok {
				ac.originMembers = n
			}
			if n, ok := asFakeInt(t["current_active_count"]); ok {
				ac.currentActive = n
			}
			if n, ok := asFakeInt64(t["origin_publication_id"]); ok {
				ac.originPub = n
			}
			if n, ok := asFakeInt64(t["current_publication_id"]); ok {
				ac.currentPub = n
			}
			if n, ok := asFakeInt64(t["publication_id"]); ok && ac.currentPub == 0 {
				ac.currentPub = n
			}
			if members, ok := t["members"].([]any); ok {
				for _, m := range members {
					mm, ok := m.(map[string]any)
					if !ok {
						continue
					}
					if params, ok := mm["origin_parameters"].(map[string]any); ok {
						if rp, ok := asFakeFloat(params["recommended_price"]); ok && rp > 0 {
							ac.recommendedPrice = rp
						}
					}
					if ac.recommendedPrice == 0 {
						if params, ok := mm["parameters"].(map[string]any); ok {
							if rp, ok := asFakeFloat(params["recommended_price"]); ok && rp > 0 {
								ac.recommendedPrice = rp
							}
						}
					}
				}
			}
			for _, x := range t {
				walk(x)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(parsed)
	if ac.currentPub == 0 {
		var pubID int64
		_ = json.Unmarshal([]byte(raw), &parsed)
		var walkPub func(any)
		walkPub = func(v any) {
			switch t := v.(type) {
			case map[string]any:
				if n, ok := asFakeInt64(t["id"]); ok && ac.currentPub == 0 {
					if _, hasSummary := t["executive_summary"]; hasSummary {
						pubID = n
					}
				}
				for _, x := range t {
					walkPub(x)
				}
			case []any:
				for _, x := range t {
					walkPub(x)
				}
			}
		}
		walkPub(parsed)
		ac.currentPub = pubID
	}
	return ac
}

func exportFiltersFromToolJSON(raw string, req ProviderRequest) []ExportFilterClause {
	ids := skuIDsFromInput(req)
	if len(ids) == 0 {
		var parsed any
		_ = json.Unmarshal([]byte(raw), &parsed)
		var walk func(any)
		walk = func(v any) {
			switch t := v.(type) {
			case map[string]any:
				if n, ok := asFakeInt64(t["canonical_sku_id"]); ok && n > 0 {
					ids = append(ids, n)
				}
				if n, ok := asFakeInt64(t["id"]); ok && n > 0 {
					if kind, _ := t["kind"].(string); strings.EqualFold(kind, "SKU") {
						ids = append(ids, n)
					}
				}
				for _, x := range t {
					walk(x)
				}
			case []any:
				for _, x := range t {
					walk(x)
				}
			}
		}
		walk(parsed)
	}
	if len(ids) == 0 {
		return nil
	}
	return []ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(ids[0], 10)}}
}

func asFakeInt(v any) (int, bool) {
	n, ok := asFakeInt64(v)
	if !ok {
		return 0, false
	}
	return int(n), true
}

func asFakeInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func asFakeFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
