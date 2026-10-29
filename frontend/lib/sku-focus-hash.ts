import type { SKUFocusGroupKey } from "@/lib/dashboard-home-types";
import type { StockRiskFilterKey } from "@/lib/sku-focus-display";

export const SKU_FOCUS_SECTION_ID = "sku-focus";
export const SKU_FOCUS_NAVIGATE_EVENT = "sku-focus-navigate";

const STOCK_FILTER_VALUES = new Set<StockRiskFilterKey>([
  "all-risk",
  "out-of-stock",
  "critical",
  "short",
  "planning",
]);

/** Strips optional query suffixes, keeping only the base `#target` hash. */
function extractSKUFocusHashBase(hash: string): string {
  const raw = (hash ?? "").trim();
  if (!raw) return "";
  const withoutHash = raw.startsWith("#") ? raw.slice(1) : raw;
  const base = withoutHash.split(/[?&]/)[0]?.trim() ?? "";
  return base ? `#${base}` : "";
}

function parseHashQuery(hash: string): URLSearchParams {
  const raw = (hash ?? "").trim();
  if (!raw) return new URLSearchParams();
  const withoutHash = raw.startsWith("#") ? raw.slice(1) : raw;
  const qIndex = withoutHash.indexOf("?");
  if (qIndex >= 0) {
    return new URLSearchParams(withoutHash.slice(qIndex + 1));
  }
  // Support legacy `#target&filter=all-risk` form.
  const ampIndex = withoutHash.indexOf("&");
  if (ampIndex >= 0) {
    return new URLSearchParams(withoutHash.slice(ampIndex + 1));
  }
  return new URLSearchParams();
}

/**
 * Map dashboard anchor hashes to SKU-фокус tab keys. Accepts optional
 * `?show=all` / `&filter=all-risk` suffixes (e.g. `#sku-focus-stock?show=all`).
 */
export function resolveSKUFocusTabFromHash(hash: string): SKUFocusGroupKey | null {
  switch (extractSKUFocusHashBase(hash)) {
    case "#sku-focus-no-sales":
      return "no_sales_with_stock";
    case "#sku-focus-stock":
      return "stock_risk";
    default:
      return null;
  }
}

/**
 * True when the hash requests the full (unclamped) stock-risk list, e.g.
 * `#sku-focus-stock?show=all` or `#sku-focus-stock&filter=all-risk`.
 */
export function resolveSKUFocusShowAllFromHash(hash: string): boolean {
  const params = parseHashQuery(hash);
  if (params.get("show") === "all") return true;
  const filter = params.get("filter");
  return filter === "all-risk";
}

/** Reads stock filter from hash (`filter=all-risk|out-of-stock|critical|short|planning`). */
export function resolveSKUFocusFilterFromHash(hash: string): StockRiskFilterKey | null {
  const filter = parseHashQuery(hash).get("filter");
  if (!filter) return null;
  return STOCK_FILTER_VALUES.has(filter as StockRiskFilterKey)
    ? (filter as StockRiskFilterKey)
    : null;
}

export function targetToSKUFocusHash(target: string): string | null {
  const normalized = target.startsWith("#") ? target : `#${target}`;
  const base = extractSKUFocusHashBase(normalized);
  return resolveSKUFocusTabFromHash(base) ? base : null;
}

export function scrollToSKUFocusSection(): void {
  if (typeof document === "undefined") return;
  const el = document.getElementById(SKU_FOCUS_SECTION_ID);
  if (!el) return;
  requestAnimationFrame(() => {
    el.scrollIntoView({ behavior: "smooth", block: "start" });
  });
}

export type NavigateToSKUFocusOptions = {
  /** Appends `?show=all` so the SKU-фокус section shows the full stock-risk list, not just the top-N. */
  showAll?: boolean;
  /** Stock filter to activate (defaults to all-risk when showAll). */
  filter?: StockRiskFilterKey;
};

/** Update SKU-фокус hash and scroll; repeat clicks with the same hash still scroll. */
export function navigateToSKUFocusTarget(target: string, options: NavigateToSKUFocusOptions = {}): void {
  if (typeof window === "undefined") return;
  const baseHash = targetToSKUFocusHash(target);
  if (!baseHash) return;

  const params = new URLSearchParams();
  if (options.showAll) params.set("show", "all");
  const filter = options.filter ?? (options.showAll ? "all-risk" : undefined);
  if (filter) params.set("filter", filter);
  const query = params.toString();
  const hash = query ? `${baseHash}?${query}` : baseHash;

  if (window.location.hash !== hash) {
    window.location.hash = hash.slice(1);
    return;
  }

  window.dispatchEvent(new CustomEvent(SKU_FOCUS_NAVIGATE_EVENT, { detail: { hash } }));
}
