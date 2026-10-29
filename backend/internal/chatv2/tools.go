package chatv2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type CountingAPI struct {
	Inner         factualapi.API
	Calls         atomic.Int64
	SKUGets       atomic.Int64
	BatchSKU      atomic.Int64
	CategoryGets  atomic.Int64
	BatchCategory atomic.Int64
}

func (c *CountingAPI) wrap() { c.Calls.Add(1) }

type ToolExecutor struct {
	API     factualapi.API
	Pubs    PublicationReader
	Policy  ToolPolicy
	Counter *CountingAPI
	// TestBatches overrides factualapi.API for unit tests.
	TestBatches *TestBatches
	Executions  atomic.Int64
}

type TestBatches struct {
	SKUs        func(ctx context.Context, ids []int64) (factualapi.BatchSKUResult, error)
	Demand      func(ctx context.Context, ids []int64) (factualapi.BatchSKUDemandResult, error)
	Stock       func(ctx context.Context, ids []int64) (factualapi.BatchSKUStockResult, error)
	Price       func(ctx context.Context, ids []int64) (factualapi.BatchSKUPriceResult, error)
	Advertising func(ctx context.Context, ids []int64) (factualapi.BatchSKUAdvertisingResult, error)
	Campaigns   func(ctx context.Context, ids []int64) (factualapi.BatchCampaignResult, error)
	Categories  func(ctx context.Context, ids []int64) (factualapi.BatchCategoryResult, error)
}

type ToolOutput struct {
	Payload    map[string]any
	Handles    []string
	ErrorCode  string
	Limitation string
	Final      *TurnResult
}

func (e *ToolExecutor) api() factualapi.API {
	if e.Counter != nil {
		return e.Counter.Inner
	}
	return e.API
}

func (e *ToolExecutor) Execute(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, key string, args map[string]any) (ToolOutput, error) {
	if args == nil {
		args = map[string]any{}
	}
	if err := ValidateToolArguments(key, args); err != nil {
		code := providerToolCallFailureCode(err)
		if code == "INVALID_TOOL_CALL" {
			code = "INVALID_ARGS"
		}
		return ToolOutput{ErrorCode: code}, err
	}
	e.Executions.Add(1)
	switch key {
	case ToolSubmitTurnResult:
		return e.submit(args)
	case ToolGetPublicationOverview:
		return e.publicationOverview(frozen, reg)
	case ToolListPublicationFindings:
		return e.listFindings(frozen, reg, intArg(args, "limit", 8))
	case ToolListCurrentActions:
		return e.listActions(frozen, reg, intArg(args, "limit", 12))
	case ToolGetActionContext:
		return e.actionContext(frozen, reg)
	case ToolSearchEntities:
		return e.search(ctx, frozen, reg, stringArg(args, "query"), intArg(args, "limit", 8))
	case ToolGetSKUContexts:
		ids, err := int64Slice(args["sku_ids"], e.Policy.MaxEntities)
		if err != nil {
			return ToolOutput{ErrorCode: "INVALID_ARGS"}, err
		}
		return e.skuContexts(ctx, frozen, reg, ids)
	case ToolGetCategoryContexts:
		ids, err := int64Slice(args["category_ids"], e.Policy.MaxEntities)
		if err != nil {
			return ToolOutput{ErrorCode: "INVALID_ARGS"}, err
		}
		return e.categoryContexts(ctx, frozen, reg, ids)
	case ToolGetCampaignContexts:
		ids, err := int64Slice(args["campaign_ids"], e.Policy.MaxEntities)
		if err != nil {
			return ToolOutput{ErrorCode: "INVALID_ARGS"}, err
		}
		return e.campaignContexts(ctx, frozen, reg, ids)
	case ToolGetAccountDomainFacts:
		return e.accountDomain(ctx, frozen, reg, stringArg(args, "domain"), args)
	case ToolListSKUMetrics:
		return e.listSKUMetrics(ctx, frozen, reg, args)
	case ToolListCategoryMetrics:
		return e.listCategoryMetrics(ctx, frozen, reg, args)
	case ToolListCampaignMetrics:
		return e.listCampaignMetrics(ctx, frozen, reg, args)
	default:
		return ToolOutput{ErrorCode: "UNKNOWN_TOOL"}, fmt.Errorf("unknown tool")
	}
}

func (e *ToolExecutor) submit(args map[string]any) (ToolOutput, error) {
	raw, _ := json.Marshal(args)
	var res TurnResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return ToolOutput{ErrorCode: "INVALID_ARGS"}, ErrMalformedToolArgs
	}
	if strings.TrimSpace(res.Answer) == "" || res.AnswerMode == "" {
		return ToolOutput{ErrorCode: "INVALID_ARGS"}, ErrMalformedToolArgs
	}
	return ToolOutput{Final: &res, Payload: args}, nil
}

func (e *ToolExecutor) publicationOverview(frozen FrozenTurnContext, reg *GroundingRegistry) (ToolOutput, error) {
	if frozen.ContextMode != ContextPublicationBound || frozen.Publication == nil {
		return ToolOutput{Payload: map[string]any{"available": false, "reason": "NO_CURRENT_PUBLICATION"}, Limitation: PrepublicationCopy()}, nil
	}
	h := reg.Register(RefPublication, strconv.FormatInt(frozen.Publication.ID, 10), frozen.Hashes.Base, "Current publication")
	return ToolOutput{Payload: map[string]any{"available": true, "base": frozen.BaseContext, "handle": h.Handle}, Handles: []string{h.Handle}}, nil
}

func (e *ToolExecutor) listFindings(frozen FrozenTurnContext, reg *GroundingRegistry, limit int) (ToolOutput, error) {
	if frozen.ContextMode != ContextPublicationBound {
		return ToolOutput{Payload: map[string]any{"findings": []any{}, "available": false}}, nil
	}
	raw, _ := frozen.BaseContext["top_findings"].([]projectionFinding)
	_ = raw
	findings, _ := frozen.BaseContext["top_findings"].([]any)
	if findings == nil {
		if typed, ok := frozen.BaseContext["top_findings"]; ok {
			b, _ := json.Marshal(typed)
			_ = json.Unmarshal(b, &findings)
		}
	}
	if limit <= 0 {
		limit = 8
	}
	out := []map[string]any{}
	handles := []string{}
	for i, f := range findings {
		if i >= limit {
			break
		}
		b, _ := json.Marshal(f)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		title, _ := m["title"].(string)
		ref, _ := m["ref"].(string)
		if ref == "" {
			ref = fmt.Sprintf("finding_%d", i+1)
		}
		h := reg.Register(RefFinding, ref, hashJSON(m), title)
		m["handle"] = h.Handle
		out = append(out, m)
		handles = append(handles, h.Handle)
	}
	return ToolOutput{Payload: map[string]any{"findings": out}, Handles: handles}, nil
}

type projectionFinding struct{}

func (e *ToolExecutor) listActions(frozen FrozenTurnContext, reg *GroundingRegistry, limit int) (ToolOutput, error) {
	if frozen.ContextMode != ContextPublicationBound {
		return ToolOutput{Payload: map[string]any{"actions": []any{}, "available": false}}, nil
	}
	var items []any
	if typed, ok := frozen.BaseContext["current_actions"]; ok {
		b, _ := json.Marshal(typed)
		_ = json.Unmarshal(b, &items)
	}
	if limit <= 0 {
		limit = 12
	}
	out := []map[string]any{}
	handles := []string{}
	for i, f := range items {
		if i >= limit {
			break
		}
		var m map[string]any
		b, _ := json.Marshal(f)
		_ = json.Unmarshal(b, &m)
		ref, _ := m["local_ref"].(string)
		if ref == "" {
			ref = fmt.Sprintf("action_%d", i+1)
		}
		title, _ := m["title"].(string)
		h := reg.Register(RefAction, ref, hashJSON(m), title)
		m["handle"] = h.Handle
		out = append(out, m)
		handles = append(handles, h.Handle)
	}
	return ToolOutput{Payload: map[string]any{"actions": out}, Handles: handles}, nil
}

func (e *ToolExecutor) actionContext(frozen FrozenTurnContext, reg *GroundingRegistry) (ToolOutput, error) {
	if frozen.Action == nil {
		return ToolOutput{Payload: map[string]any{"bound": false}}, nil
	}
	h := reg.Register(RefAction, strconv.FormatInt(frozen.Action.BindingID, 10), frozen.Hashes.Action,
		fmt.Sprintf("%s origin=%d current_active=%d", frozen.Action.Title, frozen.Action.OriginMemberCount, frozen.Action.CurrentActiveCount))
	payload := map[string]any{
		"bound": true, "handle": h.Handle, "title": frozen.Action.Title, "origin_kind": frozen.Action.OriginKind,
		"origin_publication_id": frozen.Action.OriginPublicationID, "origin_member_count": frozen.Action.OriginMemberCount,
		"current_active_count": frozen.Action.CurrentActiveCount, "members": frozen.Action.Members,
	}
	if frozen.Publication != nil {
		payload["current_publication_id"] = frozen.Publication.ID
	}
	return ToolOutput{Payload: payload, Handles: []string{h.Handle}}, nil
}

func (e *ToolExecutor) search(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, query string, limit int) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Фактические данные сейчас недоступны."}, nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return ToolOutput{ErrorCode: "INVALID_ARGS"}, ErrMalformedToolArgs
	}
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
	}
	found, err := api.SearchEntities(ctx, frozen.Factual, factualapi.EntitySearchRequest{
		Query: query,
		Kinds: factualapi.DefaultEntitySearchKinds(nil),
		Limit: limit,
	})
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Поиск сущностей сейчас недоступен."}, nil
	}
	var matches []map[string]any
	var handles []string
	for _, it := range found.Items {
		switch it.Kind {
		case factualapi.EntitySearchSKU:
			ozon := int64(0)
			if it.OzonSKU != nil {
				ozon = *it.OzonSKU
			}
			h := reg.Register(RefFact, "sku:"+strconv.FormatInt(it.CanonicalID, 10), hashJSON(it.CanonicalID), fmt.Sprintf("SKU %d %s", ozon, it.Name))
			row := map[string]any{"kind": "SKU", "id": it.CanonicalID, "canonical_sku_id": it.CanonicalID, "ozon_sku": ozon, "name": it.Name, "offer_id": it.OfferID, "match_reason": it.MatchReason, "handle": h.Handle}
			matches = append(matches, row)
			handles = append(handles, h.Handle)
		case factualapi.EntitySearchCampaign:
			h := reg.Register(RefFact, "campaign:"+strconv.FormatInt(it.CanonicalID, 10), hashJSON(it.CanonicalID), it.Name)
			row := map[string]any{"kind": "CAMPAIGN", "id": it.CanonicalID, "canonical_campaign_id": it.CanonicalID, "name": it.Name, "match_reason": it.MatchReason, "handle": h.Handle}
			if it.ProviderID != nil {
				row["provider_campaign_id"] = *it.ProviderID
			}
			matches = append(matches, row)
			handles = append(handles, h.Handle)
		case factualapi.EntitySearchCategory:
			h := reg.Register(RefFact, "category:"+strconv.FormatInt(it.CanonicalID, 10), hashJSON(it.CanonicalID), it.Name)
			row := map[string]any{"kind": "CATEGORY", "id": it.CanonicalID, "canonical_category_id": it.CanonicalID, "name": it.Name, "match_reason": it.MatchReason, "handle": h.Handle}
			if it.ProviderID != nil {
				row["provider_category_id"] = *it.ProviderID
			}
			matches = append(matches, row)
			handles = append(handles, h.Handle)
		}
	}
	clarify := len(matches) > 1
	return ToolOutput{Payload: map[string]any{"matches": matches, "needs_clarification": clarify}, Handles: handles}, nil
}

func (e *ToolExecutor) skuContexts(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, ids []int64) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Не могу достоверно ответить: данные для этого запроса сейчас недоступны."}, nil
	}
	api := e.api()
	var skus factualapi.BatchSKUResult
	var err error
	if e.TestBatches != nil && e.TestBatches.SKUs != nil {
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
			e.Counter.BatchSKU.Add(1)
		}
		skus, err = e.TestBatches.SKUs(ctx, ids)
	} else {
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
			e.Counter.BatchSKU.Add(1)
		}
		skus, err = api.BatchGetSKUs(ctx, frozen.Factual, ids)
	}
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Данные каталога недоступны."}, nil
	}
	var demand factualapi.BatchSKUDemandResult
	var stock factualapi.BatchSKUStockResult
	var price factualapi.BatchSKUPriceResult
	var ads factualapi.BatchSKUAdvertisingResult
	var demandFailed, adsFailed bool
	if e.TestBatches != nil {
		if e.TestBatches.Demand != nil {
			var derr error
			demand, derr = e.TestBatches.Demand(ctx, ids)
			demandFailed = derr != nil
		}
		if e.TestBatches.Stock != nil {
			stock, _ = e.TestBatches.Stock(ctx, ids)
		}
		if e.TestBatches.Price != nil {
			price, _ = e.TestBatches.Price(ctx, ids)
		}
		if e.TestBatches.Advertising != nil {
			var aerr error
			ads, aerr = e.TestBatches.Advertising(ctx, ids)
			adsFailed = aerr != nil
		}
	} else {
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
		}
		var derr error
		demand, derr = api.BatchGetSKUDemand(ctx, frozen.Factual, ids, factualapi.PeriodCurrent7D, nil)
		if derr != nil {
			demand = factualapi.BatchSKUDemandResult{}
			demandFailed = true
		}
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
		}
		stock, _ = api.BatchGetSKUStock(ctx, frozen.Factual, ids)
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
		}
		price, _ = api.BatchGetSKUPrice(ctx, frozen.Factual, ids)
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
		}
		var adsErr error
		ads, adsErr = api.BatchGetSKUAdvertising(ctx, frozen.Factual, ids, factualapi.PeriodCurrent7D, nil)
		if adsErr != nil {
			ads = factualapi.BatchSKUAdvertisingResult{}
			adsFailed = true
		}
	}
	items := []map[string]any{}
	handles := []string{}
	demandBy := map[int64]factualapi.BatchSKUDemandItem{}
	for _, d := range demand.Items {
		demandBy[d.ID] = d
	}
	stockBy := map[int64]factualapi.BatchSKUStockItem{}
	for _, d := range stock.Items {
		stockBy[d.ID] = d
	}
	priceBy := map[int64]factualapi.BatchSKUPriceItem{}
	for _, d := range price.Items {
		priceBy[d.ID] = d
	}
	adsBy := map[int64]factualapi.BatchSKUAdvertisingItem{}
	for _, d := range ads.Items {
		adsBy[d.ID] = d
	}
	for _, it := range skus.Items {
		row := map[string]any{"canonical_sku_id": it.ID, "status": it.Status}
		if it.SKU != nil {
			row["ozon_sku"] = it.SKU.OzonSKU
			row["name"] = it.SKU.Name
		}
		if d, ok := demandBy[it.ID]; ok && d.Demand != nil {
			row["ordered_units_7d"] = valueJSON(d.Demand.Demand.OrderedUnits)
			row["ordered_amount_7d"] = valueJSON(d.Demand.Demand.OrderedAmount)
		}
		if d, ok := stockBy[it.ID]; ok && d.Stock != nil {
			row["sellable_stock"] = valueJSON(d.Stock.SellableUnits)
		}
		if d, ok := priceBy[it.ID]; ok && d.Price != nil {
			row["price"] = d.Price
		}
		if d, ok := adsBy[it.ID]; ok && d.Advertising != nil {
			row["advertising"] = d.Advertising
		}
		h := reg.Register(RefFact, fmt.Sprintf("sku:%d", it.ID), hashJSON(row), fmt.Sprintf("SKU %d context", it.ID))
		row["handle"] = h.Handle
		items = append(items, row)
		handles = append(handles, h.Handle)
	}
	payload := map[string]any{"items": items, "batch": true}
	lim := ""
	if demandFailed {
		payload["sales_status"] = "UNAVAILABLE"
		lim = "Не могу достоверно ответить: данные продаж для этого запроса сейчас недоступны."
	}
	if adsFailed {
		payload["advertising_status"] = "UNAVAILABLE"
		if lim == "" {
			lim = "Рекламный контекст сейчас недоступен; ответ ограничен продажами и остатками."
		} else {
			lim += " Рекламный контекст также недоступен."
		}
	}
	return ToolOutput{Payload: payload, Handles: handles, Limitation: lim}, nil
}

func (e *ToolExecutor) categoryContexts(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, ids []int64) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	api := e.api()
	var res factualapi.BatchCategoryResult
	var err error
	if e.TestBatches != nil && e.TestBatches.Categories != nil {
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
			e.Counter.BatchCategory.Add(1)
		}
		res, err = e.TestBatches.Categories(ctx, ids)
	} else {
		if e.Counter != nil {
			e.Counter.Calls.Add(1)
			e.Counter.BatchCategory.Add(1)
		}
		res, err = api.BatchGetCategories(ctx, frozen.Factual, ids)
	}
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Данные категорий недоступны."}, nil
	}
	items := []map[string]any{}
	handles := []string{}
	for _, it := range res.Items {
		row := map[string]any{"id": it.ID, "status": it.Status}
		if it.Category != nil {
			row["name"] = it.Category.Name
			h := reg.Register(RefFact, fmt.Sprintf("category:%d", it.ID), hashJSON(it.Category.ID), it.Category.Name)
			row["handle"] = h.Handle
			handles = append(handles, h.Handle)
		}
		items = append(items, row)
	}
	return ToolOutput{Payload: map[string]any{"items": items, "batch": true}, Handles: handles}, nil
}

func (e *ToolExecutor) campaignContexts(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, ids []int64) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
		e.Counter.BatchSKU.Add(1)
	}
	res, err := api.BatchGetCampaigns(ctx, frozen.Factual, ids)
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Данные кампаний недоступны."}, nil
	}
	items := []map[string]any{}
	handles := []string{}
	for _, it := range res.Items {
		row := map[string]any{"id": it.ID, "status": it.Status}
		if it.Campaign != nil {
			row["campaign"] = it.Campaign
		}
		h := reg.Register(RefFact, fmt.Sprintf("campaign:%d", it.ID), hashJSON(it.ID), fmt.Sprintf("campaign %d", it.ID))
		row["handle"] = h.Handle
		items = append(items, row)
		handles = append(handles, h.Handle)
	}
	return ToolOutput{Payload: map[string]any{"items": items, "batch": true}, Handles: handles}, nil
}

func (e *ToolExecutor) accountDomain(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, domain string, args map[string]any) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Не могу достоверно ответить: данные сейчас недоступны."}, nil
	}
	std, explicit, lim, err := parsePeriod(frozen, args, e.Policy.MaxPeriodDays)
	if err != nil {
		return ToolOutput{ErrorCode: "PERIOD_LIMITED", Limitation: lim}, err
	}
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
	}
	var payload map[string]any
	switch domain {
	case "SALES":
		d, err := api.GetAccountDemand(ctx, frozen.Factual, std, explicit)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Не могу достоверно ответить: данные продаж для этого запроса сейчас недоступны."}, nil
		}
		payload = map[string]any{"ordered_units": valueJSON(d.OrderedUnits), "ordered_amount": valueJSON(d.OrderedAmount), "label_ordered_amount": "Заказано на сумму"}
	case "ADVERTISING":
		s, err := api.GetAdvertisingSpendShare(ctx, frozen.Factual, std, explicit)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Рекламные факты недоступны."}, nil
		}
		payload = map[string]any{"spend_share": s, "semantics": "paid_ad_spend / canonical ordered_amount"}
	case "FINANCE":
		f, err := api.GetFinanceAccountSummary(ctx, frozen.Factual, std, explicit)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Финансовые факты недоступны."}, nil
		}
		payload = map[string]any{"summary": f, "note": "profit/margin only if authoritative cost present"}
	case "FUNNEL":
		f, err := api.GetAccountFunnelPeriod(ctx, frozen.Factual, "", std, explicit)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Воронка недоступна."}, nil
		}
		payload = map[string]any{"funnel": f}
	case "SEARCH":
		page, err := api.ListSKUSearchSummary(ctx, frozen.Factual, std, explicit, factualapi.PageRequest{PageSize: 10})
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Поисковые факты недоступны."}, nil
		}
		payload = map[string]any{"items": page.Items, "limitation": "Top-N retrieval is not the complete search universe"}
	case "STOCK":
		page, err := api.ListSKUStock(ctx, frozen.Factual, factualapi.StockFilter{}, factualapi.PageRequest{PageSize: 10})
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Остатки недоступны."}, nil
		}
		payload = map[string]any{"items": page.Items, "note": "inbound ≠ sellable; UNKNOWN ≠ 0"}
	case "CATALOG":
		page, err := api.ListSKUs(ctx, frozen.Factual, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: 10})
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
		}
		payload = map[string]any{"sku_count_page": len(page.Items)}
	case "PRICE":
		page, err := api.ListSKUPrices(ctx, frozen.Factual, factualapi.PriceFilter{}, factualapi.PageRequest{PageSize: 10})
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
		}
		payload = map[string]any{"items": page.Items}
	case "PROMOTIONS":
		p, err := api.GetPromotionAccountSummary(ctx, frozen.Factual)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
		}
		payload = map[string]any{"summary": p}
	case "ACCOUNT_HEALTH":
		h, err := api.GetAccountHealthSummary(ctx, frozen.Factual)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
		}
		payload = map[string]any{"summary": h}
	case "SUPPLY":
		h, err := api.GetSupplyAccountSummary(ctx, frozen.Factual)
		if err != nil {
			return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
		}
		payload = map[string]any{"summary": h, "note": "inbound ≠ current sellable stock"}
	default:
		return ToolOutput{ErrorCode: "INVALID_ARGS"}, ErrMalformedToolArgs
	}
	h := reg.Register(RefFact, "domain:"+domain, hashJSON(payload), domain+" account facts")
	payload["handle"] = h.Handle
	if lim != "" {
		payload["period_limitation"] = lim
	}
	return ToolOutput{Payload: payload, Handles: []string{h.Handle}, Limitation: lim}, nil
}

func (e *ToolExecutor) listSKUMetrics(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, args map[string]any) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	std, explicit, lim, err := parsePeriod(frozen, args, e.Policy.MaxPeriodDays)
	if err != nil {
		return ToolOutput{ErrorCode: "PERIOD_LIMITED", Limitation: lim}, err
	}
	limit := intArg(args, "limit", 20)
	if limit > e.Policy.MaxRowsPerTool {
		limit = e.Policy.MaxRowsPerTool
	}
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
	}
	page, err := api.ListSKUDemand(ctx, frozen.Factual, std, explicit, factualapi.PageRequest{PageSize: limit})
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE", Limitation: "Метрики продаж недоступны."}, nil
	}
	h := reg.Register(RefFact, "list_sku_metrics", hashJSON(len(page.Items)), fmt.Sprintf("%d sku metric rows", len(page.Items)))
	return ToolOutput{Payload: map[string]any{"rows": page.Items, "handle": h.Handle, "ordered_amount_label": "Заказано на сумму"}, Handles: []string{h.Handle}, Limitation: lim}, nil
}

func (e *ToolExecutor) listCategoryMetrics(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, args map[string]any) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	std, explicit, lim, err := parsePeriod(frozen, args, e.Policy.MaxPeriodDays)
	if err != nil {
		return ToolOutput{ErrorCode: "PERIOD_LIMITED", Limitation: lim}, err
	}
	limit := intArg(args, "limit", 20)
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
	}
	page, err := api.ListCategoryDemand(ctx, frozen.Factual, std, explicit, factualapi.PageRequest{PageSize: limit})
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	h := reg.Register(RefFact, "list_category_metrics", hashJSON(len(page.Items)), "category metrics")
	return ToolOutput{Payload: map[string]any{"rows": page.Items, "handle": h.Handle}, Handles: []string{h.Handle}, Limitation: lim}, nil
}

func (e *ToolExecutor) listCampaignMetrics(ctx context.Context, frozen FrozenTurnContext, reg *GroundingRegistry, args map[string]any) (ToolOutput, error) {
	if !frozen.HasFactual {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	std, explicit, lim, err := parsePeriod(frozen, args, e.Policy.MaxPeriodDays)
	if err != nil {
		return ToolOutput{ErrorCode: "PERIOD_LIMITED", Limitation: lim}, err
	}
	limit := intArg(args, "limit", 20)
	api := e.api()
	if e.Counter != nil {
		e.Counter.Calls.Add(1)
	}
	page, err := api.ListCampaigns(ctx, frozen.Factual, factualapi.CampaignFilter{}, factualapi.PageRequest{PageSize: limit})
	if err != nil {
		return ToolOutput{ErrorCode: "FACTUAL_UNAVAILABLE"}, nil
	}
	h := reg.Register(RefFact, "list_campaign_metrics", hashJSON(len(page.Items)), "campaign metrics")
	_ = std
	_ = explicit
	return ToolOutput{Payload: map[string]any{"rows": page.Items, "handle": h.Handle}, Handles: []string{h.Handle}, Limitation: lim}, nil
}

func parsePeriod(frozen FrozenTurnContext, args map[string]any, maxDays int) (factualapi.StandardPeriod, *factualapi.Period, string, error) {
	period := stringArg(args, "period")
	if period == "" {
		period = "last_7d"
	}
	switch period {
	case "last_7d":
		return factualapi.PeriodCurrent7D, nil, "", nil
	case "last_30d":
		return factualapi.PeriodTrend30D, nil, "", nil
	case "analysis_day":
		return factualapi.PeriodAnalysisDay, nil, "", nil
	case "custom":
		fromS, toS := stringArg(args, "date_from"), stringArg(args, "date_to")
		from, err1 := time.Parse("2006-01-02", fromS)
		to, err2 := time.Parse("2006-01-02", toS)
		if err1 != nil || err2 != nil {
			return "", nil, "Некорректный период.", ErrMalformedToolArgs
		}
		days := int(to.Sub(from).Hours()/24) + 1
		if days > maxDays {
			return "", nil, fmt.Sprintf("Запрошенный период %d дней превышает лимит %d дней. Не могу ответить по полному диапазону.", days, maxDays), ErrPeriodLimited
		}
		p := factualapi.Period{From: from, To: to}
		return "", &p, "", nil
	default:
		return factualapi.PeriodCurrent7D, nil, "", nil
	}
}

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return def
	}
}

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

func int64Slice(v any, max int) ([]int64, error) {
	arr, ok := v.([]any)
	if !ok {
		if typed, ok := v.([]int64); ok {
			if len(typed) > max {
				return nil, ErrMalformedToolArgs
			}
			return typed, nil
		}
		return nil, ErrMalformedToolArgs
	}
	if len(arr) == 0 || len(arr) > max {
		return nil, ErrMalformedToolArgs
	}
	out := make([]int64, 0, len(arr))
	for _, x := range arr {
		switch t := x.(type) {
		case float64:
			out = append(out, int64(t))
		case int:
			out = append(out, int64(t))
		case int64:
			out = append(out, t)
		case json.Number:
			n, err := t.Int64()
			if err != nil {
				return nil, ErrMalformedToolArgs
			}
			out = append(out, n)
		default:
			return nil, ErrMalformedToolArgs
		}
	}
	return out, nil
}

func valueJSON[T any](v factualapi.Value[T]) map[string]any {
	out := map[string]any{"state": v.State, "reason_code": v.ReasonCode}
	if v.Value != nil {
		out["value"] = *v.Value
	}
	return out
}

func entityTextMatches(haystack, query string) bool {
	h := strings.ToLower(haystack)
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	if strings.Contains(h, q) {
		return true
	}
	if isAllDigits(q) && strings.Contains(h, q) {
		return true
	}
	for _, tok := range strings.Fields(q) {
		tok = strings.Trim(tok, ".,!?;:«»\"'")
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, "sku") {
			rest := strings.TrimPrefix(tok, "sku")
			rest = strings.Trim(rest, "-_#")
			if rest != "" && strings.Contains(h, rest) {
				return true
			}
		}
		if isAllDigits(tok) {
			if strings.Contains(h, tok) {
				return true
			}
			continue
		}
		if utf8.RuneCountInString(tok) < 4 {
			continue
		}
		if strings.Contains(h, tok) {
			return true
		}
	}
	return false
}
