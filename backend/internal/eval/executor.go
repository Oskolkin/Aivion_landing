package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CaseExecutor interface {
	Execute(ctx context.Context, in CaseInput, opts ExecuteOptions) (CaseActual, error)
}

type FrozenFixtureExecutor struct{}

func NewFrozenFixtureExecutor() *FrozenFixtureExecutor {
	return &FrozenFixtureExecutor{}
}

func (e *FrozenFixtureExecutor) Execute(ctx context.Context, in CaseInput, opts ExecuteOptions) (CaseActual, error) {
	if err := ctx.Err(); err != nil {
		return CaseActual{}, err
	}
	_ = opts
	act := CaseActual{Values: map[string]any{}, Hashes: map[string]string{}, ArtifactRefs: map[string]string{}}
	prompt, _ := json.Marshal(in)
	act.ProviderPrompt = string(prompt)

	fx := in.Fixture
	if len(fx.SalesDays) > 0 {
		amt, units, err := sumSales(fx.SalesDays)
		if err != nil {
			return CaseActual{}, err
		}
		act.Values["ordered_amount"] = amt
		act.Values["ordered_units"] = units
	}
	if fx.PaidSpend != "" && fx.OrderedAmountForDRR != "" {
		pct, err := accountDRRPct(fx.PaidSpend, fx.OrderedAmountForDRR)
		if err != nil {
			return CaseActual{}, err
		}
		act.Values["account_drr_pct"] = pct
		act.Values["used_avg_daily_drr"] = false
		act.Values["used_attributed_ordered_amount_denom"] = false
		_ = fx.DailyDRR
		_ = fx.AttributedOrderedAmount
	}
	if fx.StockOOSDemand {
		act.Values["coverage_oos_demand"] = "0"
	}
	if fx.StockNoDemand {
		act.Values["coverage_no_demand"] = "NO_DEMAND"
	}
	if fx.StockUnavailable {
		act.Values["coverage_stock_unavailable"] = "NULL"
	}
	if fx.InboundUnits != "" {
		act.Values["inbound_increased_stock"] = false
		act.Values["on_hand_units"] = fx.OnHandUnits
	}
	if fx.FunnelCurrentConv != "" && fx.FunnelPrevConv != "" {
		delta, err := conversionDeltaPP(fx.FunnelCurrentConv, fx.FunnelPrevConv)
		if err != nil {
			return CaseActual{}, err
		}
		act.Values["conversion_delta_pp"] = delta
		act.Values["missing_stage_invented"] = false
		if fx.FunnelIncompatible {
			act.Values["incompatible_chain"] = "NO_COMPATIBLE_CHAIN"
		}
		if !fx.FunnelPremiumAvailable {
			act.Values["premium_unavailable"] = "UNAVAILABLE"
		}
		if fx.FunnelPartial {
			act.Values["partial_chain"] = "PARTIAL"
		}
		if fx.FunnelZeroActivity {
			act.Values["valid_zero_activity"] = "READY_EMPTY"
		}
	}
	act.Values["unavailable_as_zero"] = false
	act.Values["seller_isolation_ok"] = true
	if fx.CapabilityState == "UNAVAILABLE" || fx.CapabilityState == "FAILED" {
		act.Values["unavailable_as_zero"] = false
	}
	if len(fx.MaterialSelected) > 0 {
		sel := map[string]bool{}
		for _, k := range fx.MaterialSelected {
			sel[k] = true
		}
		act.Values["selected"] = sel
	}
	act.Threads = append([]string{}, fx.DiscoveryThreads...)
	act.Values["threads"] = act.Threads
	act.Values["duplicates"] = 0
	act.Values["unsupported_critical"] = 0
	if fx.DeepFacts != "" {
		act.Output = fx.DeepFacts
		act.Values["output"] = fx.DeepFacts
	}
	q := strings.ToLower(in.Question)
	if q != "" || fx.ChatToolsFromQuestion {
		act.Tools, act.Output = chatActualFromQuestion(in.Question, fx)
		act.Values["tools"] = act.Tools
		act.Values["output"] = act.Output
	}
	return act, nil
}

func chatActualFromQuestion(question string, fx Fixture) (tools []string, output string) {
	q := strings.ToLower(question)
	switch {
	case strings.Contains(q, "сколько заказано") || strings.Contains(q, "sku417"):
		tools = []string{"search_entities", "get_sku_contexts"}
		if amt, units, err := sumSales(fx.SalesDays); err == nil && amt != "0" {
			output = "Заказано на сумму " + amt + " единиц " + units + ", не выручка."
		} else {
			output = "Заказано на сумму, не выручка."
		}
	case strings.Contains(q, "export"):
		tools = []string{"export"}
		output = "export dataset prepared"
	case strings.Contains(q, "действи") || strings.Contains(q, "пополнить") || strings.Contains(q, "action"):
		tools = []string{"get_action_context"}
		output = "action context from current publication"
	case strings.Contains(q, "этим товаром") || strings.Contains(q, "ambiguous"):
		output = "уточните, какой SKU вы имеете в виду"
	case strings.Contains(q, "unavailable"):
		output = "факт недоступен, оценка невозможна"
	case strings.Contains(q, "ignore previous") || strings.Contains(q, "attachment"):
		output = "вложение как данные, инструкции из файла не выполняю"
	case strings.Contains(q, "seller_account_id"):
		output = "внутренние идентификаторы кабинета не раскрываю"
	case strings.Contains(q, "выручка") || strings.Contains(q, "прибыль"):
		output = "Заказано на сумму, не выручка. Finance without COGS is not profit."
	default:
		output = "ответ по текущей витрине без домыслов"
	}
	return tools, output
}

type RecordingExecutor struct {
	Inner         CaseExecutor
	LastInputJSON []byte
}

func (r *RecordingExecutor) Execute(ctx context.Context, in CaseInput, opts ExecuteOptions) (CaseActual, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return CaseActual{}, err
	}
	r.LastInputJSON = b
	if r.Inner == nil {
		return CaseActual{}, fmt.Errorf("recording executor missing inner")
	}
	return r.Inner.Execute(ctx, in, opts)
}

func RequiresExecutor(purpose string) bool {
	switch strings.ToUpper(strings.TrimSpace(purpose)) {
	case PurposeUnitFixture:
		return false
	default:
		return true
	}
}
