import { asArray } from "@/lib/as-array";
import { sanitizeSellerFacingText } from "@/lib/seller-facing-text";
import { formatNarrativeSentence } from "@/lib/seller-pulse-view";
import type { DashboardHomeResponse, WhatChangedGroup, WhatChangedItem } from "@/lib/dashboard-home-types";

export type WhatChangedTone = "negative" | "warning" | "neutral" | "positive" | "empty_positive";

export type WhatChangedGroupView = {
  key: string;
  label: string;
  tone: WhatChangedTone;
  summary: string;
  items: WhatChangedItem[];
  remainingCount: number;
  emptyText: string;
  totalCount: number;
};

export type WhatChangedView = {
  summary: string;
  groups: WhatChangedGroupView[];
};

const GROUP_ORDER = ["worsened", "new_risks", "unchanged_risks", "improved"] as const;
const LEGACY_GROUP_KEYS: ReadonlySet<string> = new Set(GROUP_ORDER);
export const WHAT_CHANGED_ITEM_LIMIT = 3;

const SUMMARY_EMPTY = "AI-вывод появится после обновления сводки.";
const THEMATIC_GROUP_EMPTY_TEXT = "Заметных изменений не выявлено.";

const DEFAULT_EMPTY_TEXT: Record<(typeof GROUP_ORDER)[number], string> = {
  worsened: "Заметных ухудшений не выявлено.",
  new_risks: "Новых рисков не выявлено. В текущей неделе не появились новые материальные проблемы.",
  unchanged_risks:
    "Недостаточно истории публикаций. Это первая доступная AI-сводка, поэтому пока невозможно определить, какие проблемы сохраняются из предыдущих отчётов.",
  improved: "Заметных улучшений за неделю не выявлено.",
};

const TONE_BORDER: Record<WhatChangedTone, string> = {
  negative: "border-red-200/90",
  warning: "border-amber-200/90",
  neutral: "border-slate-200/90",
  positive: "border-emerald-200/90",
  empty_positive: "border-slate-200/70",
};

const TONE_BG: Record<WhatChangedTone, string> = {
  negative: "bg-red-50/80",
  warning: "bg-amber-50/70",
  neutral: "bg-slate-50/80",
  positive: "bg-emerald-50/70",
  empty_positive: "bg-slate-50/40",
};

const TONE_ACCENT: Record<WhatChangedTone, string> = {
  negative: "bg-[#f04438]",
  warning: "bg-[#f79009]",
  neutral: "bg-slate-400",
  positive: "bg-[#12b76a]",
  empty_positive: "bg-slate-300",
};

const TONE_ICON: Record<WhatChangedTone, string> = {
  negative: "text-[#b42318] bg-[#fef3f2]",
  warning: "text-[#b54708] bg-[#fffaeb]",
  neutral: "text-slate-600 bg-slate-100",
  positive: "text-[#027a48] bg-[#ecfdf3]",
  empty_positive: "text-aivion-muted-soft bg-aivion-surface-soft",
};

export function getWhatChangedToneBorder(tone: WhatChangedTone): string {
  return TONE_BORDER[tone] ?? TONE_BORDER.neutral;
}

export function getWhatChangedToneBackground(tone: WhatChangedTone): string {
  return TONE_BG[tone] ?? TONE_BG.neutral;
}

export function getWhatChangedToneAccent(tone: WhatChangedTone): string {
  return TONE_ACCENT[tone] ?? TONE_ACCENT.neutral;
}

export function getWhatChangedToneIconClass(tone: WhatChangedTone): string {
  return TONE_ICON[tone] ?? TONE_ICON.neutral;
}

export function normalizeWhatChangedTone(raw?: string | null): WhatChangedTone {
  switch ((raw ?? "").trim().toLowerCase()) {
    case "negative":
      return "negative";
    case "warning":
      return "warning";
    case "positive":
      return "positive";
    default:
      return "neutral";
  }
}

export function buildWhatChangedView(
  whatChanged: DashboardHomeResponse["ai_diagnostics"]["what_changed"],
  options?: { expandedGroups?: ReadonlySet<string> },
): WhatChangedView {
  const rawGroups = asArray(whatChanged.groups).filter(
    (group): group is WhatChangedGroup => Boolean(group?.key),
  );

  const groups = hasLegacyWhatChangedGroups(rawGroups)
    ? buildLegacyWhatChangedGroups(rawGroups, options?.expandedGroups)
    : buildThematicWhatChangedGroups(rawGroups, options?.expandedGroups);

  const rawSummary = (whatChanged.main_conclusion ?? whatChanged.summary ?? "").trim();
  // Preserve paragraph breaks for managerial multi-paragraph conclusions.
  const summary = rawSummary
    ? formatMainConclusionText(sanitizeSellerFacingText(rawSummary))
    : SUMMARY_EMPTY;

  return {
    summary,
    groups,
  };
}

function hasLegacyWhatChangedGroups(groups: WhatChangedGroup[]): boolean {
  return groups.some((group) => LEGACY_GROUP_KEYS.has(group.key));
}

function buildLegacyWhatChangedGroups(
  rawGroups: WhatChangedGroup[],
  expandedGroups?: ReadonlySet<string>,
): WhatChangedGroupView[] {
  const groupsByKey = new Map<string, WhatChangedGroup>();
  for (const group of rawGroups) {
    groupsByKey.set(group.key, group);
  }

  return GROUP_ORDER.map((key) => {
    const group = groupsByKey.get(key);
    const allItems = asArray(group?.items).map(sanitizeWhatChangedItem);
    const expanded = expandedGroups?.has(key) ?? false;
    const items = expanded ? allItems : allItems.slice(0, WHAT_CHANGED_ITEM_LIMIT);
    const remainingCount = Math.max(0, allItems.length - items.length);
    const hasItems = allItems.length > 0;
    const rawTone = normalizeWhatChangedTone(group?.tone);
    const tone: WhatChangedTone = key === "improved" && !hasItems ? "empty_positive" : rawTone;
    const emptyText = sanitizeEmptyText(key, (group?.empty_text ?? "").trim() || DEFAULT_EMPTY_TEXT[key]);
    const summary = sanitizeSellerFacingText(group?.summary ?? "");

    const resolvedSummary =
      key === "improved" && !hasItems
        ? DEFAULT_EMPTY_TEXT.improved
        : summary || emptyText;

    return {
      key,
      label: resolveGroupLabel(key, group?.label, hasItems),
      tone,
      summary: hasItems ? "" : resolvedSummary,
      items,
      remainingCount,
      emptyText,
      totalCount: allItems.length,
    };
  });
}

function buildThematicWhatChangedGroups(
  rawGroups: WhatChangedGroup[],
  expandedGroups?: ReadonlySet<string>,
): WhatChangedGroupView[] {
  return rawGroups.map((group) => {
    const allItems = asArray(group.items).map(sanitizeWhatChangedItem);
    const expanded = expandedGroups?.has(group.key) ?? false;
    const items = expanded ? allItems : allItems.slice(0, WHAT_CHANGED_ITEM_LIMIT);
    const remainingCount = Math.max(0, allItems.length - items.length);
    const hasItems = allItems.length > 0;
    const tone = normalizeWhatChangedTone(group.tone);
    const emptyText = (group.empty_text ?? "").trim() || THEMATIC_GROUP_EMPTY_TEXT;
    const summary = sanitizeSellerFacingText(group.summary ?? "");

    return {
      key: group.key,
      label: (group.label ?? "").trim() || group.key,
      tone,
      summary: hasItems ? "" : summary || emptyText,
      items,
      remainingCount,
      emptyText,
      totalCount: allItems.length,
    };
  });
}

function sanitizeEmptyText(key: string, text: string): string {
  const lower = text.toLowerCase();
  if (key === "unchanged_risks") {
    if (
      lower.includes("нет долговремен") ||
      lower.includes("нет долгоживущ") ||
      lower.includes("за пределами текущего") ||
      lower.includes("не выявлено")
    ) {
      return DEFAULT_EMPTY_TEXT.unchanged_risks;
    }
  }
  return text;
}

function sanitizeWhatChangedItem(item: WhatChangedItem): WhatChangedItem {
  return {
    ...item,
    label: sanitizeSellerFacingText(item.label),
    text: sanitizeSellerFacingText(item.text),
    current_value_text: item.current_value_text ? sanitizeSellerFacingText(item.current_value_text) : item.current_value_text,
    previous_value_text: item.previous_value_text
      ? sanitizeSellerFacingText(item.previous_value_text)
      : item.previous_value_text,
    delta_text: item.delta_text ? sanitizeSellerFacingText(item.delta_text) : item.delta_text,
    impact_text: item.impact_text ? sanitizeSellerFacingText(item.impact_text) : item.impact_text,
    action_link_label: item.action_link_label ? sanitizeSellerFacingText(item.action_link_label) : item.action_link_label,
    entity_names: asArray(item.entity_names).map((n) => sanitizeSellerFacingText(n)).filter(Boolean),
  };
}

function resolveGroupLabel(key: string, rawLabel?: string | null, hasItems = false): string {
  if (key === "improved") {
    return "Улучшилось";
  }
  if (key === "unchanged_risks") {
    return "Активные проблемы";
  }
  const label = (rawLabel ?? defaultLabelForKey(key)).trim();
  if (label === "Сохраняются" || label === "Сохраняющиеся") {
    return "Активные проблемы";
  }
  if (label === "Точки опоры" || label === "Сильные стороны" || label === "Точки роста") {
    return "Улучшилось";
  }
  return label || defaultLabelForKey(key);
}

function defaultLabelForKey(key: string): string {
  switch (key) {
    case "worsened":
      return "Ухудшилось";
    case "new_risks":
      return "Новые риски";
    case "unchanged_risks":
      return "Активные проблемы";
    case "improved":
      return "Улучшилось";
    default:
      return key;
  }
}

export function labelHasCount(label: string): boolean {
  return /[·•]/.test(label);
}

export type WhatChangedTextSegment = {
  text: string;
  bold: boolean;
};

export function splitWhatChangedItemText(text: string, highlights: string[]): WhatChangedTextSegment[] {
  const normalizedHighlights = asArray(highlights)
    .map((value) => value.trim())
    .filter(Boolean)
    .sort((a, b) => b.length - a.length);

  if (normalizedHighlights.length === 0) {
    return [{ text, bold: false }];
  }

  const segments: WhatChangedTextSegment[] = [];
  let cursor = 0;
  while (cursor < text.length) {
    let nextIndex = -1;
    let matched = "";
    for (const highlight of normalizedHighlights) {
      const index = text.indexOf(highlight, cursor);
      if (index === -1) {
        continue;
      }
      if (nextIndex === -1 || index < nextIndex) {
        nextIndex = index;
        matched = highlight;
      }
    }
    if (nextIndex === -1) {
      segments.push({ text: text.slice(cursor), bold: false });
      break;
    }
    if (nextIndex > cursor) {
      segments.push({ text: text.slice(cursor, nextIndex), bold: false });
    }
    segments.push({ text: matched, bold: true });
    cursor = nextIndex + matched.length;
  }
  return segments.filter((segment) => segment.text.length > 0);
}

export type WhatChangedItemDisplay = {
  title: string;
  subtitle?: string;
  fullText: string;
  comparison?: string;
  impact?: string;
  entityLine?: string;
  actionHref?: string;
  actionLabel?: string;
  severity?: string;
};

export function buildWhatChangedItemDisplay(item: WhatChangedItem): WhatChangedItemDisplay {
  const text = sanitizeSellerFacingText(item.text);
  const label = sanitizeSellerFacingText(item.label);

  let title = label;
  let subtitle = text || undefined;
  if (!title) {
    const colonIdx = text.indexOf(":");
    if (colonIdx > 0 && colonIdx < 48) {
      title = text.slice(0, colonIdx).trim();
      subtitle = text.slice(colonIdx + 1).trim() || undefined;
    } else {
      title = text;
      subtitle = undefined;
    }
  }

  const prev = (item.previous_value_text ?? "").trim();
  const cur = (item.current_value_text ?? "").trim();
  const delta = (item.delta_text ?? "").trim();
  let comparison: string | undefined;
  if (prev && cur) {
    comparison = `Было: ${prev} → Стало: ${cur}`;
  } else if (delta) {
    comparison = delta;
  } else if (cur) {
    comparison = `Сейчас: ${cur}`;
  }

  const names = asArray(item.entity_names).filter(Boolean).slice(0, 2);
  const entityCount = item.entity_count ?? 0;
  let entityLine: string | undefined;
  if (names.length > 0) {
    entityLine = names.map((n) => `«${n}»`).join(" и ");
    if (entityCount > names.length) {
      entityLine += ` и ещё ${entityCount - names.length}`;
    }
  } else if (entityCount > 0) {
    entityLine = `${entityCount} позиций`;
  }

  const linked = (item.linked_action_id ?? "").trim();
  const deep = (item.deep_link ?? "").trim();
  const actionHref = linked
    ? `/app/actions?action=${encodeURIComponent(linked)}`
    : deep || undefined;
  const actionLabel =
    (item.action_link_label ?? "").trim() ||
    (actionHref ? "Открыть действие" : undefined);

  return {
    title,
    subtitle,
    fullText: subtitle ? `${title}: ${subtitle}` : title,
    comparison,
    impact: (item.impact_text ?? "").trim() || undefined,
    entityLine,
    actionHref,
    actionLabel,
    severity: (item.severity ?? "").trim() || undefined,
  };
}

export function inferWhatChangedItemIconCategory(item: WhatChangedItem): string {
  if (item.domain) {
    const d = item.domain.toLowerCase();
    if (d === "advertising") return "advertising";
    if (d === "stock" || d === "surplus" || d === "no_sales") return "stock";
    if (d === "funnel") return "funnel";
    if (d === "sales") return "sales";
  }
  const haystack = `${item.label ?? ""} ${item.text ?? ""}`.toLowerCase();
  if (/реклам|кампан|дрр|бюджет/.test(haystack)) return "advertising";
  if (/остат|запас|sku|артикул|дефицит/.test(haystack)) return "stock";
  if (/воронк|корзин|карточк|переход|конверс/.test(haystack)) return "funnel";
  if (/отмен/.test(haystack)) return "cancellations";
  if (/выручк|заказ|продаж/.test(haystack)) return "sales";
  return "neutral";
}

export function severityBadgeLabel(severity?: string): string | undefined {
  switch ((severity ?? "").toLowerCase()) {
    case "critical":
      return "Критично";
    case "high":
      return "Высокий риск";
    case "medium":
      return "Средний риск";
    case "low":
      return "Низкий риск";
    default:
      return undefined;
  }
}

/** Format main conclusion without collapsing paragraph breaks. */
export function formatMainConclusionText(text: string): string {
  const normalized = text
    .replace(/\r\n/g, "\n")
    .split(/\n{2,}/)
    .map((para) => para.replace(/[ \t]+/g, " ").trim())
    .filter(Boolean)
    .join("\n\n");
  if (!normalized) {
    return "";
  }
  if (/\n\n/.test(normalized)) {
    return normalized;
  }
  return formatNarrativeSentence(normalized);
}

/** Split main conclusion into 2–4 short paragraphs for managerial reading. */
export function splitMainConclusionParagraphs(text: string): string[] {
  const normalized = sanitizeSellerFacingText(text).replace(/\s+/g, " ").trim();
  if (!normalized) {
    return [SUMMARY_EMPTY];
  }

  // Prefer explicit paragraph breaks when already present.
  const explicit = normalized
    .split(/\n+/)
    .map((part) => part.trim())
    .filter(Boolean);
  if (explicit.length >= 2) {
    return explicit.slice(0, 4);
  }

  // Avoid "Label: sentence. Label: sentence." dumps — split on ". " then regroup.
  const sentences = normalized
    .split(/(?<=[.!?…])\s+/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (sentences.length <= 1) {
    return [normalized];
  }

  const paragraphs: string[] = [];
  if (sentences.length <= 3) {
    return sentences;
  }
  // Headline / sales, then risks, then priority — group in pairs when many sentences.
  paragraphs.push(sentences[0]);
  if (sentences.length === 4) {
    paragraphs.push(sentences.slice(1, 3).join(" "));
    paragraphs.push(sentences[3]);
    return paragraphs;
  }
  paragraphs.push(sentences.slice(1, 3).join(" "));
  paragraphs.push(sentences.slice(3, -1).join(" ") || sentences[3]);
  if (sentences.length > 4) {
    paragraphs.push(sentences[sentences.length - 1]);
  }
  return paragraphs.filter(Boolean).slice(0, 4);
}
