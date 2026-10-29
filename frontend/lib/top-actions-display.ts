import type { TopActionItem } from "@/lib/dashboard-home-types";

const CATEGORY_LABELS: Record<string, string> = {
  advertising: "Реклама",
  stock: "Остатки и поставки",
  slow_turnover: "Оборачиваемость",
  sales: "Продажи",
  funnel: "Воронка",
  cancellations: "Отмены",
  products: "SKU",
  account: "Аккаунт",
  other: "Другое",
};

export function getActionCategory(action: TopActionItem): string {
  const label = (action.category_label ?? "").trim();
  if (label) {
    return label;
  }
  const key = (action.category ?? "").trim().toLowerCase();
  return CATEGORY_LABELS[key] ?? key;
}

export type ActionVisualTone = {
  name: "red" | "amber" | "blue" | "violet" | "slate";
  border: string;
  softBg: string;
  softBorder: string;
  text: string;
  badge: string;
  priorityBadge: string;
};

function buildActionTextContext(action: TopActionItem): string {
  return [
    action.category ?? "",
    action.category_label ?? "",
    action.title ?? "",
    action.reason ?? "",
  ]
    .join(" ")
    .toLowerCase();
}

export function getActionVisualTone(action: TopActionItem): ActionVisualTone {
  const text = buildActionTextContext(action);
  const rawCategory = (action.category ?? "").trim().toLowerCase();

  if (
    text.includes("реклам") ||
    text.includes("ads") ||
    text.includes("advertising") ||
    text.includes("campaign") ||
    rawCategory === "advertising"
  ) {
    return {
      name: "red",
      border: "border-l-4 border-l-red-500",
      softBg: "bg-red-50",
      softBorder: "border-red-100",
      text: "text-red-600",
      badge: "bg-red-50 text-red-700 border-red-100",
      priorityBadge: "bg-red-50 text-red-700 border-red-100",
    };
  }

  if (
    text.includes("оборачива") ||
    text.includes("slow_turnover") ||
    rawCategory === "slow_turnover"
  ) {
    return {
      name: "amber",
      border: "border-l-4 border-l-amber-500",
      softBg: "bg-amber-50",
      softBorder: "border-amber-100",
      text: "text-amber-700",
      badge: "bg-amber-50 text-amber-800 border-amber-100",
      priorityBadge: "bg-amber-50 text-amber-800 border-amber-100",
    };
  }

  if (
    text.includes("остат") ||
    text.includes("stock") ||
    text.includes("inventory") ||
    text.includes("дефицит") ||
    text.includes("replenishment") ||
    rawCategory === "stock"
  ) {
    return {
      name: "amber",
      border: "border-l-4 border-l-amber-500",
      softBg: "bg-amber-50",
      softBorder: "border-amber-100",
      text: "text-amber-600",
      badge: "bg-amber-50 text-amber-700 border-amber-100",
      priorityBadge: "bg-amber-50 text-amber-700 border-amber-100",
    };
  }

  if (
    text.includes("ворон") ||
    text.includes("funnel") ||
    text.includes("conversion") ||
    text.includes("корзин") ||
    rawCategory === "funnel"
  ) {
    return {
      name: "blue",
      border: "border-l-4 border-l-blue-500",
      softBg: "bg-blue-50",
      softBorder: "border-blue-100",
      text: "text-blue-600",
      badge: "bg-blue-50 text-blue-700 border-blue-100",
      priorityBadge: "bg-blue-50 text-blue-700 border-blue-100",
    };
  }

  if (
    text.includes("sku") ||
    text.includes("продаж") ||
    text.includes("sales") ||
    text.includes("orders") ||
    rawCategory === "sales" ||
    rawCategory === "products"
  ) {
    return {
      name: "violet",
      border: "border-l-4 border-l-violet-500",
      softBg: "bg-violet-50",
      softBorder: "border-violet-100",
      text: "text-violet-600",
      badge: "bg-violet-50 text-violet-700 border-violet-100",
      priorityBadge: "bg-violet-50 text-violet-700 border-violet-100",
    };
  }

  return {
    name: "slate",
    border: "border-l-4 border-l-slate-300",
    softBg: "bg-slate-50",
    softBorder: "border-slate-100",
    text: "text-slate-600",
    badge: "bg-slate-50 text-slate-700 border-slate-100",
    priorityBadge: "bg-slate-50 text-slate-700 border-slate-100",
  };
}

export function getTopActionAccentClass(action: TopActionItem): string {
  return getActionVisualTone(action).border;
}

export function getEvidenceGridClass(metricCount: number): string {
  if (metricCount === 2) {
    return "grid-cols-2";
  }
  if (metricCount === 3) {
    return "grid-cols-3";
  }
  if (metricCount >= 4) {
    return "grid-cols-2";
  }
  return "grid-cols-1";
}

export function normalizeMetricLabel(label: string): string {
  return label.trim().toLowerCase();
}

export type TopActionCTA = {
  href: string;
  label: string;
};

export function getTopActionCTA(action: TopActionItem): TopActionCTA | null {
  const actionKey = (action.action_key ?? "").trim();
  if (actionKey) {
    const params = new URLSearchParams({ action: actionKey });
    return { href: `/app/actions?${params.toString()}`, label: "Открыть действие" };
  }
  if (action.action_id) {
    return { href: `/app/actions?action=${encodeURIComponent(String(action.action_id))}`, label: "Открыть действие" };
  }
  return { href: "/app/actions", label: "Открыть действия" };
}

export function getTopActionMetricKey(
  action: TopActionItem,
  metric: { label: string; value: string },
  index: number,
): string {
  return [action.action_key ?? action.id, metric.label, metric.value, index].filter(Boolean).join("-");
}
