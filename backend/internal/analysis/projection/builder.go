package projection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/funnelregistry"
)

func Build(ctx context.Context, in Input) (Built, error) {
	q := map[string]int{}
	date := in.AnalysisDate.UTC().Format("2006-01-02")
	cut := in.CutoffAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	stockRows, campRows, focusRows, stockBands, err := buildRows(ctx, in, q)
	if err != nil {
		return Built{}, err
	}
	ov, err := buildOverview(ctx, in, date, cut, focusRows, q)
	if err != nil {
		return Built{}, err
	}
	fn, err := buildFunnel(ctx, in, date, q)
	if err != nil {
		return Built{}, err
	}
	visible := visibleRefs(in.Plan)
	top := append([]string{}, visible...)
	if len(top) > 3 {
		top = top[:3]
	}
	ov.TopActions = top
	findings := findingViews(in.Validated)
	stock := DomainPageDTO{
		SchemaVersion: StockV2, State: "READY", CoverageBands: stockBands,
		FindingRefs: in.Validated.FindingRefs, Findings: filterFindings(findings, deep.DomainStock),
		RecommendationRefs: in.Validated.RecommendationRefs, ActionLocalRefs: visible, GroupLocalRefs: groupRefs(in.Plan),
	}
	adShare, yShare := buildAdShareKPIs(ctx, in, q)
	ad := DomainPageDTO{
		SchemaVersion: AdvertisingV2, State: adDomainState(adShare),
		AccountAdSpendShare: adShare, YesterdayAdSpendShare: yShare,
		FindingRefs: in.Validated.FindingRefs, Findings: filterFindings(findings, deep.DomainAdvertising),
		RecommendationRefs: in.Validated.RecommendationRefs, ActionLocalRefs: visible, GroupLocalRefs: groupRefs(in.Plan),
	}
	if adShare != nil && adShare.Status != "READY" && adShare.Status != "KNOWN" {
		ad.ReasonCode = adShare.ReasonCode
	}
	acts := ActionsSnapshotDTO{SchemaVersion: ActionsSnapshotV2, Groups: toGroupViews(in.Plan), Singles: singles(in.Plan)}
	chat := ChatContextDTO{
		SchemaVersion: ChatContextV1, ExecutiveSummary: in.Validated.Output.ExecutiveSummary.Summary,
		FindingRefs: in.Validated.FindingRefs, RecommendationRefs: in.Validated.RecommendationRefs,
		ActionLocalRefs: visible, Limitations: in.Validated.Output.ExecutiveSummary.Limitations,
		FactHandles: firstHandles(in.Validated), AnalysisDate: date,
	}
	b := Built{Overview: ov, Funnel: fn, Stock: stock, Advertising: ad, Actions: acts, Chat: chat,
		StockRows: stockRows, CampaignRows: campRows, FocusRows: focusRows, QueryCounts: q, Bytes: map[string][]byte{}}
	put := func(k string, v any) {
		raw, _ := canonicaljson.Marshal(v)
		b.Bytes[k] = raw
	}
	put(KeyOverview, ov)
	put(KeyFunnel, fn)
	put(KeyStock, stock)
	put(KeyAdvertising, ad)
	put(KeyActionsSnapshot, acts)
	put(KeyChatContext, chat)
	put(KeyStockSKURows, stockRows)
	put(KeyAdCampaignRows, campRows)
	put(KeySKUFocusRows, focusRows)
	hashes := map[string]string{}
	for k, raw := range b.Bytes {
		sum := sha256.Sum256(raw)
		hashes[k] = hex.EncodeToString(sum[:])
	}
	b.Manifest = Manifest{SchemaVersion: ManifestV2, PolicyVersion: PolicyV2, Hashes: hashes, QueryCounts: q}
	mb, _ := canonicaljson.Marshal(b.Manifest)
	b.Bytes["MANIFEST"] = mb
	return b, nil
}

func buildOverview(ctx context.Context, in Input, date, cut string, focus []map[string]any, q map[string]int) (OverviewV1DTO, error) {
	kpis := []KPI{
		{Key: "ORDERED_AMOUNT", Status: "MISSING"},
		{Key: "ORDERED_UNITS", Status: "MISSING"},
		{Key: "AD_SHARE", Status: "MISSING"},
	}
	periods := []PeriodWindow{{Key: "current_7d"}, {Key: "previous_7d"}, {Key: "trend_30d"}}
	if in.API != nil {
		q["GetAccountDemand"] += 2
		cur, err1 := in.API.GetAccountDemand(ctx, in.Sealed, factualapi.PeriodCurrent7D, nil)
		prev, err2 := in.API.GetAccountDemand(ctx, in.Sealed, factualapi.PeriodPrevious7D, nil)
		if err1 == nil {
			fillDemandKPI(&kpis[0], cur.OrderedAmount, prevIf(err2 == nil, prev.OrderedAmount), "MONEY")
			fillUnitsKPI(&kpis[1], cur.OrderedUnits, prevIfUnits(err2 == nil, prev.OrderedUnits))
		}
		q["GetAccountDemandSeries"]++
		if series, err := in.API.GetAccountDemandSeries(ctx, in.Sealed, factualapi.PeriodTrend30D, nil); err == nil {
			kpis[0].Trend = demandTrend(series)
			kpis[1].Trend = unitsTrend(series)
		}
		q["GetAdvertisingSpendShare"]++
		if share, err := in.API.GetAdvertisingSpendShare(ctx, in.Sealed, factualapi.PeriodCurrent7D, nil); err == nil {
			fillShareKPI(&kpis[2], share)
		}
	}
	preview := focus
	if len(preview) > 8 {
		preview = preview[:8]
	}
	takeaways := make([]TakeawayView, 0, len(in.Validated.Output.ExecutiveSummary.KeyTakeaways))
	for i, t := range in.Validated.Output.ExecutiveSummary.KeyTakeaways {
		if i >= 3 {
			break
		}
		takeaways = append(takeaways, TakeawayView{Statement: t.Statement, PrimaryDomain: t.PrimaryDomain})
	}
	return OverviewV1DTO{
		SchemaVersion: OverviewV2, SellerAccountID: in.SellerID, AnalysisDate: date, AnalysisCutoffAt: cut,
		FactualUniverseHash: in.UniverseHash, Periods: periods, KPIs: kpis,
		ExecutiveHeadline: in.Validated.Output.ExecutiveSummary.Headline,
		ExecutiveSummary:  in.Validated.Output.ExecutiveSummary.Summary,
		Takeaways:         takeaways, Findings: findingViews(in.Validated),
		Recommendations: in.Validated.RecommendationRefs, SKUFocusPreview: preview,
	}, nil
}

func prevIf(ok bool, v factualapi.Value[factualapi.Money]) *factualapi.Value[factualapi.Money] {
	if !ok {
		return nil
	}
	return &v
}

func prevIfUnits(ok bool, v factualapi.Value[int64]) *factualapi.Value[int64] {
	if !ok {
		return nil
	}
	return &v
}

func fillDemandKPI(k *KPI, cur factualapi.Value[factualapi.Money], prev *factualapi.Value[factualapi.Money], unit string) {
	k.Unit = unit
	if cur.State != factualapi.ValueKnown || cur.Value == nil {
		k.Status = "MISSING"
		k.ReasonCode = cur.ReasonCode
		k.Amount = nil
		return
	}
	s := cur.Value.Amount.String()
	k.Status = "READY"
	k.Amount = &s
	k.Current = &s
	k.Unit = cur.Value.CurrencyCode
	if k.Unit == "" {
		k.Unit = unit
	}
	if prev != nil && prev.State == factualapi.ValueKnown && prev.Value != nil {
		p := prev.Value.Amount.String()
		k.Previous = &p
		d := cur.Value.Amount.SubString(prev.Value.Amount)
		k.Delta = &d
		k.DeltaUnit = k.Unit
	}
}

func fillUnitsKPI(k *KPI, cur factualapi.Value[int64], prev *factualapi.Value[int64]) {
	k.Unit = "UNITS"
	if cur.State != factualapi.ValueKnown || cur.Value == nil {
		k.Status = "MISSING"
		k.ReasonCode = cur.ReasonCode
		k.Amount = nil
		return
	}
	s := strconv.FormatInt(*cur.Value, 10)
	k.Status = "READY"
	k.Amount = &s
	k.Current = &s
	if prev != nil && prev.State == factualapi.ValueKnown && prev.Value != nil {
		p := strconv.FormatInt(*prev.Value, 10)
		k.Previous = &p
		d := strconv.FormatInt(*cur.Value-*prev.Value, 10)
		k.Delta = &d
		k.DeltaUnit = "UNITS"
	}
}

func fillShareKPI(k *KPI, share factualapi.AdvertisingSpendShare) {
	k.Unit = "RATIO"
	st := share.SpendSharePct
	if st.State == factualapi.ValueNoDenominator || share.SalesOrderedAmount.State != factualapi.ValueKnown {
		k.Status = "UNAVAILABLE"
		k.ReasonCode = firstNonEmpty(st.ReasonCode, "NO_DENOMINATOR")
		k.Amount = nil
		return
	}
	if st.State != factualapi.ValueKnown || st.Value == nil {
		k.Status = "MISSING"
		k.ReasonCode = st.ReasonCode
		k.Amount = nil
		return
	}
	s := st.Value.String()
	k.Status = "READY"
	k.Amount = &s
	k.Current = &s
}

func demandTrend(series []factualapi.DemandPoint) []TrendPoint {
	out := make([]TrendPoint, 0, len(series))
	for _, p := range series {
		tp := TrendPoint{Date: p.BusinessDate.UTC().Format("2006-01-02"), State: string(p.OrderedAmount.State)}
		if p.OrderedAmount.State == factualapi.ValueKnown && p.OrderedAmount.Value != nil {
			s := p.OrderedAmount.Value.Amount.String()
			tp.Amount = &s
		}
		out = append(out, tp)
	}
	return out
}

func unitsTrend(series []factualapi.DemandPoint) []TrendPoint {
	out := make([]TrendPoint, 0, len(series))
	for _, p := range series {
		tp := TrendPoint{Date: p.BusinessDate.UTC().Format("2006-01-02"), State: string(p.OrderedUnits.State)}
		if p.OrderedUnits.State == factualapi.ValueKnown && p.OrderedUnits.Value != nil {
			s := strconv.FormatInt(*p.OrderedUnits.Value, 10)
			tp.Amount = &s
		}
		out = append(out, tp)
	}
	return out
}

func buildAdShareKPIs(ctx context.Context, in Input, q map[string]int) (seven *KPI, yesterday *KPI) {
	seven = &KPI{Key: "account_ad_spend_share", Status: "MISSING"}
	yesterday = &KPI{Key: "yesterday_account_ad_spend_share", Status: "MISSING"}
	if in.API == nil {
		return seven, yesterday
	}
	q["GetAdvertisingSpendShare"] += 2
	if share, err := in.API.GetAdvertisingSpendShare(ctx, in.Sealed, factualapi.PeriodCurrent7D, nil); err == nil {
		fillShareKPI(seven, share)
		seven.Key = "account_ad_spend_share"
	}
	if share, err := in.API.GetAdvertisingSpendShare(ctx, in.Sealed, factualapi.PeriodPreviousDay, nil); err == nil {
		fillShareKPI(yesterday, share)
		yesterday.Key = "yesterday_account_ad_spend_share"
	}
	return seven, yesterday
}

func adDomainState(k *KPI) string {
	if k == nil {
		return "UNAVAILABLE"
	}
	switch k.Status {
	case "READY":
		return "READY"
	case "UNAVAILABLE":
		return "UNAVAILABLE"
	default:
		return "UNAVAILABLE"
	}
}

func buildFunnel(ctx context.Context, in Input, date string, q map[string]int) (FunnelV1DTO, error) {
	out := FunnelV1DTO{
		SchemaVersion: FunnelV2, State: "UNAVAILABLE", Status: "UNAVAILABLE",
		Period: FunnelPeriods{
			Current:  PeriodWindow{Key: "current_7d"},
			Previous: PeriodWindow{Key: "previous_7d"},
		},
		DataAsOf: date, Stages: []FunnelStage{}, Transitions: []FunnelTransition{},
		Limitations: []string{}, RelatedActions: visibleRefs(in.Plan),
	}
	if in.API == nil {
		out.ReasonCode = "METRICS_UNAVAILABLE"
		return out, nil
	}
	q["GetFunnelCapabilities"]++
	caps, err := in.API.GetFunnelCapabilities(ctx, in.Sealed)
	if err != nil {
		out.State, out.Status, out.ReasonCode = "FAILED", "FAILED", "SOURCE_FAILED"
		return out, nil
	}
	avail := map[string]bool{}
	pendingOnly := true
	for _, m := range caps.Metrics {
		if m.SemanticStatus == string(funnelregistry.Available) {
			avail[m.MetricKey] = true
			pendingOnly = false
		}
	}
	if len(caps.Metrics) == 0 {
		out.ReasonCode = "METRICS_UNAVAILABLE"
		out.Limitations = []string{"FUNNEL_CAPABILITY_ABSENT"}
		return out, nil
	}
	if pendingOnly && len(avail) == 0 {
		out.ReasonCode = "SUBSCRIPTION_REQUIRED"
		return out, nil
	}
	chain := pickFunnelChain(avail)
	if len(chain) == 0 {
		out.ReasonCode = "NO_COMPATIBLE_CHAIN"
		return out, nil
	}
	ready, zeros := 0, 0
	insufficientPrev := false
	for _, key := range chain {
		st := FunnelStage{Key: key, Status: "MISSING"}
		q["GetAccountFunnelPeriod"] += 2
		fp, ferr := in.API.GetAccountFunnelPeriod(ctx, in.Sealed, key, factualapi.PeriodCurrent7D, nil)
		if ferr == nil && fp.Metric.Value.State == factualapi.ValueKnown && fp.Metric.Value.Value != nil {
			s := fp.Metric.Value.Value.String()
			st.Status = "READY"
			st.Amount = &s
			ready++
			if fp.Metric.Value.Value.IsZero() {
				zeros++
			}
		} else if ferr == nil {
			st.ReasonCode = fp.Metric.Value.ReasonCode
		}
		pp, perr := in.API.GetAccountFunnelPeriod(ctx, in.Sealed, key, factualapi.PeriodPrevious7D, nil)
		if perr == nil && pp.Metric.Value.State == factualapi.ValueKnown && pp.Metric.Value.Value != nil {
			s := pp.Metric.Value.Value.String()
			st.PreviousAmount = &s
			if st.Amount != nil {
				if cur, err1 := factualapi.ParseDecimal(*st.Amount); err1 == nil {
					d := cur.SubString(*pp.Metric.Value.Value)
					st.Delta = &d
				}
			}
		} else if st.Status == "READY" {
			insufficientPrev = true
		}
		out.Stages = append(out.Stages, st)
	}
	if len(chain) == 2 {
		convKey := transitionKey(chain[0], chain[1])
		if convKey != "" && avail[convKey] {
			q["GetAccountFunnelPeriod"]++
			tr := FunnelTransition{FromKey: chain[0], ToKey: chain[1], Key: convKey, Status: "MISSING"}
			fp, ferr := in.API.GetAccountFunnelPeriod(ctx, in.Sealed, convKey, factualapi.PeriodCurrent7D, nil)
			if ferr == nil && fp.Metric.Value.State == factualapi.ValueKnown && fp.Metric.Value.Value != nil {
				s := fp.Metric.Value.Value.String()
				tr.Status = "READY"
				tr.Amount = &s
			}
			out.Transitions = append(out.Transitions, tr)
		}
	}
	switch {
	case ready == 0:
		out.State, out.Status, out.ReasonCode = "UNAVAILABLE", "UNAVAILABLE", "NO_COMPATIBLE_CHAIN"
	case ready == len(chain) && zeros == ready:
		out.State, out.Status, out.ReasonCode = "READY_EMPTY", "READY_EMPTY", "NO_ACTIVITY"
	case ready == len(chain):
		out.State, out.Status = "READY", "READY"
	default:
		out.State, out.Status = "PARTIAL", "PARTIAL"
		out.Limitations = append(out.Limitations, "PARTIAL_FUNNEL")
	}
	if insufficientPrev && (out.State == "READY" || out.State == "PARTIAL" || out.State == "READY_EMPTY") {
		out.Limitations = append(out.Limitations, "INSUFFICIENT_HISTORY")
	}
	out.Insight = funnelInsight(in.Validated)
	return out, nil
}

func pickFunnelChain(avail map[string]bool) []string {
	candidates := [][]string{
		{"PDP_IMPRESSIONS", "PDP_CART_ADDS"},
		{"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS"},
	}
	best := []string{}
	bestN := 0
	for _, c := range candidates {
		n := 0
		ok := false
		for _, k := range c {
			if avail[k] {
				n++
				ok = true
			}
		}
		if ok && n > bestN {
			best, bestN = c, n
		}
	}
	if bestN == 0 {
		return nil
	}
	var present []string
	for _, k := range best {
		if avail[k] {
			present = append(present, k)
		}
	}
	return present
}

func transitionKey(from, to string) string {
	switch {
	case from == "PDP_IMPRESSIONS" && to == "PDP_CART_ADDS":
		return "PDP_CART_CONVERSION"
	case from == "SEARCH_IMPRESSIONS" && to == "SEARCH_CART_ADDS":
		return "SEARCH_CART_CONVERSION"
	default:
		return ""
	}
}

func funnelInsight(v deep.ValidatedOutputV1) *FunnelInsight {
	for _, f := range v.Output.Findings {
		if f.PrimaryDomain == deep.DomainFunnel {
			return &FunnelInsight{Headline: f.Title, Statement: f.FindingStatement, PrimaryDomain: f.PrimaryDomain}
		}
	}
	return nil
}

func buildRows(ctx context.Context, in Input, q map[string]int) (stock, camp, focus []map[string]any, bands []CoverageBandCount, err error) {
	stock, camp, focus = []map[string]any{}, []map[string]any{}, []map[string]any{}
	bands = coverageBandCounts(nil)
	if in.API == nil {
		return
	}
	var skus []factualapi.SKUStock
	token := ""
	for {
		q["ListSKUStock"]++
		page, e := in.API.ListSKUStock(ctx, in.Sealed, factualapi.StockFilter{}, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if e != nil {
			return stock, camp, focus, bands, nil
		}
		skus = append(skus, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	ids := make([]int64, 0, len(skus))
	for _, it := range skus {
		ids = append(ids, it.CanonicalSKUID)
	}
	covBy := map[int64]*factualapi.StockCoverage{}
	demBy := map[int64]*factualapi.SKUDemand{}
	for i := 0; i < len(ids); i += 100 {
		end := i + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		q["BatchGetStockCoverage"]++
		if br, e := in.API.BatchGetStockCoverage(ctx, in.Sealed, chunk); e == nil {
			for _, item := range br.Items {
				if item.Coverage != nil {
					covBy[item.Coverage.CanonicalSKUID] = item.Coverage
				}
			}
		}
		q["BatchGetSKUDemand"]++
		if br, e := in.API.BatchGetSKUDemand(ctx, in.Sealed, chunk, factualapi.PeriodCurrent7D, nil); e == nil {
			for _, item := range br.Items {
				if item.Demand != nil {
					demBy[item.Demand.CanonicalSKUID] = item.Demand
				}
			}
		}
	}
	keySKUs := map[string]struct{}{}
	orderSKUs := map[string]struct{}{}
	for _, f := range in.Validated.Output.Findings {
		id := strings.TrimSpace(f.PrimaryEntity.ID)
		if id == "" {
			continue
		}
		if f.PrimaryDomain == deep.DomainSales {
			orderSKUs[id] = struct{}{}
		} else {
			keySKUs[id] = struct{}{}
		}
	}
	for i, it := range skus {
		srow := stockRowFromSKU(it, covBy[it.CanonicalSKUID], demBy[it.CanonicalSKUID], int64(i+1))
		stock = append(stock, srow)
		frow := map[string]any{}
		for k, v := range srow {
			frow[k] = v
		}
		skuKey := strconv.FormatInt(it.CanonicalSKUID, 10)
		var tabs []string
		if _, ok := orderSKUs[skuKey]; ok {
			tabs = append(tabs, "ORDERS")
		}
		if _, ok := keySKUs[skuKey]; ok {
			tabs = append(tabs, "KEY")
		}
		if len(tabs) > 0 {
			frow["focus_tabs"] = tabs
		}
		focus = append(focus, frow)
	}
	bands = coverageBandCounts(stock)
	token = ""
	var camps []factualapi.Campaign
	for {
		q["ListCampaigns"]++
		page, e := in.API.ListCampaigns(ctx, in.Sealed, factualapi.CampaignFilter{}, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if e != nil {
			return stock, camp, focus, bands, nil
		}
		camps = append(camps, page.Items...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	cids := make([]int64, 0, len(camps))
	for _, c := range camps {
		cids = append(cids, c.ID)
	}
	perfBy := map[int64]*factualapi.CampaignPeriodPerformance{}
	for i := 0; i < len(cids); i += 100 {
		end := i + 100
		if end > len(cids) {
			end = len(cids)
		}
		q["BatchGetCampaignPeriodPerformance"]++
		if br, e := in.API.BatchGetCampaignPeriodPerformance(ctx, in.Sealed, cids[i:end], factualapi.PeriodCurrent7D, nil); e == nil {
			for _, item := range br.Items {
				if item.Performance != nil {
					perfBy[item.Performance.CampaignID] = item.Performance
				}
			}
		}
	}
	for _, it := range camps {
		camp = append(camp, campaignRowFrom(it, perfBy[it.ID]))
	}
	return
}

func findingViews(v deep.ValidatedOutputV1) []FindingView {
	out := make([]FindingView, 0, len(v.Output.Findings))
	for i, f := range v.Output.Findings {
		ref := ""
		if i < len(v.FindingRefs) {
			ref = v.FindingRefs[i]
		}
		out = append(out, FindingView{
			Ref: ref, Title: f.Title, Statement: f.FindingStatement,
			PrimaryDomain: f.PrimaryDomain, OverviewBucket: f.OverviewBucket,
			EntityKind: string(f.PrimaryEntity.Kind), EntityID: f.PrimaryEntity.ID,
		})
	}
	return out
}

func filterFindings(all []FindingView, domain string) []FindingView {
	var out []FindingView
	for _, f := range all {
		if f.PrimaryDomain == domain {
			out = append(out, f)
		}
	}
	if out == nil {
		out = []FindingView{}
	}
	return out
}

func visibleRefs(p actionreconcile.PlanV1) []string {
	var out []string
	for _, n := range p.NewProposalIntents {
		if n.Visible {
			out = append(out, n.LocalRef)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func groupRefs(p actionreconcile.PlanV1) []string {
	out := make([]string, 0, len(p.Groups))
	for _, g := range p.Groups {
		out = append(out, g.LocalRef)
	}
	return out
}

func toGroupViews(p actionreconcile.PlanV1) []actionreconcileGroupView {
	out := make([]actionreconcileGroupView, 0, len(p.Groups))
	for _, g := range p.Groups {
		out = append(out, actionreconcileGroupView{
			LocalRef: g.LocalRef, MemberRefs: g.MemberLocalRefs, DisplayOrder: g.DisplayOrder,
			DisplayTitle: g.DisplayTitle, DisplaySummary: g.DisplaySummary,
			ActionType: g.ActionType, Family: g.Family, PresentationTemplateKey: g.PresentationKey,
		})
	}
	return out
}

func singles(p actionreconcile.PlanV1) []string {
	inGroup := map[string]bool{}
	for _, g := range p.Groups {
		for _, m := range g.MemberLocalRefs {
			inGroup[m] = true
		}
	}
	var out []string
	for _, n := range p.NewProposalIntents {
		if n.Visible && !inGroup[n.LocalRef] {
			out = append(out, n.LocalRef)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func firstHandles(v deep.ValidatedOutputV1) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range v.Output.ActionProposals {
		for _, h := range a.FactHandles {
			if !seen[h] {
				seen[h] = true
				out = append(out, h)
			}
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
