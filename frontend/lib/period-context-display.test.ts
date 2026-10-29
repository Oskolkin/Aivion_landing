import {
  formatPeriodContextDisplay,
  formatPeriodContextLines,
  formatRUDateRange,
  formatRUDateRangeWithYear,
} from "@/lib/period-context-display";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(formatRUDateRange("2026-07-02", "2026-07-08") === "2–8 июля", "same-month range");
  assert(
    formatRUDateRange("2026-06-25", "2026-07-01") === "25 июня–1 июля",
    "cross-month range",
  );
  assert(
    formatRUDateRangeWithYear("2026-07-02", "2026-07-08") === "2–8 июля 2026",
    "same-month range with year",
  );
  assert(
    formatRUDateRangeWithYear("2026-06-25", "2026-07-01") === "25 июня–1 июля 2026",
    "cross-month range with year",
  );

  const current = formatPeriodContextDisplay({
    value_period: "2026-07-08",
    comparison_period: "2026-07-07",
    trend_period: "7d",
    current_range: "2026-07-02/2026-07-08",
    previous_range: "2026-06-25/2026-07-01",
  });
  assert(current.groups.length === 3, "three semantic groups");
  assert(
    current.groups[0]?.label === "Период анализа" && current.groups[0]?.value === "2–8 июля 2026",
    "analysis period",
  );
  assert(
    current.groups[1]?.label === "Сравнение" && current.groups[1]?.value === "25 июня–1 июля 2026",
    "comparison period",
  );
  assert(
    current.groups[2]?.label === "Данные обновлены" && current.groups[2]?.value === "8 июля 2026",
    "data freshness date",
  );
  assert(
    !current.groups.some((item) => /Карточки|Графики|AI-анализ|Изменение/.test(item.label)),
    "no technical UI labels",
  );
  assert(
    !current.groups.some((item) => item.value.startsWith("к ")),
    "day-over-day delta not in header",
  );

  const fixed = formatPeriodContextDisplay({
    value_period: "2026-07-08",
    comparison_period: "2026-07-07",
    current_range: "2026-07-02/2026-07-08",
    previous_range: "2026-06-25/2026-07-01",
  });
  assert(fixed.groups.some((item) => item.value.includes("8 июля 2026")) === true, "uses metric date");
  assert(fixed.groups.some((item) => item.value.includes("2–8 июля 2026")) === true, "weekly range");

  const fromCaptions = formatPeriodContextDisplay({
    caption_primary: "Данные на 1 января · изменение к 31 декабря · графики за последние 7 дней",
    caption_secondary: "Недельный анализ: 26 декабря–1 января · сравнение: 19–25 декабря",
    value_period: "2026-07-08",
    comparison_period: "2026-07-07",
    current_range: "2026-07-02/2026-07-08",
    previous_range: "2026-06-25/2026-07-01",
  });
  assert(
    fromCaptions.groups.some((item) => item.label === "Данные обновлены" && item.value === "8 июля 2026"),
    "ISO dates preferred over caption_primary long strings",
  );
  assert(
    !fromCaptions.groups.some((item) => item.value.includes("1 января")),
    "caption_primary not used when ISO fields present",
  );

  const legacyLines = formatPeriodContextLines({
    value_period: "2026-07-08",
    comparison_period: "2026-07-07",
    trend_period: "7d",
    current_range: "2026-07-02/2026-07-08",
    previous_range: "2026-06-25/2026-07-01",
  });
  assert(Boolean(legacyLines.primary), "legacy lines still produced");
  assert(legacyLines.primary?.includes("Период анализа:") === true, "legacy primary uses new labels");
  assert(legacyLines.secondary === null, "legacy secondary unused");

  console.log("period-context-display.test.ts: ok");
}

run();
