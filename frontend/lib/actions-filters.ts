import type { ActionCategory, ListActionsParams } from "@/lib/actions-types";
import type {
  ActionCategoryFilter,
  ActionSortKey,
  ActionStatusFilter,
} from "@/lib/actions-view-model";

export const ACTIONS_PAGE_LIMIT = 50;

export const CATEGORY_FILTER_OPTIONS: Array<{
  value: ActionCategoryFilter;
  label: string;
  counterKey:
    | "total_open"
    | "stock"
    | "slow_turnover"
    | "sku_sales"
    | "advertising"
    | "funnel"
    | "category_count";
}> = [
  { value: "all", label: "Все", counterKey: "total_open" },
  { value: "stock", label: "Остатки и поставки", counterKey: "stock" },
  { value: "slow_turnover", label: "Оборачиваемость", counterKey: "slow_turnover" },
  { value: "sku_sales", label: "SKU и продажи", counterKey: "sku_sales" },
  { value: "advertising", label: "Реклама", counterKey: "advertising" },
  { value: "funnel", label: "Воронка и карточки", counterKey: "funnel" },
  { value: "category", label: "Категории", counterKey: "category_count" },
];

export const STATUS_FILTER_OPTIONS: Array<{ value: ActionStatusFilter; label: string }> = [
  { value: "active", label: "Активные" },
  { value: "completed", label: "Выполненные" },
  { value: "irrelevant", label: "Неактуальные" },
];

export type ActionsWorkspaceQuery = {
  status: ActionStatusFilter;
  category: ActionCategoryFilter;
  sort: ActionSortKey;
  q: string;
  action: string | null;
};

/** Legacy single `filter` param → split status/category. */
export function migrateLegacyFilter(
  rawFilter: string | null,
): Pick<ActionsWorkspaceQuery, "status" | "category"> | null {
  if (!rawFilter) return null;
  switch (rawFilter) {
    case "open":
      return { status: "active", category: "all" };
    case "stock":
    case "slow_turnover":
    case "sku_sales":
    case "advertising":
    case "funnel":
    case "category":
      return { status: "active", category: rawFilter };
    case "checked":
      return { status: "completed", category: "all" };
    case "dismissed":
      return { status: "irrelevant", category: "all" };
    default:
      return null;
  }
}

export function normalizeStatusFilter(raw: string | null): ActionStatusFilter {
  if (raw === "completed" || raw === "checked") return "completed";
  if (raw === "irrelevant" || raw === "dismissed") return "irrelevant";
  if (raw === "active" || raw === "open") return "active";
  return "active";
}

export function normalizeCategoryFilter(raw: string | null): ActionCategoryFilter {
  const allowed: ActionCategoryFilter[] = [
    "all",
    "stock",
    "slow_turnover",
    "sku_sales",
    "advertising",
    "funnel",
    "category",
  ];
  if (raw && allowed.includes(raw as ActionCategoryFilter)) return raw as ActionCategoryFilter;
  return "all";
}

export function normalizeSortKey(raw: string | null): ActionSortKey {
  const allowed: ActionSortKey[] = ["priority", "newest", "oldest", "category", "entities"];
  if (raw && allowed.includes(raw as ActionSortKey)) return raw as ActionSortKey;
  return "priority";
}

export function parseActionsWorkspaceQuery(searchParams: URLSearchParams): ActionsWorkspaceQuery {
  const legacy = migrateLegacyFilter(searchParams.get("filter"));
  const status = searchParams.get("status")
    ? normalizeStatusFilter(searchParams.get("status"))
    : legacy?.status ?? "active";
  const category = searchParams.get("category")
    ? normalizeCategoryFilter(searchParams.get("category"))
    : legacy?.category ?? "all";
  return {
    status,
    category,
    sort: normalizeSortKey(searchParams.get("sort")),
    q: searchParams.get("q") ?? "",
    action: searchParams.get("action") ?? searchParams.get("action_id"),
  };
}

export function statusToApiStatus(status: ActionStatusFilter): ListActionsParams["status"] {
  switch (status) {
    case "completed":
      return "checked";
    case "irrelevant":
      return "dismissed";
    default:
      return "open";
  }
}

export function getListRequestParams(
  status: ActionStatusFilter,
  category: ActionCategoryFilter,
): ListActionsParams[] {
  const base = {
    metric_date: "latest",
    limit: ACTIONS_PAGE_LIMIT,
    offset: 0,
    status: statusToApiStatus(status),
  } as const;

  if (status !== "active" || category === "all") {
    return [{ ...base }];
  }

  if (category === "sku_sales") {
    return [
      { ...base, category: "sku" satisfies ActionCategory },
      { ...base, category: "sales" satisfies ActionCategory },
    ];
  }

  return [{ ...base, category: category as ActionCategory }];
}

export function counterForCategory(
  category: ActionCategoryFilter,
  counters: {
    total_open: number;
    stock: number;
    slow_turnover: number;
    sku: number;
    sales: number;
    advertising: number;
    funnel: number;
    category_count: number;
  },
): number {
  switch (category) {
    case "all":
      return counters.total_open;
    case "stock":
      return counters.stock;
    case "slow_turnover":
      return counters.slow_turnover;
    case "sku_sales":
      return counters.sku + counters.sales;
    case "advertising":
      return counters.advertising;
    case "funnel":
      return counters.funnel;
    case "category":
      return counters.category_count;
    default:
      return 0;
  }
}

export function counterForStatus(
  status: ActionStatusFilter,
  counters: { total_open: number; checked: number; dismissed: number },
): number {
  switch (status) {
    case "completed":
      return counters.checked;
    case "irrelevant":
      return counters.dismissed;
    default:
      return counters.total_open;
  }
}

/** Keep backward-compat exports used by older tests. */
export type ActionsFilterKey =
  | "open"
  | "stock"
  | "slow_turnover"
  | "sku_sales"
  | "advertising"
  | "funnel"
  | "category"
  | "checked"
  | "dismissed";

export const ACTIONS_FILTER_KEYS: ActionsFilterKey[] = [
  "open",
  "stock",
  "slow_turnover",
  "sku_sales",
  "advertising",
  "funnel",
  "category",
  "checked",
  "dismissed",
];

export const CATEGORY_FILTER_KEYS: ActionsFilterKey[] = [
  "open",
  "stock",
  "slow_turnover",
  "sku_sales",
  "advertising",
  "funnel",
  "category",
];

export const STATUS_FILTER_KEYS: ActionsFilterKey[] = ["checked", "dismissed"];

export function isCategoryFilter(filter: ActionsFilterKey): boolean {
  return CATEGORY_FILTER_KEYS.includes(filter);
}

export function isStatusFilter(filter: ActionsFilterKey): boolean {
  return STATUS_FILTER_KEYS.includes(filter);
}

export function normalizeFilterKey(raw: string | null): ActionsFilterKey {
  if (!raw) return "open";
  return ACTIONS_FILTER_KEYS.includes(raw as ActionsFilterKey) ? (raw as ActionsFilterKey) : "open";
}

export function getFilterRequestParams(filter: ActionsFilterKey): ListActionsParams[] {
  const migrated = migrateLegacyFilter(filter);
  if (!migrated) return getListRequestParams("active", "all");
  return getListRequestParams(migrated.status, migrated.category);
}

export function counterForFilter(
  filter: ActionsFilterKey,
  counters: {
    total_open: number;
    stock: number;
    slow_turnover: number;
    sku: number;
    sales: number;
    advertising: number;
    funnel: number;
    category_count: number;
    checked: number;
    dismissed: number;
  },
): number {
  if (filter === "checked") return counters.checked;
  if (filter === "dismissed") return counters.dismissed;
  const category = filter === "open" ? "all" : (filter as ActionCategoryFilter);
  return counterForCategory(category, counters);
}
