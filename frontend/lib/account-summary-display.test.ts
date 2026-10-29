import {
  buildDashboardSummaryView,
  formatDateForDashboardSummary,
  sanitizeDashboardSummaryText,
  splitDashboardSummaryIntoParagraphs,
} from "@/lib/account-summary-display";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(formatDateForDashboardSummary("2026-05-21") === "21.05.2026", "ISO date formatted for UI");

  const sanitized = sanitizeDashboardSummaryText(
    "На срезе 2026-05-21 week-to-week выручка 101721.33 в dashboard. account_metrics_last_7d показывает top_skus, stock_available 4, status out_of_stock, cancellation_rate 13.64%, ROAS 0.12, DRR 257.9% и coverage 2.5.",
  );
  assert(sanitized.includes("21.05.2026"), "date inside summary formatted");
  assert(sanitized.includes("по сравнению с прошлой неделей"), "week-to-week localized naturally");
  assert(sanitized.includes("кабинет"), "dashboard localized");
  assert(sanitized.includes("данные за последние 7 дней"), "internal field localized");
  assert(sanitized.includes("ключевые SKU"), "top_skus localized");
  assert(sanitized.includes("остаток 4 шт."), "stock_available formatted with unit");
  assert(sanitized.includes("нет в наличии"), "out_of_stock localized");
  assert(sanitized.includes("доля отмен 13,64%"), "cancellation_rate localized");
  assert(sanitized.includes("окупаемость рекламы 0,12x"), "ROAS localized");
  assert(sanitized.includes("ДРР (доля рекламных расходов) 257,9%"), "DRR expanded");
  assert(/покрыти[ею] 2,5 дн\./.test(sanitized), "coverage formatted with unit");
  assert(sanitized.includes("выручка"), "money sentence preserved");
  assert(sanitized.includes("₽"), "money formatted with ruble sign");
  assert(!sanitized.includes("101721.33"), "raw money value is reformatted");
  assert(!sanitized.includes("week-to-week"), "raw week-to-week removed");
  assert(!sanitized.includes("dashboard"), "raw dashboard removed");
  assert(!sanitized.includes("account_metrics_last_7d"), "raw internal field removed");

  const paragraphs = splitDashboardSummaryIntoParagraphs(
    "Продажи снижаются. Выручка 101721.33. Реклама уходит в слабые кампании. Есть риск дефицита по SKU. Нужно проверить пополнение.",
  );
  assert(paragraphs.length >= 2, "long summary split into paragraphs");

  const naturalPhrases = sanitizeDashboardSummaryText(
    "В тексте в сравнении week-to-week продажи ниже. Заказы по кабинету снизились до 76 против 83 вчера. По дневному ряду account_metrics_last_7d видно ухудшение. Одновременно выросла отмена.",
    { ordersToday: 88, ordersYesterday: 87 },
  );
  assert(naturalPhrases.includes("по сравнению с прошлой неделей"), "awkward week comparison smoothed");
  assert(naturalPhrases.includes("Количество заказов выросло до 88 против 87 вчера"), "orders sentence aligned to seller pulse");
  assert(naturalPhrases.includes("По динамике за последние 7 дней видно"), "daily series phrasing improved");
  assert(naturalPhrases.includes("увеличилось количество отмен"), "cancellation phrase improved");
  assert(!naturalPhrases.includes("Заказы по кабинету"), "awkward orders phrase removed");

  const previousWeekPhrase = sanitizeDashboardSummaryText(
    "и на 12% ниже предыдущих 7 дней по сравнению с прошлой неделей",
  );
  assert(
    previousWeekPhrase === "и на 12% ниже уровня прошлой недели",
    "previous week phrase smoothed",
  );

  const idsAndMoney = sanitizeDashboardSummaryText(
    "Кампания 882000071563 потратила 29596 и принесла выручка 3472.61. Диапазон 146312.17–154915.60.",
  );
  assert(idsAndMoney.includes("882000071563"), "campaign id preserved");
  assert(idsAndMoney.includes("29 596 ₽") || idsAndMoney.includes("29 596 ₽"), "money rounded to rubles");
  assert(idsAndMoney.includes("3 473 ₽") || idsAndMoney.includes("3 473 ₽"), "decimal revenue rounded to rubles");
  assert(idsAndMoney.includes("146-155 тыс. ₽"), "large money range compacted");

  const longSummary = [
    "На срезе 2026-05-21 аккаунт показывает ухудшение по основным продажным метрикам.",
    "Выручка 101721.33, это на 17% ниже вчерашнего уровня 122566.36.",
    "Заказы по dashboard снизились до 76 против 83 вчера.",
    "В рекламе есть убыточные кампании с ROAS 0.12 и DRR 852.27%.",
    "По stock_available 4 и coverage 2.5 есть риск out_of_stock.",
  ].join(" ");

  const collapsedView = buildDashboardSummaryView(longSummary, false);
  assert(collapsedView.isExpandable, "long summary is collapsible");
  assert(collapsedView.toggleLabel === "Подробнее", "collapsed state shows expand button");
  assert(collapsedView.paragraphs.length <= 2, "collapsed state keeps compact preview");
  assert(!collapsedView.paragraphs.join(" ").includes("остаток 4 шт."), "collapsed preview hides tail details");

  const expandedView = buildDashboardSummaryView(longSummary, true);
  assert(expandedView.isExpandable, "expanded state remains expandable");
  assert(expandedView.toggleLabel === "Свернуть", "expanded state shows collapse button");
  assert(expandedView.paragraphs.join(" ").includes("852,27%"), "expanded state shows full text");
  assert(expandedView.paragraphs.length >= 2, "expanded state renders multiple paragraphs");

  const alignedView = buildDashboardSummaryView(
    "Заказы по dashboard снизились до 76 против 83 вчера. По дневному ряду account_metrics_last_7d видно ухудшение.",
    true,
    { ordersToday: 88, ordersYesterday: 87 },
  );
  assert(alignedView.paragraphs.join(" ").includes("Количество заказов выросло до 88 против 87 вчера"), "rendered summary uses canonical orders");
  assert(!alignedView.paragraphs.join(" ").includes("76"), "stale orders value removed from rendered summary");

  const shortView = buildDashboardSummaryView("Выручка выросла на 12%, а доля отмен снизилась до 3%.", false);
  assert(!shortView.isExpandable, "short summary does not need toggle");
  assert(shortView.toggleLabel === null, "short summary hides button");
  assert(shortView.paragraphs.length === 1, "short summary stays fully visible");

  console.log("account-summary-display.test.ts: ok");
}

run();
