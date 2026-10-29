/**
 * Centralized Dashboard section routes and helpers.
 * Keep deep links here — do not concatenate paths in Overview components.
 */

export const DASHBOARD_BASE = "/app/dashboard";

export const DASHBOARD_ROUTES = {
  overview: DASHBOARD_BASE,
  funnel: `${DASHBOARD_BASE}/funnel`,
  stock: `${DASHBOARD_BASE}/stock`,
  advertising: `${DASHBOARD_BASE}/advertising`,
  categories: `${DASHBOARD_BASE}/categories`,
  sku: `${DASHBOARD_BASE}/sku`,
} as const;

export type DashboardSectionId =
  | "overview"
  | "funnel"
  | "stock"
  | "advertising";

export type DashboardTabDef = {
  id: DashboardSectionId;
  label: string;
  href: string;
};

export const DASHBOARD_TABS: DashboardTabDef[] = [
  { id: "overview", label: "Обзор", href: DASHBOARD_ROUTES.overview },
  { id: "funnel", label: "Воронка", href: DASHBOARD_ROUTES.funnel },
  { id: "stock", label: "Остатки", href: DASHBOARD_ROUTES.stock },
  { id: "advertising", label: "Реклама", href: DASHBOARD_ROUTES.advertising },
];

export function normalizeDashboardPath(pathname: string | null | undefined): string {
  if (!pathname) return DASHBOARD_BASE;
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return pathname.slice(0, -1);
  }
  return pathname;
}

export function isDashboardPath(pathname: string | null | undefined): boolean {
  const p = normalizeDashboardPath(pathname);
  return p === DASHBOARD_BASE || p.startsWith(`${DASHBOARD_BASE}/`);
}

export function getDashboardSectionId(pathname: string | null | undefined): DashboardSectionId {
  const p = normalizeDashboardPath(pathname);
  if (p === DASHBOARD_ROUTES.funnel) return "funnel";
  if (p === DASHBOARD_ROUTES.stock) return "stock";
  if (p === DASHBOARD_ROUTES.advertising) return "advertising";
  return "overview";
}

export function getDashboardTabHref(id: DashboardSectionId): string {
  return DASHBOARD_TABS.find((tab) => tab.id === id)?.href ?? DASHBOARD_ROUTES.overview;
}

export function actionsListHref(): string {
  return "/app/actions";
}

export function actionDetailHref(actionKeyOrId: string | number): string {
  const params = new URLSearchParams({ action: String(actionKeyOrId) });
  return `/app/actions?${params.toString()}`;
}
