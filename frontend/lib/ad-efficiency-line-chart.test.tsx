import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { AdEfficiencyLineChart } from "@/components/dashboard/ad-efficiency-line-chart";
import type { TrendPointDisplay } from "@/lib/ad-efficiency-display";

const points: TrendPointDisplay[] = [
  { date: "2026-05-08", label: "8 мая", drrValue: 28, drrLabel: "28%", deltaLabel: null, deltaTone: "neutral", isToday: false, isYesterday: false, hasDrr: true },
  { date: "2026-05-09", label: "9 мая", drrValue: 32, drrLabel: "32%", deltaLabel: "+4 п.п.", deltaTone: "warning", isToday: false, isYesterday: false, hasDrr: true },
  { date: "2026-05-10", label: "10 мая", drrValue: null, drrLabel: null, deltaLabel: null, deltaTone: "neutral", isToday: false, isYesterday: false, hasDrr: false },
  { date: "2026-05-11", label: "11 мая", drrValue: 29, drrLabel: "29%", deltaLabel: "-3 п.п.", deltaTone: "success", isToday: false, isYesterday: false, hasDrr: true },
  { date: "2026-05-12", label: "12 мая", drrValue: 31, drrLabel: "31%", deltaLabel: "+2 п.п.", deltaTone: "warning", isToday: false, isYesterday: false, hasDrr: true },
  { date: "2026-05-13", label: "13 мая", drrValue: 34, drrLabel: "34%", deltaLabel: "+3 п.п.", deltaTone: "warning", isToday: false, isYesterday: true, hasDrr: true },
  { date: "2026-05-14", label: "14 мая", drrValue: 38, drrLabel: "38%", deltaLabel: "+4 п.п.", deltaTone: "warning", isToday: true, isYesterday: false, hasDrr: true },
];

function run() {
  const html = renderToStaticMarkup(
    <AdEfficiencyLineChart points={points} hasTrendData averageDrrPct={30} sampleDays={7} />,
  );

  assert.ok(html.includes("Динамика ДРР в выручке с рекламных кампаний за 7 дней"));
  assert.ok(html.includes('data-testid="ad-efficiency-average-line"'));
  assert.ok(html.includes('stroke-dasharray="6 5"'));
  assert.ok(html.includes('data-testid="ad-efficiency-average-label"'));
  assert.ok(html.includes("За 7 дней: 30%"));
  assert.ok(!html.includes("Среднее:"));
  assert.ok(!html.includes("Динамика доли расходов"));
  assert.ok(html.includes('data-testid="ad-efficiency-today-marker"'));
  assert.ok(html.includes('data-testid="ad-efficiency-chart-summary"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-delta-label"'));
  assert.ok(!html.includes(">сегодня<"));
  assert.ok(!html.includes(">вчера<"));
  assert.ok(html.includes("14 мая"));
  assert.ok(html.includes('data-testid="ad-efficiency-area-segment"'));
  assert.ok(html.includes('stroke-linejoin="round"'));
  assert.ok(html.includes('paint-order:stroke'));
  assert.ok(html.includes('text-anchor="middle"'));

  const valueLabels = html.match(/data-testid="ad-efficiency-value-label"/g) ?? [];
  assert.equal(valueLabels.length, 6, "six value labels for available points");

  const emptyHtml = renderToStaticMarkup(<AdEfficiencyLineChart points={[]} hasTrendData={false} />);
  assert.ok(emptyHtml.includes('data-testid="ad-efficiency-line-chart-empty"'));
  assert.ok(emptyHtml.includes("ДРР в выручке с рекламных кампаний"));

  console.log("ad-efficiency-line-chart.test.tsx: ok");
}

run();
