import assert from "node:assert/strict";
import {
  buildAdEfficiencyDisplay,
  buildAdEfficiencyTodayTooltip,
  buildAdEfficiencyPeriodTooltip,
  formatAverageChartLabel,
  formatAverageDeltaCaption,
  formatAverageLineLabel,
  formatDrrPercent,
  formatFooterNote,
  formatMoneyRub,
  formatPeriodLabel,
  formatPpDelta,
  getDrrDeltaTone,
} from "@/lib/ad-efficiency-display";
import type { AdEfficiencyWidget } from "@/lib/ad-efficiency-display";

function run() {
  assert.equal(formatDrrPercent(38), "38%");
  assert.equal(formatDrrPercent(31.6), "31,6%");
  assert.equal(formatDrrPercent(null), "—");

  assert.equal(formatMoneyRub(26417), "26 417 ₽");
  assert.equal(formatMoneyRub(undefined), "—");

  assert.equal(formatPpDelta(6.4), "+6,4 п.п.");
  assert.equal(formatPpDelta(-2.4), "−2,4 п.п.");
  assert.equal(formatPpDelta(0), "0 п.п.");

  assert.equal(getDrrDeltaTone(2), "warning");
  assert.equal(getDrrDeltaTone(-2), "success");
  assert.equal(getDrrDeltaTone(0), "neutral");

  assert.equal(formatPeriodLabel(7), "за последние 7 дней");
  assert.equal(formatPeriodLabel(5), "за доступные 5 дней");
  assert.equal(formatPeriodLabel(null), null);

  assert.equal(formatAverageDeltaCaption(7, 30), "к среднему за 7 дней");
  assert.equal(formatAverageDeltaCaption(5, 28), "к среднему за доступные 5 дней");
  assert.equal(formatAverageDeltaCaption(1, 30), null);

  assert.equal(formatAverageLineLabel(7), "За 7 дней");
  assert.equal(formatAverageLineLabel(5), "За доступные 5 дней");
  assert.equal(formatAverageChartLabel(7, 30), "За 7 дней: 30%");
  assert.equal(formatAverageChartLabel(5, 28), "За доступные 5 дней: 28%");
  assert.equal(formatAverageChartLabel(7, null), null);

  assert.match(formatFooterNote(7), /последние 7 дней/);
  assert.match(formatFooterNote(4), /доступные 4 дня/);

  const widget: AdEfficiencyWidget = {
    status: "available",
    summary: {
      current_drr_pct: null,
      delta_vs_previous_pp: 6.4,
      delta_vs_average_7d_pp: -2,
      average_drr_7d_pct: 30,
      total_spend_today: 1000,
      total_revenue_today: 2000,
      total_store_revenue_today: 10000,
      total_orders_today: 5,
      total_spend_7d: 7000,
      total_attributed_revenue_7d: 14000,
      total_store_revenue_7d: 70000,
      campaigns_high_drr_count: 1,
      campaigns_no_orders_count: 2,
      high_drr_spend: 500,
      spend_without_orders: 26417,
      high_drr_threshold_pct: 100,
    },
    period: { metric_date: "2026-05-14", date_from: "2026-05-08", date_to: "2026-05-14", sample_days: 7 },
    trend_7d: [],
    ad_efficiency_narrative: { status: "pending" },
    cta: { label: "Открыть рекламные кампании", href: "/app/advertising" },
  };

  const display = buildAdEfficiencyDisplay(widget);
  assert.equal(display.currentDrr, "—");
  assert.equal(display.deltaVsPrevious, "+6,4 п.п.");
  assert.equal(display.aiConclusion.kind, "pending");
  assert.equal(display.averageLineLabel, "За 7 дней");
  assert.equal(display.averageChartLabel, "За 7 дней: 30%");
  assert.match(display.todayTooltip, /атрибутированная рекламным кампаниям/);
  assert.match(display.todayTooltip, /1 000 ₽ ÷ 2 000 ₽ = 50%/);
  assert.match(display.periodTooltip, /7 000 ₽ ÷ 14 000 ₽ = 50%/);
  assert.ok(!("recommendation" in display.aiConclusion));
  assert.ok(!JSON.stringify(display).includes("current_drr_pct"));
  assert.ok(!JSON.stringify(display).includes("not_enough_data"));

  const todayTooltip = buildAdEfficiencyTodayTooltip(widget.summary);
  assert.match(todayTooltip, /Формула:/);
  assert.match(todayTooltip, /Расчёт:/);

  const periodTooltip = buildAdEfficiencyPeriodTooltip(widget.summary);
  assert.match(periodTooltip, /сумма расходов/);
  assert.match(periodTooltip, /Расчёт:/);

  const conflictingSummaryDisplay = buildAdEfficiencyDisplay({
    ...widget,
    summary: {
      ...widget.summary!,
      delta_vs_previous_pp: -0.3,
    },
    ad_efficiency_narrative: {
      status: "ready",
      summary: "Рост ДРР связан с проблемными кампаниями.",
      checklist: [],
    },
  });
  assert.equal(conflictingSummaryDisplay.aiConclusion.kind, "unavailable");

  console.log("ad-efficiency-display.test.ts: ok");
}

run();
