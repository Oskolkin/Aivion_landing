import {
  comparisonBarHalfWidthPct,
  isComparisonConversionItem,
  partitionComparisonItems,
  scaleDivergingComparisonBars,
} from "@/lib/funnel-comparison-bars";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  const items = [
    { key: "impressions", label: "Показы", tone: "neutral" as const },
    { key: "views_to_cart", label: "Конверсия из просмотров в корзину", tone: "negative" as const },
    { key: "orders", label: "Заказанные единицы", tone: "neutral" as const },
  ];
  const { stages, conversions } = partitionComparisonItems(items);
  assert(stages.length === 2, "two stage rows");
  assert(conversions.length === 1, "one conversion row");
  assert(isComparisonConversionItem(items[1]), "views_to_cart is conversion");
  assert(!isComparisonConversionItem(items[0]), "impressions is stage");

  const scaled = scaleDivergingComparisonBars([
    { key: "a", label: "A", delta: "-12,2%", tone: "negative" },
    { key: "b", label: "B", delta: "+9,4%", tone: "positive" },
    { key: "c", label: "C", delta: "0%", tone: "neutral" },
  ]);
  assert(scaled[0].signedBarPct < 0, "negative goes left");
  assert(scaled[1].signedBarPct > 0, "positive goes right");
  assert(scaled[2].signedBarPct === 0, "neutral centered");
  assert(comparisonBarHalfWidthPct(-100) === 50, "max negative fills left half");
  assert(comparisonBarHalfWidthPct(40) === 20, "40% scaled maps to 20% half-width");
  assert(comparisonBarHalfWidthPct(0) === 0, "zero stays centered");
}

run();
