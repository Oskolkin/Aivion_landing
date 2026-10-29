import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildSalesFunnelDisplay,
  formatFunnelNumber,
  formatFunnelDeltaPct,
  formatFunnelDeltaText,
  formatFunnelPercent,
  getConversionFromPreviousCaption,
  getFixedFunnelStageWidth,
  getFunnelDeltaTone,
  getStageDisplayLabel,
  normalizeFunnelStageLabel,
  resolveConversionFromPreviousPct,
} from "@/lib/sales-funnel-display";
import type { FunnelStage } from "@/lib/dashboard-home-types";
import { testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function assertNoInvalidNumbers(display: ReturnType<typeof buildSalesFunnelDisplay>, context: string) {
  const blob = JSON.stringify(display);
  assert(!blob.includes("NaN"), `${context}: no NaN in output`);
  assert(!blob.includes("Infinity"), `${context}: no Infinity in output`);
}

const __dirname = dirname(fileURLToPath(import.meta.url));

function run() {
  assert(formatFunnelNumber(1234) === "1 234" || formatFunnelNumber(1234) === "1 234", "funnel number formatting");
  assert(formatFunnelPercent(13.24) === "13,2%", "funnel percent formatting");
  assert(formatFunnelDeltaPct(-7.7) === "−7,7%", "negative funnel delta formatting");
  assert(
    formatFunnelDeltaText(-7.7, { stageKey: "product_views", periodType: "current_7d" }) ===
      "Открытий карточек стало на 7,7% меньше, чем неделю назад",
    "count delta uses event-specific wording",
  );

  // —— Universal: arbitrary values (not demo 4025/533/…) ——
  const arbitrary: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 1000 }),
    testFunnelStage({ key: "product_views", value: 250 }),
    testFunnelStage({ key: "add_to_cart", value: 75 }),
    testFunnelStage({ key: "orders", value: 30 }),
  ];
  const arbitraryDisplay = buildSalesFunnelDisplay("available", arbitrary);
  assert(arbitraryDisplay.length === 4, "four arbitrary stages");
  assert(arbitraryDisplay[0].conversionLabel === "100,0%", "first stage always 100%");
  assert(arbitraryDisplay[1].conversionLabel === "25,0%", "250/1000");
  assert(arbitraryDisplay[2].conversionLabel === "30,0%", "75/250");
  assert(arbitraryDisplay[3].conversionLabel === "40,0%", "30/75");
  assert(arbitraryDisplay[3].conversionCaption === "от корзины", "orders caption by key");
  assertNoInvalidNumbers(arbitraryDisplay, "arbitrary values");

  // Fallback when conversion_from_previous omitted
  const fallbackOnly: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 800 }),
    testFunnelStage({ key: "product_views", value: 160 }),
    testFunnelStage({ key: "orders", value: 32 }),
  ];
  assert(resolveConversionFromPreviousPct(fallbackOnly[2], 2, fallbackOnly) === 20, "32/160 fallback");
  const fallbackDisplay = buildSalesFunnelDisplay("available", fallbackOnly);
  assert(fallbackDisplay[2].conversionLabel === "20,0%", "computed from adjacent values");
  assert(fallbackDisplay[2].conversionCaption === "от карточек", "caption from previous stage when cart omitted");

  // API field preferred when present
  const withApiField: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 100 }),
    testFunnelStage({ key: "product_views", value: 50, conversion_from_previous: 99.9 }),
  ];
  assert(resolveConversionFromPreviousPct(withApiField[1], 1, withApiField) === 99.9, "API conversion_from_previous wins");

  // Zero denominator
  const zeroDenom: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 0 }),
    testFunnelStage({ key: "product_views", value: 10 }),
  ];
  assert(resolveConversionFromPreviousPct(zeroDenom[1], 1, zeroDenom) === null, "zero previous value");
  const zeroDisplay = buildSalesFunnelDisplay("available", zeroDenom);
  assert(zeroDisplay[1].conversionLabel === "н/д", "zero denominator shows nd");
  assert(zeroDisplay[1].conversionCaption === null, "no caption when nd");

  // Non-finite API values fall back to neighbor calculation or nd
  const nonFiniteApi: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 200 }),
    testFunnelStage({ key: "product_views", value: 40, conversion_from_previous: Number.NaN }),
    testFunnelStage({ key: "add_to_cart", value: 10, conversion_from_previous: Number.POSITIVE_INFINITY }),
  ];
  assert(resolveConversionFromPreviousPct(nonFiniteApi[1], 1, nonFiniteApi) === 20, "NaN API falls back to 40/200");
  assert(resolveConversionFromPreviousPct(nonFiniteApi[2], 2, nonFiniteApi) === 25, "Infinity API falls back to 10/40");
  assertNoInvalidNumbers(buildSalesFunnelDisplay("available", nonFiniteApi), "non-finite API");

  // Without delivered_units (3 stages)
  const threeStages: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 500 }),
    testFunnelStage({ key: "product_views", value: 100 }),
    testFunnelStage({ key: "orders", value: 25 }),
  ];
  const threeDisplay = buildSalesFunnelDisplay("available", threeStages);
  assert(threeDisplay.length === 3, "works without delivered_units");
  assert(threeDisplay[2].conversionLabel === "25,0%", "orders from product_views");
  assertNoInvalidNumbers(threeDisplay, "three stages");

  // Unknown stage key: safe caption fallback
  const customStage: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 100 }),
    testFunnelStage({ key: "custom_checkout", value: 40 }),
  ];
  assert(getConversionFromPreviousCaption("custom_checkout") === "от «custom checkout»", "unknown key uses humanized caption");
  assert(normalizeFunnelStageLabel("hits_view") === "Увидели товар", "normalize legacy key");
  const customDisplay = buildSalesFunnelDisplay("available", customStage);
  assert(customDisplay[1].conversionCaption === "от показов", "unknown target stage uses previous stage caption");

  // Volume delta separate from transition conversion
  const volumeDeltaStages: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 1000, change_vs_previous_day_pct: -5 }),
    testFunnelStage({ key: "product_views", value: 200, conversion_from_previous: 20, change_vs_previous_day_pct: -50 }),
  ];
  const volumeDisplay = buildSalesFunnelDisplay("available", volumeDeltaStages);
  assert(volumeDisplay[1].conversionLabel === "20,0%", "transition percent unchanged");
  assert(volumeDisplay[1].previousDayChangePrimary === "−50%", "compact volume badge primary");
  assert(volumeDisplay[1].previousDayChangeSecondary?.includes("открытий") === true, "event noun in secondary");
  assert(volumeDisplay[1].previousDayChangeText.includes("Открытий карточек"), "full event sentence");
  assert(!volumeDisplay[1].previousDayChangeText.includes("объём"), "no abstract volume wording");
  assert(volumeDisplay[0].previousDayChangeSecondary?.includes("показов") === true, "impressions event noun");

  // Fixed widths scale with stage count, not demo values
  const widths3 = [0, 1, 2].map((i) => getFixedFunnelStageWidth(i, 3));
  assert(JSON.stringify(widths3) === JSON.stringify([100, 79, 58]), "widths for 3 stages");

  const widths6 = [0, 1, 2, 3, 4, 5].map((i) => getFixedFunnelStageWidth(i, 6));
  assert(widths6[0] === 100 && widths6[5] === 58, "six stage widths via formula");

  const sectionSrc = readFileSync(join(__dirname, "../components/dashboard/sales-funnel-module.tsx"), "utf8");
  assert(sectionSrc.includes("количество событий"), "footer mentions event counts and deltas");
  assert(!sectionSrc.includes("по объёму"), "module source has no по объёму");
  assert(!sectionSrc.includes("объём этапа"), "module source has no объём этапа");

  assert(getFunnelDeltaTone(null) === "unavailable", "unavailable delta tone");
  assert(getStageDisplayLabel("unknown_key", "Fallback") === "Fallback", "unknown display label uses API label");

  console.log("sales-funnel-display.test.ts: ok");
}

run();
