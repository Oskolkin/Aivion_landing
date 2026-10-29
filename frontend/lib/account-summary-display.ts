const COLLAPSED_PARAGRAPH_LIMIT = 2;
const COLLAPSED_CHARACTER_LIMIT = 320;

const isoDatePattern = /\b(\d{4})-(\d{2})-(\d{2})\b/g;

type SummaryView = {
  paragraphs: string[];
  isExpandable: boolean;
  toggleLabel: "Подробнее" | "Свернуть" | null;
};

type SummaryDisplayOptions = {
  ordersToday?: number;
  ordersYesterday?: number;
};

const integerFormatter = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
});

function formatNumber(value: string, maximumFractionDigits = 2): string {
  const normalized = value.replace(/\s+/g, "").replace(",", ".");
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed)) {
    return value;
  }

  const hasFraction = normalized.includes(".");
  return new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 0,
    maximumFractionDigits: hasFraction ? maximumFractionDigits : 0,
  }).format(parsed);
}

function formatMoney(value: string): string {
  const normalized = value.replace(/\s+/g, "").replace(",", ".");
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed)) {
    return `${value} ₽`;
  }
  return `${integerFormatter.format(Math.round(parsed))} ₽`;
}

function formatMoneyRange(start: string, end: string): string {
  const parsedStart = Number(start.replace(/\s+/g, "").replace(",", "."));
  const parsedEnd = Number(end.replace(/\s+/g, "").replace(",", "."));
  if (!Number.isFinite(parsedStart) || !Number.isFinite(parsedEnd)) {
    return `${start}-${end}`;
  }

  if (Math.abs(parsedStart) >= 100_000 && Math.abs(parsedEnd) >= 100_000) {
    return `${integerFormatter.format(Math.round(parsedStart / 1000))}-${integerFormatter.format(Math.round(parsedEnd / 1000))} тыс. ₽`;
  }

  return `${formatMoney(start)}-${formatMoney(end)}`;
}

function formatCount(value: string, unit: string): string {
  return `${integerFormatter.format(Number(value))} ${unit}`;
}

function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text;
  }

  const sliced = text.slice(0, Math.max(0, maxLength - 1));
  const boundary = sliced.lastIndexOf(" ");
  const safeSlice = boundary >= Math.floor(maxLength * 0.6) ? sliced.slice(0, boundary) : sliced;
  return `${safeSlice.trimEnd()}...`;
}

function truncateParagraphs(paragraphs: string[], maxLength: number): string[] {
  const result: string[] = [];
  let remaining = maxLength;

  for (const paragraph of paragraphs) {
    if (remaining <= 0) {
      break;
    }
    if (paragraph.length <= remaining) {
      result.push(paragraph);
      remaining -= paragraph.length + 1;
      continue;
    }
    result.push(truncateText(paragraph, remaining));
    break;
  }

  return result;
}

function normalizeWhitespace(text: string): string {
  return text.replace(/\r\n/g, "\n").replace(/\s*\n+\s*/g, " ").replace(/\s{2,}/g, " ").trim();
}

function replaceMoneyKeywords(text: string): string {
  const patterns: Array<[RegExp, (keyword: string, value: string) => string]> = [
    [/(revenue|выручка)\s+(-?\d+(?:[.,]\d+)?)/giu, (keyword, value) => `${keyword === "revenue" ? "выручка" : keyword} ${formatMoney(value)}`],
    [/(spend|расход|расходы|бюджет|потратила|потратил|потратили|потрачено|уровня)\s+(-?\d+(?:[.,]\d+)?)/giu, (keyword, value) => `${keyword === "spend" ? "расход" : keyword} ${formatMoney(value)}`],
  ];

  let output = text;
  for (const [pattern, formatter] of patterns) {
    output = output.replace(pattern, (_, keyword: string, value: string) => formatter(keyword, value));
  }
  return output;
}

function replaceMoneyLikeDecimalsInContext(text: string): string {
  return text.replace(/\b\d{4,}(?:[.,]\d+)\b/gu, (value) => formatMoney(value));
}

function replaceMoneyRanges(text: string): string {
  return text.replace(/\b(\d{5,}(?:[.,]\d+)?)\s*[–-]\s*(\d{5,}(?:[.,]\d+)?)\b/gu, (_, start: string, end: string) => {
    return formatMoneyRange(start, end);
  });
}

function replaceCountKeywords(text: string): string {
  const patterns: Array<[RegExp, string]> = [
    [/\borders\s+(\d+)\b/gi, "заказов"],
    [/\bimpressions\s+(\d+)\b/gi, "показов"],
    [/\bclicks\s+(\d+)\b/gi, "кликов"],
    [/\bviews\s+(\d+)\b/gi, "просмотров"],
    [/\bstock_available\s+(\d+)\b/gi, "шт."],
    [/\bstock\s+(\d+)\b/gi, "шт."],
  ];

  let output = text;
  for (const [pattern, unit] of patterns) {
    output = output.replace(pattern, (_, value: string) => {
      const prefix = unit === "шт." ? "остаток" : "";
      const formatted = formatCount(value, unit);
      return prefix ? `${prefix} ${formatted}` : formatted;
    });
  }
  return output;
}

function replaceMetricKeywords(text: string): string {
  let output = text;
  let didExpandDRR = false;

  output = output.replace(/По dashboard/g, "По кабинету");
  output = output.replace(/по dashboard/gi, "по кабинету");
  output = output.replace(/status\s+out_of_stock/gi, "статус: нет в наличии");
  output = output.replace(/Есть риск out_of_stock/g, "Есть риск дефицита товара");
  output = output.replace(/есть риск out_of_stock/gi, "есть риск дефицита товара");
  output = output.replace(/По stock_available\s+(\d+)/g, (_, value: string) => {
    return `По остатку ${formatCount(value, "шт.")}`;
  });
  output = output.replace(/по stock_available\s+(\d+)/gi, (_, value: string) => {
    return `по остатку ${formatCount(value, "шт.")}`;
  });
  output = output.replace(/По stock\s+(\d+)/g, (_, value: string) => {
    return `По остатку ${formatCount(value, "шт.")}`;
  });
  output = output.replace(/по stock\s+(\d+)/gi, (_, value: string) => {
    return `по остатку ${formatCount(value, "шт.")}`;
  });
  output = output.replace(/И coverage\s+(-?\d+(?:[.,]\d+)?)/g, (_, value: string) => {
    return `И покрытию ${formatNumber(value, 1)} дн.`;
  });
  output = output.replace(/и coverage\s+(-?\d+(?:[.,]\d+)?)/gi, (_, value: string) => {
    return `и покрытию ${formatNumber(value, 1)} дн.`;
  });

  output = output.replace(/\bcancellation_rate\s+(-?\d+(?:[.,]\d+)?)%?/gi, (_, value: string) => {
    return `доля отмен ${formatNumber(value)}%`;
  });

  output = output.replace(/\b(?:days_of_cover|coverage)\s+(-?\d+(?:[.,]\d+)?)\b/gi, (_, value: string) => {
    return `покрытие ${formatNumber(value, 1)} дн.`;
  });

  output = output.replace(/С ROAS\s+(-?\d+(?:[.,]\d+)?)/g, (_, value: string) => {
    return `С окупаемостью рекламы ${formatNumber(value)}x`;
  });

  output = output.replace(/с ROAS\s+(-?\d+(?:[.,]\d+)?)/gi, (_, value: string) => {
    return `с окупаемостью рекламы ${formatNumber(value)}x`;
  });

  output = output.replace(/\bROAS\s+(-?\d+(?:[.,]\d+)?)\b/gi, (_, value: string) => {
    return `окупаемость рекламы ${formatNumber(value)}x`;
  });

  output = output.replace(/\b(?:DRR|ДРР)\s+(-?\d+(?:[.,]\d+)?)%/gi, (_, value: string) => {
    const prefix = didExpandDRR ? "ДРР" : "ДРР (доля рекламных расходов)";
    didExpandDRR = true;
    return `${prefix} ${formatNumber(value)}%`;
  });

  return output;
}

function replacePlainPhrases(text: string): string {
  const replacements: Array<[RegExp, string]> = [
    [/\bweek-to-week\b/gi, "по сравнению с прошлой неделей"],
    [/\bdashboard\b/gi, "кабинет"],
    [/\baccount_metrics_last_7d\b/gi, "данные за последние 7 дней"],
    [/\btop_skus\b/gi, "ключевые SKU"],
    [/\bout_of_stock\b/gi, "нет в наличии"],
    [/\bstatus\b/gi, "статус"],
    [/\bsource_status\b/gi, "статус источника"],
    [/с окупаемость рекламы/gi, "с окупаемостью рекламы"],
    [/есть риск нет в наличии/gi, "есть риск дефицита товара"],
  ];

  let output = text;
  for (const [pattern, replacement] of replacements) {
    output = output.replace(pattern, replacement);
  }
  return output;
}

function buildOrdersSentence(ordersToday: number, ordersYesterday: number): string {
  const formattedToday = integerFormatter.format(ordersToday);
  const formattedYesterday = integerFormatter.format(ordersYesterday);
  if (ordersToday > ordersYesterday) {
    return `Количество заказов выросло до ${formattedToday} против ${formattedYesterday} вчера`;
  }
  if (ordersToday < ordersYesterday) {
    return `Количество заказов снизилось до ${formattedToday} против ${formattedYesterday} вчера`;
  }
  return `Количество заказов осталось на уровне ${formattedToday} по сравнению со вчерашним днём`;
}

function replaceAwkwardPhrases(text: string, options: SummaryDisplayOptions): string {
  let output = text;

  if (typeof options.ordersToday === "number" && typeof options.ordersYesterday === "number") {
    output = output.replace(
      /(?:Заказы по кабинету|Количество заказов)\s+(?:снизились|снизилось|выросли|выросло|увеличились|увеличилось|сократились|сократилось)\s+до\s+\d+\s+против\s+\d+\s+вчера/giu,
      buildOrdersSentence(options.ordersToday, options.ordersYesterday),
    );
  }

  const replacements: Array<[RegExp, string]> = [
    [/в сравнении по сравнению с прошлой неделей/giu, "по сравнению с прошлой неделей"],
    [/в сравнении неделя к неделе/giu, "по сравнению с прошлой неделей"],
    [/предыдущих 7 дней по сравнению с прошлой неделей/giu, "уровня прошлой недели"],
    [/предыдущие 7 дней по сравнению с прошлой неделей/giu, "уровень прошлой недели"],
    [/Заказы по кабинету/gu, "Количество заказов"],
    [/заказы по кабинету/gu, "Количество заказов"],
    [/Количество заказов снизились/gu, "Количество заказов снизилось"],
    [/Количество заказов выросли/gu, "Количество заказов выросло"],
    [/Количество заказов увеличились/gu, "Количество заказов увеличилось"],
    [/Количество заказов сократились/gu, "Количество заказов сократилось"],
    [/По дневному ряду данные за последние 7 дней видно/gu, "По динамике за последние 7 дней видно"],
    [/по дневному ряду данные за последние 7 дней видно/gu, "по динамике за последние 7 дней видно"],
    [/По дневному ряду данные за последние 7 дней/gu, "По динамике за последние 7 дней"],
    [/по дневному ряду данные за последние 7 дней/gu, "по динамике за последние 7 дней"],
    [/выросла отмена/giu, "увеличилось количество отмен"],
  ];

  for (const [pattern, replacement] of replacements) {
    output = output.replace(pattern, replacement);
  }

  return output;
}

function cleanupText(text: string): string {
  return text
    .replace(/\s+([,.;:%)])/g, "$1")
    .replace(/\(\s+/g, "(")
    .replace(/\s+\)/g, ")")
    .replace(/дн\.\./g, "дн.")
    .replace(/шт\.\./g, "шт.")
    .replace(/\s{2,}/g, " ")
    .trim();
}

function sentenceSplit(text: string): string[] {
  return text
    .split(/(?<=[.!?])\s+(?=[A-ZА-ЯЁ0-9])/u)
    .map((sentence) => sentence.trim())
    .filter(Boolean);
}

export function formatDateForDashboardSummary(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value.trim());
  if (!match) {
    return value;
  }
  return `${match[3]}.${match[2]}.${match[1]}`;
}

export function sanitizeDashboardSummaryText(text: string, options: SummaryDisplayOptions = {}): string {
  if (!text.trim()) {
    return "";
  }

  let output = normalizeWhitespace(text);
  output = output.replace(isoDatePattern, (_, year: string, month: string, day: string) => `${day}.${month}.${year}`);
  output = replaceMetricKeywords(output);
  output = replaceCountKeywords(output);
  output = replaceMoneyRanges(output);
  output = replaceMoneyKeywords(output);
  output = replaceMoneyLikeDecimalsInContext(output);
  output = replacePlainPhrases(output);
  output = replaceAwkwardPhrases(output, options);

  return cleanupText(output);
}

export function splitDashboardSummaryIntoParagraphs(text: string, options: SummaryDisplayOptions = {}): string[] {
  const sanitized = sanitizeDashboardSummaryText(text, options);
  if (!sanitized) {
    return [];
  }

  const sentences = sentenceSplit(sanitized);
  if (sentences.length <= 2) {
    return [sanitized];
  }

  const paragraphs: string[] = [];
  for (let index = 0; index < sentences.length; index += 2) {
    paragraphs.push(sentences.slice(index, index + 2).join(" ").trim());
  }
  return paragraphs;
}

export function buildDashboardSummaryView(text: string, expanded: boolean, options: SummaryDisplayOptions = {}): SummaryView {
  const paragraphs = splitDashboardSummaryIntoParagraphs(text, options);
  const sanitized = paragraphs.join(" ");
  const isExpandable = sanitized.length > COLLAPSED_CHARACTER_LIMIT || paragraphs.length > COLLAPSED_PARAGRAPH_LIMIT;

  if (!isExpandable) {
    return {
      paragraphs,
      isExpandable: false,
      toggleLabel: null,
    };
  }

  if (expanded) {
    return {
      paragraphs,
      isExpandable: true,
      toggleLabel: "Свернуть",
    };
  }

  const previewParagraphs = paragraphs.slice(0, COLLAPSED_PARAGRAPH_LIMIT);

  return {
    paragraphs: truncateParagraphs(previewParagraphs, COLLAPSED_CHARACTER_LIMIT),
    isExpandable: true,
    toggleLabel: "Подробнее",
  };
}
