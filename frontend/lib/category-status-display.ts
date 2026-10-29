import type { LucideIcon } from "lucide-react";
import {
  Archive,
  Bath,
  Box,
  Brush,
  Droplets,
  Grid2X2,
  Lamp,
  Layers,
  LayoutGrid,
  Sparkles,
  Sun,
} from "lucide-react";

export type CategoryVisual = {
  icon: LucideIcon;
  iconClassName: string;
  badgeClassName: string;
};

export type SeverityBadgeConfig = {
  label: string;
  className: string;
};

export type CategoryHealthScoreDisplay = {
  value: string;
  hasScore: boolean;
};

export type CategoryConditionScoreTone = "high" | "medium" | "low" | "none" | "unknown";

export type CategoryConditionScoreDisplay = {
  value: number | null;
  label: string;
  ariaLabel: string;
  tone: CategoryConditionScoreTone;
};

const scoreFormatter = new Intl.NumberFormat("ru-RU", {
  maximumFractionDigits: 0,
});

const FALLBACK_CATEGORY_NAME = "Категория без названия";

const SECTION_FALLBACKS = {
  headline: "Недостаточно данных для краткого вывода.",
  key_risk: "Явные риски не выделены.",
  recommended_action: "Действия не сформированы.",
} as const;

function normalizeSeverity(severity?: string | null): string {
  return (severity ?? "").trim().toLowerCase().replace(/\s+/g, "_").replace(/-/g, "_");
}

function normalizeCategoryName(categoryName?: string | null): string {
  return (categoryName ?? "").trim().toLowerCase();
}

function isValidHealthScore(score: unknown): score is number {
  return typeof score === "number" && Number.isFinite(score);
}

function clampHealthScore(score?: number | null): number | null {
  if (!isValidHealthScore(score)) {
    return null;
  }
  return Math.max(0, Math.min(100, Math.round(score)));
}

function severityToTone(severity?: string | null): CategoryConditionScoreTone | null {
  switch (normalizeSeverity(severity)) {
    case "critical":
    case "high":
      return "high";
    case "medium":
      return "medium";
    case "low":
      return "low";
    case "stable":
      return "none";
    default:
      return null;
  }
}

function score10ToTone(score10: number): CategoryConditionScoreTone {
  if (score10 <= 3) {
    return "high";
  }
  if (score10 <= 6) {
    return "medium";
  }
  if (score10 <= 9) {
    return "low";
  }
  return "none";
}

function resolveCategoryScore10(
  healthScore100: unknown,
  categoryScore?: number | null,
): number | null {
  if (typeof categoryScore === "number" && Number.isFinite(categoryScore)) {
    return Math.max(0, Math.min(10, Math.round(categoryScore)));
  }
  return null;
}

function resolveCategoryConditionTone(
  healthScore100: unknown,
  severity?: string | null,
  categoryScore?: number | null,
): CategoryConditionScoreTone {
  const score10 = resolveCategoryScore10(healthScore100, categoryScore);
  if (score10 != null) {
    return score10ToTone(score10);
  }
  return severityToTone(severity) ?? "unknown";
}

const CATEGORY_RISK_BADGE_BY_TONE: Record<CategoryConditionScoreTone, SeverityBadgeConfig> = {
  high: {
    label: "Высокий риск",
    className: "border-[#FED7AA] bg-[#FFF7ED] text-[#EA580C]",
  },
  medium: {
    label: "Средний риск",
    className: "border-[#FDE68A] bg-[#FFFBEB] text-[#D97706]",
  },
  low: {
    label: "Небольшой риск",
    className: "border-[#BBF7D0] bg-[#ECFDF5] text-[#059669]",
  },
  none: {
    label: "Нет значимых рисков",
    className: "border-[#BBF7D0] bg-[#F0FDF4] text-[#15803D]",
  },
  unknown: {
    label: "Без оценки",
    className: "border-[#E2E8F0] bg-[#F8FAFC] text-[#64748B]",
  },
};

export function formatCategoryConditionScore(
  healthScore100: unknown,
  severity?: string | null,
  categoryScore?: number | null,
): CategoryConditionScoreDisplay {
  const score10 = resolveCategoryScore10(healthScore100, categoryScore);
  const tone = resolveCategoryConditionTone(healthScore100, severity, categoryScore);

  if (score10 == null) {
    return {
      value: null,
      label: "— / 10",
      ariaLabel: "Оценка состояния категории недоступна",
      tone,
    };
  }

  return {
    value: score10,
    label: `${score10} / 10`,
    ariaLabel: `Оценка состояния категории: ${score10} из 10`,
    tone,
  };
}

export function getCategoryConditionRiskBadge(
  healthScore100: unknown,
  severity?: string | null,
  categoryScore?: number | null,
): SeverityBadgeConfig {
  const tone = resolveCategoryConditionTone(healthScore100, severity, categoryScore);
  return CATEGORY_RISK_BADGE_BY_TONE[tone];
}

export function getCategoryConditionScoreColor(tone: CategoryConditionScoreTone): string {
  switch (tone) {
    case "high":
      return "text-[#EA580C]";
    case "medium":
      return "text-[#D97706]";
    case "low":
      return "text-[#16A34A]";
    case "none":
      return "text-[#15803D]";
    default:
      return "text-[#64748B]";
  }
}

export function getCategoryDisplayName(categoryName?: string | null): string {
  const name = (categoryName ?? "").trim();
  return name || FALLBACK_CATEGORY_NAME;
}

export function formatCategoryHealthScore(score?: number | null): CategoryHealthScoreDisplay {
  const clamped = clampHealthScore(score);
  if (clamped == null) {
    return { value: "—", hasScore: false };
  }
  return { value: scoreFormatter.format(clamped), hasScore: true };
}

export function getCategoryScoreColor(severity?: string | null, score?: number | null): string {
  switch (normalizeSeverity(severity)) {
    case "critical":
    case "high":
      return "text-[#EA580C]";
    case "medium":
      return "text-[#D97706]";
    case "low":
    case "stable":
      return "text-[#16A34A]";
    default:
      break;
  }

  const clamped = clampHealthScore(score);
  if (clamped == null) {
    return "text-[#64748B]";
  }
  if (clamped >= 70) {
    return "text-[#16A34A]";
  }
  if (clamped >= 50) {
    return "text-[#D97706]";
  }
  return "text-[#EA580C]";
}

export function getSeverityBadgeConfig(severity?: string | null): SeverityBadgeConfig {
  switch (normalizeSeverity(severity)) {
    case "critical":
    case "high":
      return {
        label: "Высокий риск",
        className: "border-[#FED7AA] bg-[#FFF7ED] text-[#EA580C]",
      };
    case "medium":
      return {
        label: "Средний риск",
        className: "border-[#FDE68A] bg-[#FFFBEB] text-[#D97706]",
      };
    case "low":
    case "stable":
      return {
        label: "Стабильно",
        className: "border-[#A7F3D0] bg-[#ECFDF5] text-[#059669]",
      };
    default:
      return {
        label: "Без оценки",
        className: "border-[#E2E8F0] bg-[#F8FAFC] text-[#64748B]",
      };
  }
}

export function getCategoryVisual(categoryName: string): CategoryVisual {
  const name = normalizeCategoryName(categoryName);

  if (name.includes("текстиль")) {
    return {
      icon: Layers,
      iconClassName: "text-[#7C3AED]",
      badgeClassName: "bg-[#F3E8FF]",
    };
  }
  if (name.includes("кухн") || name.includes("хранен")) {
    return {
      icon: Archive,
      iconClassName: "text-[#0F766E]",
      badgeClassName: "bg-[#CCFBF1]",
    };
  }
  if (name.includes("организац") || name.includes("пространств")) {
    return {
      icon: LayoutGrid,
      iconClassName: "text-[#2563EB]",
      badgeClassName: "bg-[#DBEAFE]",
    };
  }
  if (name.includes("ванн")) {
    return {
      icon: Bath,
      iconClassName: "text-[#7C3AED]",
      badgeClassName: "bg-[#EDE9FE]",
    };
  }
  if (name.includes("уборк")) {
    return {
      icon: Sparkles,
      iconClassName: "text-[#16A34A]",
      badgeClassName: "bg-[#DCFCE7]",
    };
  }
  if (name.includes("освещ") || name.includes("декор")) {
    return {
      icon: Lamp,
      iconClassName: "text-[#D97706]",
      badgeClassName: "bg-[#FEF3C7]",
    };
  }
  if (name.includes("вода") || name.includes("сантех")) {
    return {
      icon: Droplets,
      iconClassName: "text-[#2563EB]",
      badgeClassName: "bg-[#DBEAFE]",
    };
  }
  if (name.includes("короб") || name.includes("упаков")) {
    return {
      icon: Box,
      iconClassName: "text-[#0F766E]",
      badgeClassName: "bg-[#CCFBF1]",
    };
  }
  if (name.includes("чист")) {
    return {
      icon: Brush,
      iconClassName: "text-[#16A34A]",
      badgeClassName: "bg-[#DCFCE7]",
    };
  }
  if (name.includes("свет") || name.includes("ламп")) {
    return {
      icon: Sun,
      iconClassName: "text-[#D97706]",
      badgeClassName: "bg-[#FFEDD5]",
    };
  }

  return {
    icon: Grid2X2,
    iconClassName: "text-[#475569]",
    badgeClassName: "bg-[#F1F5F9]",
  };
}

export function normalizeDisplayText(text?: string | null): string | null {
  const value = (text ?? "").trim();
  return value || null;
}

function cleanDisplayItem(value: string): string {
  return value
    .trim()
    .replace(/^[-•–*]\s*/, "")
    .replace(/[.;,]+$/, "")
    .trim();
}

function capitalizeFirst(value: string): string {
  if (!value) {
    return value;
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function stripActionLeadIn(value: string): string {
  return value
    .trim()
    .replace(/^в\s+первую\s+очередь\s+/iu, "")
    .replace(/^сначала\s+/iu, "")
    .trim();
}

function isQualitySplit(items: string[]): boolean {
  if (items.length < 2) {
    return false;
  }

  const minLength = 18;
  return items.every((item) => item.length >= minLength);
}

function normalizeActionDelimiters(text: string): string {
  return text
    .replace(/\s*,?\s+и\s+отдельно\s+/giu, "; ")
    .replace(/\.\s+(?:затем|после\s+этого)\s+/giu, "; ")
    .replace(/\s+(?:затем|после\s+этого|отдельно)\s+/giu, "; ");
}

export type SplitTextOptions = {
  maxItems?: number;
  mode?: "default" | "action";
  /** Used to strip duplicated category name from «Состояние» headlines. */
  categoryName?: string | null;
};

/**
 * Turns legacy "Category · 35% выручки" into "Формирует 35% выручки"
 * so the card title is not repeated inside the Состояние section.
 */
export function formatCategoryStateHeadline(
  headline: string,
  categoryName?: string | null,
): string {
  const text = headline.trim();
  if (!text) return text;

  const shareMatch = text.match(/^(.+?)\s*·\s*(\d+(?:[.,]\d+)?)%\s*выручки\s*$/iu);
  if (!shareMatch) return text;

  const prefix = (shareMatch[1] ?? "").trim();
  const pct = shareMatch[2] ?? "";
  const name = (categoryName ?? "").trim();
  if (name && prefix.toLowerCase() !== name.toLowerCase()) {
    return text;
  }
  return `Формирует ${pct}% выручки`;
}

export function splitTextToDisplayItems(text: string, options: SplitTextOptions = {}): string[] {
  const maxItems = options.maxItems ?? 2;
  const trimmed = (text ?? "").trim();
  if (!trimmed) {
    return [];
  }

  const finalize = (items: string[]) => {
    const normalized =
      options.mode === "action"
        ? items.map((item) => capitalizeFirst(cleanDisplayItem(stripActionLeadIn(item))))
        : items.map((item) => capitalizeFirst(cleanDisplayItem(item)));

    return normalized.filter(Boolean).slice(0, maxItems);
  };

  const trySplit = (items: string[], strict = true): string[] | null => {
    const parts = finalize(items);
    if (parts.length > 1 && (!strict || isQualitySplit(parts))) {
      return parts;
    }
    return null;
  };

  if (trimmed.includes(";")) {
    const parts = trySplit(trimmed.split(";"), false);
    if (parts) {
      return parts;
    }
  }

  const bulletParts = trimmed
    .split(/\n?\s*[-•–*]\s+/)
    .map(cleanDisplayItem)
    .filter(Boolean);
  if (bulletParts.length > 1) {
    const parts = trySplit(bulletParts, false);
    if (parts) {
      return parts;
    }
  }

  if (options.mode === "action") {
    const normalizedAction = normalizeActionDelimiters(trimmed);
    if (normalizedAction.includes(";")) {
      const parts = trySplit(normalizedAction.split(";"), false);
      if (parts) {
        return parts;
      }
    }

    const separatelyParts = trimmed.split(/\s*,?\s+и\s+отдельно\s+/iu);
    if (separatelyParts.length > 1) {
      const parts = trySplit(separatelyParts);
      if (parts) {
        return parts;
      }
    }
  }

  const sentences = trimmed
    .split(/(?<=[.!?…])\s+/)
    .map(cleanDisplayItem)
    .filter((sentence) => sentence.length >= 24);
  if (sentences.length > 1) {
    const parts = trySplit(sentences);
    if (parts) {
      return parts;
    }
  }

  return finalize([trimmed]);
}

const SECTION_MAX_ITEMS: Record<keyof typeof SECTION_FALLBACKS, number> = {
  headline: 1,
  key_risk: 2,
  recommended_action: 2,
};

export function getCategoryCTA(_category?: { category_id?: string }): { href: string } | null {
  void _category;
  // Legacy /app/products routes are retired from seller-facing dashboard CTAs.
  return null;
}

export function getCategorySectionText(
  field: keyof typeof SECTION_FALLBACKS,
  text?: string | null,
  options?: SplitTextOptions,
): { items: string[]; isFallback: boolean } {
  const normalized = normalizeDisplayText(text);
  if (!normalized) {
    return { items: [SECTION_FALLBACKS[field]], isFallback: true };
  }

  if (field === "headline") {
    return {
      items: [formatCategoryStateHeadline(normalized, options?.categoryName)],
      isFallback: false,
    };
  }

  const items = splitTextToDisplayItems(normalized, {
    maxItems: options?.maxItems ?? SECTION_MAX_ITEMS[field],
    mode: field === "recommended_action" ? "action" : "default",
  });

  return {
    items: items.length > 0 ? items : [normalized],
    isFallback: false,
  };
}
