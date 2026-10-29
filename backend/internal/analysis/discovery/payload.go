package discovery

import (
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
)

type PayloadV1 struct {
	Context      PayloadContext      `json:"context"`
	Windows      []PayloadWindow     `json:"windows"`
	Capabilities []PayloadCapability `json:"capabilities"`
	Account      PayloadAccount      `json:"account"`
	Categories   []PayloadCategory   `json:"categories"`
	Campaigns    []PayloadCampaign   `json:"campaigns"`
	SKUs         []PayloadSKU        `json:"skus"`
	EvidenceMenu []EvidenceMenuItem  `json:"evidence_menu"`
}

type PayloadContext struct {
	AnalysisDate     string `json:"analysis_date"`
	AnalysisCutoffAt string `json:"analysis_cutoff_at"`
	BusinessTimezone string `json:"business_timezone"`
}

type PayloadDateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PayloadWindow struct {
	Domain     string            `json:"domain"`
	FactFamily string            `json:"fact_family,omitempty"`
	Current    *PayloadDateRange `json:"current,omitempty"`
	Previous   *PayloadDateRange `json:"previous,omitempty"`
	LagDays    *int64            `json:"lag_days,omitempty"`
	Readiness  string            `json:"readiness,omitempty"`
}

type PayloadCapability struct {
	Key         string   `json:"key"`
	Readiness   string   `json:"readiness"`
	Limitations []string `json:"limitations"`
}

type PayloadMoneyComparison struct {
	Current    *string `json:"current,omitempty"`
	Previous   *string `json:"previous,omitempty"`
	Delta      *string `json:"delta,omitempty"`
	PctDelta   *string `json:"pct_delta,omitempty"`
	FactHandle string  `json:"fact_handle"`
}

type PayloadCountComparison struct {
	Current    *int64  `json:"current,omitempty"`
	Previous   *int64  `json:"previous,omitempty"`
	Delta      *int64  `json:"delta,omitempty"`
	PctDelta   *string `json:"pct_delta,omitempty"`
	FactHandle string  `json:"fact_handle"`
}

type PayloadScalar struct {
	Value      *string `json:"value,omitempty"`
	FactHandle string  `json:"fact_handle"`
}

type PayloadCountValue struct {
	Value      *int64 `json:"value,omitempty"`
	FactHandle string `json:"fact_handle"`
}

type PayloadSales struct {
	State             string                  `json:"state"`
	Limitations       []string                `json:"limitations,omitempty"`
	OrderedAmount     *PayloadMoneyComparison `json:"ordered_amount,omitempty"`
	OrderedUnits      *PayloadCountComparison `json:"ordered_units,omitempty"`
	AnalysisDayAmount *PayloadMoneyComparison `json:"analysis_day_amount,omitempty"`
	AnalysisDayUnits  *PayloadCountComparison `json:"analysis_day_units,omitempty"`
}

type PayloadFinance struct {
	State       string                  `json:"state"`
	Limitations []string                `json:"limitations,omitempty"`
	NetAccrual  *PayloadMoneyComparison `json:"net_accrual,omitempty"`
}

type PayloadAds struct {
	State                 string                  `json:"state"`
	Limitations           []string                `json:"limitations,omitempty"`
	PaidSpend             *PayloadMoneyComparison `json:"paid_spend,omitempty"`
	AttributedOrderAmount *PayloadMoneyComparison `json:"attributed_order_amount,omitempty"`
}

type PayloadPrice struct {
	State              string                  `json:"state"`
	Limitations        []string                `json:"limitations,omitempty"`
	ObservedPrice      *PayloadScalar          `json:"observed_price,omitempty"`
	LastObservedChange *PayloadMoneyComparison `json:"last_observed_price_change,omitempty"`
}

type PayloadStock struct {
	State       string             `json:"state"`
	Limitations []string           `json:"limitations,omitempty"`
	Current     *PayloadCountValue `json:"current_state,omitempty"`
	Coverage    *PayloadScalar     `json:"coverage,omitempty"`
}

type PayloadFunnel struct {
	State       string                  `json:"state"`
	Limitations []string                `json:"limitations,omitempty"`
	Impressions *PayloadCountComparison `json:"impressions,omitempty"`
}

type PayloadSearch struct {
	State           string         `json:"state"`
	Limitations     []string       `json:"limitations,omitempty"`
	AveragePosition *PayloadScalar `json:"provider_average_position,omitempty"`
}

type PayloadSupply struct {
	State       string             `json:"state"`
	Limitations []string           `json:"limitations,omitempty"`
	OpenInbound *PayloadCountValue `json:"open_inbound,omitempty"`
}

type PayloadAccount struct {
	Entity  PackEntity      `json:"entity"`
	Sales   *PayloadSales   `json:"sales,omitempty"`
	Finance *PayloadFinance `json:"finance,omitempty"`
}

type PayloadCategory struct {
	Entity PackEntity    `json:"entity"`
	Name   string        `json:"name,omitempty"`
	Sales  *PayloadSales `json:"sales,omitempty"`
}

type PayloadSKU struct {
	Entity      PackEntity     `json:"entity"`
	Name        string         `json:"name,omitempty"`
	OfferID     string         `json:"offer_id,omitempty"`
	Sales       *PayloadSales  `json:"sales,omitempty"`
	Stock       *PayloadStock  `json:"stock,omitempty"`
	Price       *PayloadPrice  `json:"price,omitempty"`
	Advertising *PayloadAds    `json:"advertising,omitempty"`
	Funnel      *PayloadFunnel `json:"funnel,omitempty"`
	Search      *PayloadSearch `json:"search,omitempty"`
	Supply      *PayloadSupply `json:"supply,omitempty"`
}

type PayloadCampaign struct {
	Entity      PackEntity  `json:"entity"`
	Title       string      `json:"title,omitempty"`
	Performance *PayloadAds `json:"performance,omitempty"`
}

func ProjectPayload(detail landscape.LandscapeSet, menu []EvidenceMenuItem, handles FactHandleMapV1) PayloadV1 {
	tz := detail.Context.BusinessTimezone
	if tz == "" {
		tz = "Europe/Moscow"
	}
	p := PayloadV1{
		Context: PayloadContext{
			AnalysisDate:     time.Time(detail.Context.AnalysisDate).Format("2006-01-02"),
			AnalysisCutoffAt: detail.Context.AnalysisCutoffAt.UTC().Format(time.RFC3339),
			BusinessTimezone: tz,
		},
		Windows:      projectWindows(detail),
		Capabilities: projectCapabilities(detail),
		Account:      projectAccount(detail.Account, handles),
		EvidenceMenu: menu,
	}
	for _, c := range detail.Categories {
		id := strconv.FormatInt(c.Identity.CategoryID, 10)
		ent := PackEntity{Kind: KindCategory, ID: id}
		p.Categories = append(p.Categories, PayloadCategory{
			Entity: ent, Name: c.Identity.Name,
			Sales: projectSales(ent, c.Sales, c.Identity.IsLeaf, handles),
		})
	}
	for _, c := range detail.Campaigns {
		id := strconv.FormatInt(c.Identity.CampaignID, 10)
		ent := PackEntity{Kind: KindCampaign, ID: id}
		p.Campaigns = append(p.Campaigns, PayloadCampaign{
			Entity: ent, Title: c.Identity.Title,
			Performance: projectCampaignPerf(ent, c.Performance, handles),
		})
	}
	for _, s := range detail.SKUs {
		id := strconv.FormatInt(s.Identity.CanonicalSKUID, 10)
		ent := PackEntity{Kind: KindSKU, ID: id}
		p.SKUs = append(p.SKUs, PayloadSKU{
			Entity: ent, Name: s.Identity.Name, OfferID: s.Identity.OfferID,
			Sales:       projectSKUSales(ent, s.Sales, handles),
			Stock:       projectSKUStock(ent, s.Stock, handles),
			Price:       projectSKUPrice(ent, s.Price, handles),
			Advertising: projectSKUAds(ent, s.Advertising, handles),
			Funnel:      projectSKUFunnel(ent, s.Funnel, handles),
			Search:      projectSKUSearch(ent, s.Search, handles),
			Supply:      projectSKUSupply(ent, s.Supply, handles),
		})
	}
	if p.Categories == nil {
		p.Categories = []PayloadCategory{}
	}
	if p.Campaigns == nil {
		p.Campaigns = []PayloadCampaign{}
	}
	if p.SKUs == nil {
		p.SKUs = []PayloadSKU{}
	}
	if p.EvidenceMenu == nil {
		p.EvidenceMenu = []EvidenceMenuItem{}
	}
	return p
}

func projectWindows(detail landscape.LandscapeSet) []PayloadWindow {
	out := make([]PayloadWindow, 0, len(detail.DomainWindows))
	for _, w := range detail.DomainWindows {
		pw := PayloadWindow{Domain: w.Domain, FactFamily: w.FactFamily, LagDays: w.LagDays}
		if w.Current != nil {
			pw.Current = &PayloadDateRange{
				From: time.Time(w.Current.From).Format("2006-01-02"),
				To:   time.Time(w.Current.To).Format("2006-01-02"),
			}
		}
		if w.Previous != nil {
			pw.Previous = &PayloadDateRange{
				From: time.Time(w.Previous.From).Format("2006-01-02"),
				To:   time.Time(w.Previous.To).Format("2006-01-02"),
			}
		}
		out = append(out, pw)
	}
	return out
}

func projectCapabilities(detail landscape.LandscapeSet) []PayloadCapability {
	out := make([]PayloadCapability, 0, len(detail.GlobalCapabilities))
	for _, c := range detail.GlobalCapabilities {
		item := PayloadCapability{Key: c.Key, Readiness: c.FinalState, Limitations: []string{}}
		if c.FinalState != "READY" && c.ReasonCode != "" {
			item.Limitations = append(item.Limitations, c.ReasonCode)
		}
		out = append(out, item)
	}
	return out
}

func projectAccount(a landscape.AccountLandscape, handles FactHandleMapV1) PayloadAccount {
	ent := PackEntity{Kind: KindAccount, ID: "account"}
	out := PayloadAccount{Entity: ent}
	if a.Sales != nil && usableBlock(a.Sales.State) {
		out.Sales = &PayloadSales{
			State: string(a.Sales.State), Limitations: limitationCodes(a.Sales.Limitations),
			OrderedAmount: moneyCmp(ent, "sales.ordered_amount_comparison", a.Sales.OrderedAmount, handles),
			OrderedUnits:  countCmp(ent, "sales.ordered_units_comparison", a.Sales.OrderedUnits, handles),
		}
		if a.Sales.AnalysisDayAmount != nil {
			out.Sales.AnalysisDayAmount = moneyCmp(ent, "sales.analysis_day_amount_comparison", *a.Sales.AnalysisDayAmount, handles)
		}
		if a.Sales.AnalysisDayUnits != nil {
			out.Sales.AnalysisDayUnits = countCmp(ent, "sales.analysis_day_units_comparison", *a.Sales.AnalysisDayUnits, handles)
		}
		if out.Sales.OrderedAmount == nil && out.Sales.OrderedUnits == nil && out.Sales.AnalysisDayAmount == nil && out.Sales.AnalysisDayUnits == nil {
			out.Sales = nil
		}
	}
	if a.Finance != nil && usableBlock(a.Finance.State) {
		out.Finance = &PayloadFinance{
			State: string(a.Finance.State), Limitations: limitationCodes(a.Finance.Limitations),
			NetAccrual: moneyCmp(ent, "finance.net_accrual_comparison", a.Finance.NetAccrual, handles),
		}
		if out.Finance.NetAccrual == nil {
			out.Finance = nil
		}
	}
	return out
}

func projectSales(ent PackEntity, sales *landscape.CategorySalesBlock, _ bool, handles FactHandleMapV1) *PayloadSales {
	if sales == nil || !usableBlock(sales.State) {
		return nil
	}
	out := &PayloadSales{
		State: string(sales.State), Limitations: limitationCodes(sales.Limitations),
		OrderedAmount: moneyCmp(ent, "sales.ordered_amount_comparison", sales.OrderedAmount, handles),
		OrderedUnits:  countCmp(ent, "sales.ordered_units_comparison", sales.OrderedUnits, handles),
	}
	if out.OrderedAmount == nil && out.OrderedUnits == nil {
		return nil
	}
	return out
}

func projectSKUSales(ent PackEntity, sales *landscape.SKUSalesBlock, handles FactHandleMapV1) *PayloadSales {
	if sales == nil || !usableBlock(sales.State) {
		return nil
	}
	out := &PayloadSales{
		State: string(sales.State), Limitations: limitationCodes(sales.Limitations),
		OrderedAmount: moneyCmp(ent, "sales.ordered_amount_comparison", sales.OrderedAmount, handles),
		OrderedUnits:  countCmp(ent, "sales.ordered_units_comparison", sales.OrderedUnits, handles),
	}
	if out.OrderedAmount == nil && out.OrderedUnits == nil {
		return nil
	}
	return out
}

func projectSKUStock(ent PackEntity, stock *landscape.SKUStockBlock, handles FactHandleMapV1) *PayloadStock {
	if stock == nil || !usableBlock(stock.State) {
		return nil
	}
	out := &PayloadStock{State: string(stock.State), Limitations: limitationCodes(stock.Limitations)}
	if e, ok := handles.Lookup(ent.Kind, ent.ID, "stock.current_state"); ok {
		out.Current = &PayloadCountValue{Value: stock.SellableUnits, FactHandle: e.Handle}
	}
	if e, ok := handles.Lookup(ent.Kind, ent.ID, "stock.coverage"); ok {
		var v *string
		if stock.CoverageDays7D != nil {
			s := string(*stock.CoverageDays7D)
			v = &s
		}
		out.Coverage = &PayloadScalar{Value: v, FactHandle: e.Handle}
	}
	if out.Current == nil && out.Coverage == nil {
		return nil
	}
	return out
}

func projectSKUPrice(ent PackEntity, price *landscape.SKUPriceBlock, handles FactHandleMapV1) *PayloadPrice {
	if price == nil || !usableBlock(price.State) {
		return nil
	}
	out := &PayloadPrice{State: string(price.State), Limitations: limitationCodes(price.Limitations)}
	if e, ok := handles.Lookup(ent.Kind, ent.ID, "price.observed_price"); ok {
		var v *string
		if price.CurrentPrice != nil {
			s := string(price.CurrentPrice.Amount)
			v = &s
		}
		out.ObservedPrice = &PayloadScalar{Value: v, FactHandle: e.Handle}
	}
	if price.LastObservedChange != nil {
		out.LastObservedChange = moneyCmp(ent, "price.last_observed_price_change", *price.LastObservedChange, handles)
	}
	if out.ObservedPrice == nil && out.LastObservedChange == nil {
		return nil
	}
	return out
}

func projectSKUAds(ent PackEntity, ads *landscape.SKUAdsBlock, handles FactHandleMapV1) *PayloadAds {
	if ads == nil || !usableBlock(ads.State) {
		return nil
	}
	out := &PayloadAds{State: string(ads.State), Limitations: limitationCodes(ads.Limitations)}
	if ads.Traffic != nil && usableBlock(ads.Traffic.State) {
		out.PaidSpend = moneyCmp(ent, "advertising.traffic.paid_spend_comparison", ads.Traffic.PaidSpend, handles)
	}
	if ads.Attribution != nil && usableBlock(ads.Attribution.State) {
		out.AttributedOrderAmount = moneyCmp(ent, "advertising.attribution.attributed_order_amount_comparison", ads.Attribution.AttributedOrderAmount, handles)
	}
	if out.PaidSpend == nil && out.AttributedOrderAmount == nil {
		return nil
	}
	return out
}

func projectSKUFunnel(ent PackEntity, funnel *landscape.SKUFunnelBlock, handles FactHandleMapV1) *PayloadFunnel {
	if funnel == nil || !usableBlock(funnel.State) || funnel.SearchSurface == nil || funnel.SearchSurface.Impressions == nil {
		return nil
	}
	imp := countCmp(ent, "funnel.search.impressions_comparison", *funnel.SearchSurface.Impressions, handles)
	if imp == nil {
		return nil
	}
	return &PayloadFunnel{State: string(funnel.State), Limitations: limitationCodes(funnel.Limitations), Impressions: imp}
}

func projectSKUSearch(ent PackEntity, search *landscape.SKUSearchBlock, handles FactHandleMapV1) *PayloadSearch {
	if search == nil || !usableBlock(search.State) {
		return nil
	}
	e, ok := handles.Lookup(ent.Kind, ent.ID, "search.provider_average_position")
	if !ok {
		return nil
	}
	var v *string
	if search.ProviderAveragePosition != nil {
		s := string(*search.ProviderAveragePosition)
		v = &s
	}
	return &PayloadSearch{State: string(search.State), Limitations: limitationCodes(search.Limitations), AveragePosition: &PayloadScalar{Value: v, FactHandle: e.Handle}}
}

func projectSKUSupply(ent PackEntity, supply *landscape.SKUSupplyBlock, handles FactHandleMapV1) *PayloadSupply {
	if supply == nil || !usableBlock(supply.State) {
		return nil
	}
	e, ok := handles.Lookup(ent.Kind, ent.ID, "supply.open_inbound")
	if !ok {
		return nil
	}
	return &PayloadSupply{State: string(supply.State), Limitations: limitationCodes(supply.Limitations), OpenInbound: &PayloadCountValue{Value: supply.OpenInboundDeclaredUnits, FactHandle: e.Handle}}
}

func projectCampaignPerf(ent PackEntity, perf *landscape.CampaignPerfBlock, handles FactHandleMapV1) *PayloadAds {
	if perf == nil || !usableBlock(perf.State) {
		return nil
	}
	out := &PayloadAds{State: string(perf.State), Limitations: limitationCodes(perf.Limitations)}
	if perf.Traffic != nil && usableBlock(perf.Traffic.State) {
		out.PaidSpend = moneyCmp(ent, "advertising.traffic.paid_spend_comparison", perf.Traffic.PaidSpend, handles)
	}
	if perf.Attribution != nil && usableBlock(perf.Attribution.State) {
		out.AttributedOrderAmount = moneyCmp(ent, "advertising.attribution.attributed_order_amount_comparison", perf.Attribution.AttributedOrderAmount, handles)
	}
	if out.PaidSpend == nil && out.AttributedOrderAmount == nil {
		return nil
	}
	return out
}

func moneyCmp(ent PackEntity, path string, m landscape.MoneyComparison, handles FactHandleMapV1) *PayloadMoneyComparison {
	if !moneyPresent(m) {
		return nil
	}
	e, ok := handles.Lookup(ent.Kind, ent.ID, path)
	if !ok {
		return nil
	}
	out := &PayloadMoneyComparison{FactHandle: e.Handle}
	if m.Current != nil {
		s := string(m.Current.Amount)
		out.Current = &s
	}
	if m.Previous != nil {
		s := string(m.Previous.Amount)
		out.Previous = &s
	}
	if m.AbsoluteDelta != nil {
		s := string(m.AbsoluteDelta.Amount)
		out.Delta = &s
	}
	if m.PctDelta != nil {
		s := string(*m.PctDelta)
		out.PctDelta = &s
	}
	return out
}

func countCmp(ent PackEntity, path string, c landscape.CountComparison, handles FactHandleMapV1) *PayloadCountComparison {
	if !countPresent(c) {
		return nil
	}
	e, ok := handles.Lookup(ent.Kind, ent.ID, path)
	if !ok {
		return nil
	}
	out := &PayloadCountComparison{FactHandle: e.Handle, Current: c.Current, Previous: c.Previous, Delta: c.AbsoluteDelta}
	if c.PctDelta != nil {
		s := string(*c.PctDelta)
		out.PctDelta = &s
	}
	return out
}

func moneyPresent(m landscape.MoneyComparison) bool {
	return m.Current != nil || m.Previous != nil
}

func countPresent(c landscape.CountComparison) bool {
	return c.Current != nil || c.Previous != nil
}

func limitationCodes(ls []landscape.Limitation) []string {
	if len(ls) == 0 {
		return nil
	}
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		if l.Code != "" {
			out = append(out, l.Code)
		}
	}
	return out
}

// VisibleFact is one AI-visible grounded object (structural metadata excluded).
type VisibleFact struct {
	Entity PackEntity
	Path   string
	Handle string
}

func CollectVisibleFactHandles(p PayloadV1) []VisibleFact {
	var out []VisibleFact
	addMoney := func(ent PackEntity, path string, m *PayloadMoneyComparison) {
		if m != nil {
			out = append(out, VisibleFact{Entity: ent, Path: path, Handle: m.FactHandle})
		}
	}
	addCount := func(ent PackEntity, path string, c *PayloadCountComparison) {
		if c != nil {
			out = append(out, VisibleFact{Entity: ent, Path: path, Handle: c.FactHandle})
		}
	}
	addScalar := func(ent PackEntity, path string, s *PayloadScalar) {
		if s != nil {
			out = append(out, VisibleFact{Entity: ent, Path: path, Handle: s.FactHandle})
		}
	}
	addCV := func(ent PackEntity, path string, s *PayloadCountValue) {
		if s != nil {
			out = append(out, VisibleFact{Entity: ent, Path: path, Handle: s.FactHandle})
		}
	}
	collectSales := func(ent PackEntity, s *PayloadSales) {
		if s == nil {
			return
		}
		addMoney(ent, "sales.ordered_amount_comparison", s.OrderedAmount)
		addCount(ent, "sales.ordered_units_comparison", s.OrderedUnits)
		addMoney(ent, "sales.analysis_day_amount_comparison", s.AnalysisDayAmount)
		addCount(ent, "sales.analysis_day_units_comparison", s.AnalysisDayUnits)
	}
	collectSales(p.Account.Entity, p.Account.Sales)
	if p.Account.Finance != nil {
		addMoney(p.Account.Entity, "finance.net_accrual_comparison", p.Account.Finance.NetAccrual)
	}
	for _, c := range p.Categories {
		collectSales(c.Entity, c.Sales)
	}
	for _, c := range p.Campaigns {
		if c.Performance != nil {
			addMoney(c.Entity, "advertising.traffic.paid_spend_comparison", c.Performance.PaidSpend)
			addMoney(c.Entity, "advertising.attribution.attributed_order_amount_comparison", c.Performance.AttributedOrderAmount)
		}
	}
	for _, s := range p.SKUs {
		collectSales(s.Entity, s.Sales)
		if s.Stock != nil {
			addCV(s.Entity, "stock.current_state", s.Stock.Current)
			addScalar(s.Entity, "stock.coverage", s.Stock.Coverage)
		}
		if s.Price != nil {
			addScalar(s.Entity, "price.observed_price", s.Price.ObservedPrice)
			addMoney(s.Entity, "price.last_observed_price_change", s.Price.LastObservedChange)
		}
		if s.Advertising != nil {
			addMoney(s.Entity, "advertising.traffic.paid_spend_comparison", s.Advertising.PaidSpend)
			addMoney(s.Entity, "advertising.attribution.attributed_order_amount_comparison", s.Advertising.AttributedOrderAmount)
		}
		if s.Funnel != nil {
			addCount(s.Entity, "funnel.search.impressions_comparison", s.Funnel.Impressions)
		}
		if s.Search != nil {
			addScalar(s.Entity, "search.provider_average_position", s.Search.AveragePosition)
		}
		if s.Supply != nil {
			addCV(s.Entity, "supply.open_inbound", s.Supply.OpenInbound)
		}
	}
	return out
}
