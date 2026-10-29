import {
  formatCompactStageCountDelta,
  formatConversionCurrentCaption,
  formatConversionDeltaLabels,
  formatStageCountDelta,
  getFunnelStageCopy,
  resolveFunnelComparisonPeriod,
  resolveFunnelStageCanonicalId,
} from "@/lib/funnel-stage-copy";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(resolveFunnelStageCanonicalId("hits_view_pdp") === "product_views", "legacy alias");
  assert(getFunnelStageCopy("product_views").changeSubject === "Открытий карточек", "product_views subject");

  const week = resolveFunnelComparisonPeriod({ periodType: "current_7d" });
  assert(week.compactSuffix === "к прошлой неделе", "weekly compact");
  assert(week.fullClause === "чем неделю назад", "weekly full");

  const custom = resolveFunnelComparisonPeriod({ comparisonLabel: "к предыдущей сводке" });
  assert(custom.compactSuffix === "к предыдущей сводке", "explicit comparison label");

  const generic = resolveFunnelComparisonPeriod({ periodType: "custom_range" });
  assert(generic.compactSuffix === "к предыдущему периоду", "non-weekly fallback");

  const impressionsNeg = formatStageCountDelta({ stageKey: "impressions", deltaPercent: -51, periodType: "current_7d" });
  assert(impressionsNeg.full === "Показов стало на 51% меньше, чем неделю назад", "impressions negative full");
  assert(impressionsNeg.primary === "−51%", "impressions primary");
  assert(impressionsNeg.secondary === "показов к прошлой неделе", "impressions secondary");

  const impressionsPos = formatStageCountDelta({ stageKey: "impressions", deltaPercent: 12.5, periodType: "current_7d" });
  assert(impressionsPos.full.includes("больше"), "impressions positive");
  assert(impressionsPos.primary === "+12,5%", "impressions positive primary");

  const viewsNeg = formatStageCountDelta({ stageKey: "product_views", deltaPercent: -55.2, periodType: "current_7d" });
  assert(viewsNeg.full === "Открытий карточек стало на 55,2% меньше, чем неделю назад", "views negative full");
  assert(!viewsNeg.full.toLowerCase().includes("объём"), "no volume word in full");
  assert(!viewsNeg.secondary?.includes("объём"), "no volume word in secondary");

  const cartPos = formatStageCountDelta({ stageKey: "add_to_cart", deltaPercent: 18.8, periodType: "current_7d" });
  assert(cartPos.full === "Добавлений в корзину стало на 18,8% больше, чем неделю назад", "cart positive");

  const ordersNeg = formatStageCountDelta({ stageKey: "orders", deltaPercent: -6.9, periodType: "current_7d" });
  assert(ordersNeg.full === "Заказанных единиц стало на 6,9% меньше, чем неделю назад", "orders negative");

  const deliveredNeg = formatStageCountDelta({ stageKey: "delivered_units", deltaPercent: -7.9, periodType: "current_7d" });
  assert(deliveredNeg.full === "Доставленных единиц стало на 7,9% меньше, чем неделю назад", "delivered negative");

  const zero = formatStageCountDelta({ stageKey: "product_views", deltaPercent: 0, periodType: "current_7d" });
  assert(zero.primary === "Без изменений", "zero primary");
  assert(zero.full.includes("не изменилось"), "zero full");
  assert(!zero.full.includes("0,0%"), "zero not shown as 0%");

  const missing = formatStageCountDelta({ stageKey: "product_views", deltaPercent: null });
  assert(missing.primary === "Нет данных для сравнения", "missing primary");
  assert(missing.secondary === null, "missing secondary hidden");

  const unknown = formatStageCountDelta({ stageKey: "custom_checkout", deltaPercent: -10, periodType: "current_7d" });
  assert(unknown.full.includes("Custom checkout") || unknown.full.includes("custom checkout") || unknown.full.toLowerCase().includes("custom"), "unknown stage uses key");
  assert(!unknown.full.includes("по объёму"), "unknown stage has no volume phrasing");

  const compact = formatCompactStageCountDelta({ stageKey: "product_views", deltaPercent: -55.2, periodType: "current_7d" });
  assert(compact.primary === "−55,2%", "compact primary");
  assert(compact.secondary === "открытий к прошлой неделе", "compact secondary short noun");

  const arbitraryPeriod = formatStageCountDelta({
    stageKey: "product_views",
    deltaPercent: -10,
    comparisonLabel: "к предыдущему периоду",
  });
  assert(
    Boolean(
      arbitraryPeriod.full.includes("к предыдущему периоду") ||
        arbitraryPeriod.full.includes("чем к предыдущему периоду") ||
        arbitraryPeriod.secondary?.includes("к предыдущему периоду"),
    ),
    "arbitrary period",
  );

  const conv = formatConversionDeltaLabels({ conversionDeltaPp: -3.2, periodType: "current_7d" });
  assert(conv.primary === "−3,2 п.п.", "pp primary uses typographic minus and п.п.");
  assert(conv.secondary === "Конверсия к прошлой неделе", "pp secondary");
  assert(formatConversionDeltaLabels({ conversionDeltaPp: null }).primary === null, "pp hidden when missing");
  assert(formatConversionCurrentCaption("product_views") === "Конверсия в карточку", "conversion caption");

  const blob = JSON.stringify({
    impressionsNeg,
    viewsNeg,
    cartPos,
    ordersNeg,
    deliveredNeg,
    zero,
    missing,
    compact,
  });
  assert(!blob.includes("NaN"), "no NaN");
  assert(!/\bпо объёму\b/.test(blob), "no по объёму");
  assert(!/объём этапа/.test(blob), "no объём этапа");

  console.log("funnel-stage-copy.test.ts: ok");
}

run();
