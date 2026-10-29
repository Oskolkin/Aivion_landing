import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { AdvertisingMetricCard } from "@/components/dashboard/ad-efficiency-metric-card";
import { Activity, CalendarDays } from "lucide-react";
import { AD_EFFICIENCY_METRIC_LABEL } from "@/lib/ad-efficiency-display";

function run() {
  const currentHtml = renderToStaticMarkup(
    <AdvertisingMetricCard
      testId="ad-efficiency-drr-card"
      label={AD_EFFICIENCY_METRIC_LABEL}
      value="38%"
      valueTone="primary"
      infoTooltip="Формула: расходы ÷ выручка × 100%."
      supportingText={
        <span className="text-emerald-700">−2,1 п.п. к вчера</span>
      }
      icon={Activity}
    />,
  );

  const averageHtml = renderToStaticMarkup(
    <AdvertisingMetricCard
      testId="ad-efficiency-average-drr-card"
      label="За 7 дней"
      value="30%"
      supportingText="за последние 7 дней"
      icon={CalendarDays}
    />,
  );

  assert.ok(currentHtml.includes('data-component="advertising-metric-card"'));
  assert.ok(averageHtml.includes('data-component="advertising-metric-card"'));
  assert.ok(currentHtml.includes(AD_EFFICIENCY_METRIC_LABEL));
  assert.ok(currentHtml.includes("38%"));
  assert.ok(currentHtml.includes("text-emerald-700"));
  assert.ok(currentHtml.includes("aria-label=\"Формула: расходы ÷ выручка × 100%.\""));
  assert.ok(averageHtml.includes("За 7 дней"));
  assert.ok(averageHtml.includes("за последние 7 дней"));
  assert.ok(currentHtml.includes("mt-auto"));
  assert.ok(averageHtml.includes("mt-auto"));
  assert.ok(currentHtml.includes("min-h-[184px]"));
  assert.ok(!currentHtml.includes("ДРР сегодня"));
  assert.ok(!averageHtml.includes("Средний ДРР"));

  const unknownHtml = renderToStaticMarkup(
    <AdvertisingMetricCard
      testId="ad-efficiency-drr-card"
      label={AD_EFFICIENCY_METRIC_LABEL}
      value="—"
      icon={Activity}
    />,
  );
  assert.ok(unknownHtml.includes("Нет данных"));
  assert.ok(!unknownHtml.includes(">0%<"));

  console.log("ad-efficiency-metric-card.test.tsx: ok");
}

run();
