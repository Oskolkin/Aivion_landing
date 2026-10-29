package chat

import (
	"context"
	"strings"
	"testing"
)

type mockEntityResolver struct {
	skuExists      map[int64]bool
	offerExists    map[string]bool
	campaignExists map[int64]bool
}

func (m *mockEntityResolver) ProductExistsBySKU(_ context.Context, _ int64, sku int64) (bool, error) {
	if m.skuExists == nil {
		return false, nil
	}
	return m.skuExists[sku], nil
}

func (m *mockEntityResolver) ProductExistsByOfferID(_ context.Context, _ int64, offerID string) (bool, error) {
	if m.offerExists == nil {
		return false, nil
	}
	return m.offerExists[offerID], nil
}

func (m *mockEntityResolver) AdCampaignExistsByExternalID(_ context.Context, _ int64, campaignExternalID int64) (bool, error) {
	if m.campaignExists == nil {
		return false, nil
	}
	return m.campaignExists[campaignExternalID], nil
}

func TestPrecheckMarketplaceQuestionsSupported(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	questions := []string{
		"Почему упали продажи?",
		"Что с рекламой?",
		"Какие товары скоро закончатся?",
		"Какие кампании имеют высокий ДРР?",
	}
	for _, q := range questions {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if !d.OpenAICalled || d.Route != PrecheckRouteSupportedChat {
			t.Fatalf("%q: expected supported_chat with OpenAI, got %+v", q, d)
		}
	}
}

func TestPrecheckConversationalQuestionsNotRejected(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	questions := []string{
		"Что у меня не так?",
		"Где я теряю деньги?",
		"Что проверить сегодня?",
		"Почему всё просело?",
	}
	for _, q := range questions {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if d.Route == PrecheckRouteUnsupported {
			t.Fatalf("%q: must not be unsupported, got %+v", q, d)
		}
	}
}

func TestPrecheckEdgeCasesGuardrails(t *testing.T) {
	resolver := &mockEntityResolver{
		skuExists:      map[int64]bool{},
		campaignExists: map[int64]bool{},
	}
	p := NewPrechecker(resolver)

	t.Run("period numbers are not SKU or campaign IDs", func(t *testing.T) {
		questions := []string{
			"Почему продажи упали за последние 7 дней?",
			"Покажи топ 10 товаров по заказам",
			"Что с кампаниями за 30 дней?",
			"рекламные кампании 30 дней",
			"кампании 30 дней",
		}
		for _, q := range questions {
			d, err := p.Evaluate(context.Background(), 1, q, nil)
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			if len(d.DetectedEntities) > 0 {
				t.Fatalf("%q: must not detect entities, got %+v", q, d.DetectedEntities)
			}
			if d.Route != PrecheckRouteSupportedChat || !d.OpenAICalled {
				t.Fatalf("%q: expected supported_chat with OpenAI, got %+v", q, d)
			}
		}
	})

	t.Run("seller conversation phrases are supported_chat", func(t *testing.T) {
		for _, q := range []string{
			"Что у меня не так?",
			"Где я теряю деньги?",
		} {
			d, err := p.Evaluate(context.Background(), 1, q, nil)
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			if d.Route == PrecheckRouteClarificationNeeded || d.Route == PrecheckRouteUnsupported {
				t.Fatalf("%q: should be supported_chat, got %+v", q, d)
			}
			if !d.OpenAICalled {
				t.Fatalf("%q: OpenAI should be called", q)
			}
		}
	})

	t.Run("marketplace context prevents unsupported rejection", func(t *testing.T) {
		for _, q := range []string{
			"Как курс валют влияет на цену товара на Ozon?",
			"Напиши SQL не надо, просто покажи продажи по SKU",
		} {
			d, err := p.Evaluate(context.Background(), 1, q, nil)
			if err != nil {
				t.Fatalf("%q: %v", q, err)
			}
			if d.Route == PrecheckRouteUnsupported {
				t.Fatalf("%q: must not be unsupported, got %+v", q, d)
			}
			if !d.OpenAICalled {
				t.Fatalf("%q: OpenAI should be allowed", q)
			}
		}
	})

	t.Run("export request executes without OpenAI", func(t *testing.T) {
		q := "Выгрузи товары с риском остатка в Excel"
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.Route != PrecheckRouteExportExecute || d.OpenAICalled {
			t.Fatalf("expected export_execute without OpenAI, got %+v", d)
		}
		if d.ExportRequest == nil || d.ExportRequest.ExportType != "stock_risks" || d.ExportRequest.Format != "xlsx" {
			t.Fatalf("unexpected export request: %+v", d.ExportRequest)
		}
	})

	t.Run("clearly off-topic question is unsupported without OpenAI", func(t *testing.T) {
		d, err := p.Evaluate(context.Background(), 1, "Какая погода завтра?", nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.Route != PrecheckRouteUnsupported || d.UnsupportedReason != PrecheckReasonUnsupportedTopic {
			t.Fatalf("expected unsupported_topic, got %+v", d)
		}
		if d.OpenAICalled {
			t.Fatal("OpenAI must not be called")
		}
	})

	t.Run("unknown SKU returns unknown_sku without OpenAI", func(t *testing.T) {
		d, err := p.Evaluate(context.Background(), 1, "Что с SKU 999999999?", nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.UnsupportedReason != PrecheckReasonUnknownSKU || d.OpenAICalled {
			t.Fatalf("expected unknown_sku without OpenAI, got %+v", d)
		}
		if d.DetectedEntities["sku"] != "999999999" {
			t.Fatalf("expected sku entity, got %+v", d.DetectedEntities)
		}
	})
}

func TestExtractCampaignIDCandidateIgnoresDayPeriods(t *testing.T) {
	if id, ok := extractCampaignIDCandidate("рекламные кампании 30 дней"); ok {
		t.Fatalf("expected no campaign id, got %d", id)
	}
	if id, ok := extractCampaignIDCandidate("Что с кампанией 404040?"); !ok || id != 404040 {
		t.Fatalf("expected campaign 404040, got %d ok=%v", id, ok)
	}
}

func TestExtractSKUCandidateIgnoresSmallPeriodNumbers(t *testing.T) {
	cases := []string{
		"Почему продажи упали за последние 7 дней?",
		"Покажи топ 10 товаров по заказам",
		"покажи продажи по SKU",
	}
	for _, q := range cases {
		if sku, ok := extractSKUCandidate(q); ok {
			t.Fatalf("%q: unexpected sku %d", q, sku)
		}
	}
	if sku, ok := extractSKUCandidate("Что с SKU 999999999?"); !ok || sku != 999999999 {
		t.Fatalf("expected sku 999999999, got %d ok=%v", sku, ok)
	}
}

func TestPrecheckShowQuestionsNotExport(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	for _, q := range []string{
		"Покажи товары с риском остатка",
		"Покажи рекламу",
	} {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if d.Route == PrecheckRouteExportExecute || d.Route == PrecheckRouteExportRequestCandidate {
			t.Fatalf("%q: should not be export route, got %+v", q, d)
		}
		if !d.OpenAICalled {
			t.Fatalf("%q: expected OpenAI for normal chat", q)
		}
	}
}

func TestPrecheckExportRequestDetection(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	cases := []struct {
		question   string
		route      PrecheckRoute
		exportType string
		format     string
	}{
		{"Выгрузи товары с риском остатка в Excel", PrecheckRouteExportExecute, "stock_risks", "xlsx"},
		{"Сделай CSV по рекламным кампаниям", PrecheckRouteExportExecute, "advertising_campaigns", "csv"},
		{"Выгрузи проблемные товары", PrecheckRouteExportRequestCandidate, "", ""},
		{"Сделай PDF по продажам", PrecheckRouteExportExecute, "sales_summary", "pdf"},
		{"Выгрузи конкурентов", PrecheckRouteExportRequestCandidate, "", ""},
	}
	for _, tc := range cases {
		d, err := p.Evaluate(context.Background(), 1, tc.question, nil)
		if err != nil {
			t.Fatalf("%q: %v", tc.question, err)
		}
		if d.Route != tc.route {
			t.Fatalf("%q: route=%s want %s", tc.question, d.Route, tc.route)
		}
		if d.OpenAICalled {
			t.Fatalf("%q: OpenAI must not be called", tc.question)
		}
		if tc.exportType != "" {
			if d.ExportRequest == nil {
				t.Fatalf("%q: missing export request", tc.question)
			}
			if d.ExportRequest.ExportType != tc.exportType || d.ExportRequest.Format != tc.format {
				t.Fatalf("%q: got type=%s format=%s", tc.question, d.ExportRequest.ExportType, d.ExportRequest.Format)
			}
		}
	}
}

func TestPrecheckUnsupportedTopics(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	for _, q := range []string{
		"Какая погода завтра?",
		"Как приготовить борщ?",
		"Напиши стих",
		"Кто президент Франции?",
	} {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if d.Route != PrecheckRouteUnsupported || d.UnsupportedReason != PrecheckReasonUnsupportedTopic {
			t.Fatalf("%q: expected unsupported_topic, got %+v", q, d)
		}
		if d.OpenAICalled || len(d.SuggestedRephrases) == 0 {
			t.Fatalf("%q: expected controlled response without OpenAI", q)
		}
	}
}

func TestPrecheckMixedMarketplaceContextNotRejected(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	for _, q := range []string{
		"Как курс валют влияет на цену товара на Ozon?",
		"Напиши SQL не надо, просто покажи продажи по SKU",
		"Закон влияет на продажи на маркетплейсе?",
	} {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if d.Route == PrecheckRouteUnsupported {
			t.Fatalf("%q: must not be unsupported, got %+v", q, d)
		}
	}
}

func TestPrecheckUnknownSKU(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{skuExists: map[int64]bool{}})
	d, err := p.Evaluate(context.Background(), 1, "Что с SKU 999999999?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.UnsupportedReason != PrecheckReasonUnknownSKU || d.OpenAICalled {
		t.Fatalf("expected unknown_sku without OpenAI, got %+v", d)
	}
}

func TestPrecheckKnownSKU(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{skuExists: map[int64]bool{123456: true}})
	d, err := p.Evaluate(context.Background(), 1, "Что с SKU 123456?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !d.OpenAICalled || d.Route != PrecheckRouteSupportedChat {
		t.Fatalf("expected supported_chat, got %+v", d)
	}
}

func TestPrecheckUnknownCampaign(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{campaignExists: map[int64]bool{}})
	d, err := p.Evaluate(context.Background(), 1, "Что с кампанией 404040?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.UnsupportedReason != PrecheckReasonUnknownCampaign || d.OpenAICalled {
		t.Fatalf("expected unknown_campaign, got %+v", d)
	}
}

func TestPrecheckShortAmbiguous(t *testing.T) {
	p := NewPrechecker(&mockEntityResolver{})
	for _, q := range []string{"что делать?", "проверь"} {
		d, err := p.Evaluate(context.Background(), 1, q, nil)
		if err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if d.Route == PrecheckRouteUnsupported {
			t.Fatalf("%q: must not be unsupported", q)
		}
	}
}

func TestPrecheckTracePayload(t *testing.T) {
	d := PrecheckDecision{
		Route:              PrecheckRouteUnsupported,
		UnsupportedReason:  PrecheckReasonUnsupportedTopic,
		OpenAICalled:       false,
		IsSupported:        false,
		SuggestedRephrases: []string{"a"},
		DetectedKeywords:   []string{"unsupported_topic_signal"},
	}
	payload := d.TracePayload("Какая погода?")
	if payload["openai_called"] != false {
		t.Fatal("expected openai_called=false")
	}
	if payload["unsupported_reason"] != PrecheckReasonUnsupportedTopic {
		t.Fatalf("unexpected reason: %v", payload["unsupported_reason"])
	}
}

func TestBuildPlannerUserPromptDefaultWindow(t *testing.T) {
	prompt := BuildPlannerUserPrompt("Почему упали продажи?", nil, nil)
	if !strings.Contains(prompt, "Default analytics window:") {
		t.Fatalf("expected default window in planner prompt: %s", prompt)
	}
}

func TestServiceAskPrecheckSkipsOpenAI(t *testing.T) {
	repo := newFakeRepo()
	ai := &fakeAIClient{}
	svc, err := NewServiceWithDeps(ServiceDeps{
		Repo:         repo,
		AIClient:     ai,
		Prechecker:   NewPrechecker(&mockEntityResolver{}),
		ToolExecutor: &fakeToolExecutor{},
		ToolRegistry: NewDefaultToolRegistry(), ToolPlanValidator: NewToolPlanValidator(NewDefaultToolRegistry()),
		ContextAssembler: NewContextAssembler(), AnswerValidator: NewAnswerValidator(),
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Ask(context.Background(), AskInput{SellerAccountID: 1, Question: "Какая погода завтра?"})
	if err != nil {
		t.Fatal(err)
	}
	if ai.planCalls != 0 || ai.ansCalls != 0 {
		t.Fatal("OpenAI must not be called for unsupported topic")
	}
	if out == nil || out.Answer == nil {
		t.Fatal("expected controlled answer")
	}
	if repo.createMessageCalls < 2 {
		t.Fatalf("expected user+assistant messages in storage, createMessageCalls=%d", repo.createMessageCalls)
	}
	if repo.completeTraceCalls != 1 {
		t.Fatal("trace should complete")
	}
	if repo.lastCompleteTrace.AssistantMessageID == nil {
		t.Fatal("completed trace should reference assistant message")
	}
	precheck, ok := repo.lastCompleteTrace.ValidatedToolPlanPayload["precheck"].(map[string]any)
	if !ok || precheck["openai_called"] != false {
		t.Fatalf("expected precheck trace payload, got %+v", repo.lastCompleteTrace.ValidatedToolPlanPayload)
	}
	if precheck["unsupported_reason"] == nil || precheck["unsupported_reason"] == "" {
		t.Fatalf("expected unsupported_reason in trace payload, got %+v", precheck)
	}
	if precheck["user_message"] != "Какая погода завтра?" {
		t.Fatalf("expected question in trace payload, got %+v", precheck["user_message"])
	}
}
