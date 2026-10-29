"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Inbox, Info } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { EmptyState } from "@/components/ui/empty-state";
import { SkuFocusTrendChart } from "@/components/dashboard/sku-focus-trend-chart";
import { ProductImage } from "@/components/product-image";
import { asArray } from "@/lib/as-array";
import type { DashboardHomeResponse, KeySkuCard, SKUFocusCard, SKUFocusGroupKey } from "@/lib/dashboard-home-types";
import {
  resolveSKUFocusFilterFromHash,
  resolveSKUFocusShowAllFromHash,
  resolveSKUFocusTabFromHash,
  scrollToSKUFocusSection,
  SKU_FOCUS_NAVIGATE_EVENT,
  SKU_FOCUS_SECTION_ID,
} from "@/lib/sku-focus-hash";
import {
  buildKeySkuKpiItems,
  buildSkuFocusCheckText,
  buildSkuFocusKpiItems,
  buildSkuFocusSummaryText,
  filterStockRiskItems,
  formatKeySkuLabel,
  formatSkuLabel,
  getKeySkusTotalCount,
  getKeySkusViewAllHref,
  getKeySkuRevenueSharePct,
  getNoSalesWithStockTotalCount,
  getNoSalesWithStockViewAllHref,
  getStockRiskFilterEmptyText,
  getStockRiskFilterLabel,
  getSkuFocusEmptyStateTitle,
  getSkuFocusScenarioBadge,
  getSkuFocusTabLabel,
  getSkuFocusTrendLabel,
  getSkuFocusTrendVariant,
  getVisibleSkuFocusKpiKeys,
  hasKeySkuTrendData,
  hasSkuFocusTrendData,
  getSkuFocusTrendEmptyMessage,
  normalizeKeySkuImageUrl,
  normalizeKeySkuTrendPoints,
  resolveKeySkuCta,
  resolveSkuFocusCta,
  normalizeSkuFocusImageUrl,
  normalizeSkuFocusTrendPoints,
  shouldShowSkuFocusCheckSection,
  shouldShowSkuFocusTrend,
  resolveVisibleSkuFocusTab,
  sortOrderDropItems,
  sortStockRiskItems,
  STOCK_RISK_FILTER_ORDER,
  type StockRiskFilterKey,
  VISIBLE_SKU_FOCUS_TAB_KEYS,
} from "@/lib/sku-focus-display";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  data: DashboardHomeResponse["sku_focus"];
  initialTab?: SKUFocusGroupKey;
};

export function SKUFocusSection({ data, initialTab = "order_drop" }: Props) {
  const [tab, setTab] = useState<SKUFocusGroupKey>(() => {
    if (typeof window !== "undefined") {
      const fromHash = resolveSKUFocusTabFromHash(window.location.hash);
      return resolveVisibleSkuFocusTab(fromHash ?? initialTab);
    }
    return resolveVisibleSkuFocusTab(initialTab);
  });

  const [stockFilter, setStockFilter] = useState<StockRiskFilterKey>(() => {
    if (typeof window !== "undefined") {
      return resolveSKUFocusFilterFromHash(window.location.hash) ?? "all-risk";
    }
    return "all-risk";
  });

  useEffect(() => {
    const applyHash = (hash: string, scroll: boolean) => {
      const next = resolveSKUFocusTabFromHash(hash);
      if (!next) return;
      setTab(resolveVisibleSkuFocusTab(next));
      if (next === "stock_risk") {
        const filter = resolveSKUFocusFilterFromHash(hash);
        if (filter) {
          setStockFilter(filter);
        } else if (resolveSKUFocusShowAllFromHash(hash)) {
          setStockFilter("all-risk");
        }
      }
      if (scroll) {
        requestAnimationFrame(() => scrollToSKUFocusSection());
      }
    };

    const onHashChange = () => applyHash(window.location.hash, true);

    const onNavigate = (event: Event) => {
      const hash = (event as CustomEvent<{ hash: string }>).detail?.hash ?? window.location.hash;
      applyHash(hash, true);
    };

    if (resolveSKUFocusTabFromHash(window.location.hash)) {
      requestAnimationFrame(() => scrollToSKUFocusSection());
    }

    window.addEventListener("hashchange", onHashChange);
    window.addEventListener(SKU_FOCUS_NAVIGATE_EVENT, onNavigate);
    return () => {
      window.removeEventListener("hashchange", onHashChange);
      window.removeEventListener(SKU_FOCUS_NAVIGATE_EVENT, onNavigate);
    };
  }, []);

  const focus = data ?? {
    order_drop: [],
    stock_risk: [],
    no_sales_with_stock: [],
    no_sales_with_stock_total_count: 0,
    surplus: [],
    key_skus: [],
    key_skus_total_count: 0,
  };

  const keySkuItems = useMemo(() => asArray(focus.key_skus), [focus.key_skus]);

  const items = useMemo(() => {
    if (tab === "key_skus") {
      return [];
    }
    const rawItems = asArray(focus[tab]);
    if (tab === "order_drop") {
      return sortOrderDropItems(rawItems);
    }
    if (tab === "stock_risk") {
      return sortStockRiskItems(rawItems, stockFilter);
    }
    return rawItems;
  }, [focus, tab, stockFilter]);

  const filteredStockItems = useMemo(() => {
    if (tab !== "stock_risk") return items;
    return filterStockRiskItems(items, stockFilter);
  }, [items, tab, stockFilter]);

  // Banner count must equal rendered cards. Stock filters show the full filtered
  // universe (aligned with Stock Coverage counters); other tabs use the list
  // actually returned/rendered by the API.
  const visibleItems = useMemo(() => {
    if (tab !== "stock_risk") return items;
    return filteredStockItems;
  }, [items, tab, filteredStockItems]);

  const renderedCount =
    tab === "key_skus" ? keySkuItems.length : visibleItems.length;

  const summaryCount = renderedCount;
  const showNoSalesViewAll =
    tab === "no_sales_with_stock" &&
    getNoSalesWithStockTotalCount(focus, items.length) > renderedCount &&
    Boolean(getNoSalesWithStockViewAllHref());
  const showKeySkusViewAll =
    tab === "key_skus" &&
    getKeySkusTotalCount(focus, keySkuItems.length) > renderedCount &&
    Boolean(getKeySkusViewAllHref());

  const visibleKpiKeys = getVisibleSkuFocusKpiKeys(visibleItems, tab);
  const hasVisibleCards = tab === "key_skus" ? keySkuItems.length > 0 : items.length > 0;
  const hasFilteredCards = renderedCount > 0;

  return (
    <section
      className="scroll-mt-20 rounded-[22px] border border-[#E2E8F0] bg-white p-5 shadow-[0_8px_24px_rgba(15,23,42,0.04)] sm:p-6"
      id={SKU_FOCUS_SECTION_ID}
      data-testid="sku-focus-section"
    >
      <header className="mb-4">
        <h2 className={dashboardTypography.moduleTitle}>SKU-фокус</h2>
        <p className={cn("mt-1", dashboardTypography.moduleDescription)}>
          Только SKU, которые требуют внимания сегодня.
        </p>
      </header>

      <SkuFocusTabs activeTab={tab} onTabChange={setTab} />

      {tab === "stock_risk" && hasVisibleCards ? (
        <StockRiskFilterChips activeFilter={stockFilter} onFilterChange={setStockFilter} />
      ) : null}

      {hasVisibleCards ? (
        <SkuFocusSummaryBanner
          tab={tab}
          count={summaryCount}
          items={tab === "stock_risk" ? filteredStockItems : items}
          stockFilter={stockFilter}
          keyRevenueSharePct={getKeySkuRevenueSharePct(focus)}
        />
      ) : null}

      {!hasVisibleCards ? (
        <EmptyState
          icon={<Inbox className="h-5 w-5" aria-hidden="true" />}
          title={getSkuFocusEmptyStateTitle(tab)}
        />
      ) : !hasFilteredCards ? (
        <p
          className={cn("py-6 text-center", dashboardTypography.secondary)}
          data-testid="sku-focus-filter-empty"
        >
          {getStockRiskFilterEmptyText()}
        </p>
      ) : (
        <>
          <div
            className={cn(
              "grid gap-4",
              skuFocusGridClass(renderedCount),
            )}
            data-testid="sku-focus-grid"
            data-stock-filter={tab === "stock_risk" ? stockFilter : undefined}
            data-rendered-count={renderedCount}
          >
            {tab === "key_skus"
              ? keySkuItems.map((item) => (
                  <KeySkuFocusCardView key={`${item.ozon_product_id}-${item.sku ?? ""}`} item={item} />
                ))
              : visibleItems.map((item) => (
                  <SkuFocusCardView
                    key={`${item.ozon_product_id}-${item.sku ?? ""}`}
                    item={item}
                    tab={tab}
                    visibleKpiKeys={visibleKpiKeys}
                  />
                ))}
          </div>
          {showNoSalesViewAll ? (
            <div className="mt-4 flex justify-center">
              <Link
                href={getNoSalesWithStockViewAllHref()!}
                className="inline-flex h-9 items-center rounded-full border border-[#BFDBFE] bg-[#EFF6FF] px-4 text-[13px] font-semibold text-[#2563EB] transition-colors hover:bg-[#DBEAFE]"
                data-testid="sku-focus-no-sales-view-all"
              >
                Посмотреть все SKU
              </Link>
            </div>
          ) : null}
          {showKeySkusViewAll ? (
            <div className="mt-4 flex justify-center">
              <Link
                href={getKeySkusViewAllHref()!}
                className="inline-flex h-9 items-center rounded-full border border-[#BFDBFE] bg-[#EFF6FF] px-4 text-[13px] font-semibold text-[#2563EB] transition-colors hover:bg-[#DBEAFE]"
                data-testid="sku-focus-key-skus-view-all"
              >
                Посмотреть все SKU
              </Link>
            </div>
          ) : null}
        </>
      )}
    </section>
  );
}

function SkuFocusTabs({
  activeTab,
  onTabChange,
}: {
  activeTab: SKUFocusGroupKey;
  onTabChange: (tab: SKUFocusGroupKey) => void;
}) {
  return (
    <div className="mb-4 flex flex-wrap gap-2" data-testid="sku-focus-tabs">
      {VISIBLE_SKU_FOCUS_TAB_KEYS.map((key) => {
        const isActive = activeTab === key;
        return (
          <button
            key={key}
            type="button"
            onClick={() => onTabChange(key)}
            data-testid={`sku-focus-tab-${key}`}
            className={cn(
              "inline-flex h-9 items-center rounded-full border px-3 transition-colors",
              dashboardTypography.button,
              isActive
                ? "border-transparent bg-[#2563EB] text-white"
                : "border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC]",
            )}
          >
            {getSkuFocusTabLabel(key)}
          </button>
        );
      })}
    </div>
  );
}

function StockRiskFilterChips({
  activeFilter,
  onFilterChange,
}: {
  activeFilter: StockRiskFilterKey;
  onFilterChange: (filter: StockRiskFilterKey) => void;
}) {
  return (
    <div className="mb-3 flex flex-wrap gap-1.5" data-testid="sku-focus-stock-filter-chips">
      {STOCK_RISK_FILTER_ORDER.map((key) => {
        const isActive = activeFilter === key;
        return (
          <button
            key={key}
            type="button"
            onClick={() => onFilterChange(key)}
            data-testid={`sku-focus-stock-filter-${key}`}
            className={cn(
              "inline-flex h-7 items-center rounded-full border px-2.5 text-xs font-medium transition-colors",
              isActive
                ? "border-transparent bg-slate-900 text-white"
                : "border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC]",
            )}
          >
            {getStockRiskFilterLabel(key)}
          </button>
        );
      })}
    </div>
  );
}

function SkuFocusSummaryBanner({
  tab,
  count,
  items,
  stockFilter,
  keyRevenueSharePct,
}: {
  tab: SKUFocusGroupKey;
  count: number;
  items: SKUFocusCard[];
  stockFilter?: StockRiskFilterKey;
  keyRevenueSharePct?: number | null;
}) {
  return (
    <div
      className="mb-4 flex min-h-10 items-center gap-2.5 rounded-xl border border-[#DBEAFE] bg-[#EFF6FF] px-3.5 py-2.5"
      data-testid="sku-focus-summary-banner"
    >
      <Info className="h-4 w-4 shrink-0 text-[#2563EB]" aria-hidden="true" />
      <p className={cn(dashboardTypography.body, "font-medium text-slate-700")}>
        {buildSkuFocusSummaryText(tab, count, items, {
          keyRevenueSharePct,
          stockFilter,
        })}
      </p>
    </div>
  );
}

function KeySkuFocusCardView({ item }: { item: KeySkuCard }) {
  const skuLabel = formatKeySkuLabel(item);
  const kpis = buildKeySkuKpiItems(item);
  const trendPoints = normalizeKeySkuTrendPoints(item);
  const hasTrend = hasKeySkuTrendData(item);
  const imageUrl = normalizeKeySkuImageUrl(item);
  const cta = resolveKeySkuCta(item);

  return (
    <article
      className={cn(
        "flex h-full flex-col rounded-2xl border border-[#E2E8F0] bg-white p-4 transition duration-150",
        "hover:border-[#CBD5E1] hover:shadow-[0_6px_18px_rgba(15,23,42,0.06)]",
      )}
      data-testid="sku-focus-card"
    >
      <header className="flex items-start gap-3" data-testid="sku-focus-card-header">
        <ProductImage
          imageUrl={imageUrl}
          sku={item.sku}
          offerId={item.offer_id}
          ozonProductId={item.ozon_product_id}
          productName={item.product_name}
          alt={item.product_name}
          imgClassName="h-full w-full object-cover"
          placeholder="thumb"
          variant="product-card"
          useDemoFallback={true}
        />
        <div className="min-w-0 flex-1">
          <p className={cn("whitespace-normal break-words", dashboardTypography.listItemTitle)}>
            {item.product_name}
          </p>
          {skuLabel ? (
            <p className={cn("mt-0.5", dashboardTypography.metadata)}>{skuLabel}</p>
          ) : null}
        </div>
      </header>

      {kpis.length > 0 ? <SkuFocusMetricGrid kpis={kpis} isOrderDrop={false} /> : null}

      <SkuFocusTrend
        points={trendPoints}
        hasTrend={hasTrend}
        variant="revenue"
        tab="key_skus"
        showTrend={shouldShowSkuFocusTrend("key_skus")}
        emptyMessage="Нет данных для тренда"
      />

      {cta ? (
        <div className="mt-auto pt-1">
          <SkuFocusCardFooter cta={cta} showTopSpacing={false} />
        </div>
      ) : null}
    </article>
  );
}

function SkuFocusCardView({
  item,
  tab,
  visibleKpiKeys,
}: {
  item: SKUFocusCard;
  tab: SKUFocusGroupKey;
  visibleKpiKeys: ReturnType<typeof getVisibleSkuFocusKpiKeys>;
}) {
  const skuLabel = formatSkuLabel(item);
  const kpis = buildSkuFocusKpiItems(item, tab, visibleKpiKeys);
  const checkText = buildSkuFocusCheckText(item, tab);
  const trendVariant = getSkuFocusTrendVariant(tab);
  const trendPoints = normalizeSkuFocusTrendPoints(item, tab);
  const hasTrend = hasSkuFocusTrendData(item, tab);
  const isOrderDrop = tab === "order_drop";
  const imageUrl = normalizeSkuFocusImageUrl(item);
  const badge = getSkuFocusScenarioBadge(tab);
  const cta = resolveSkuFocusCta(item);
  const showCheckSection = shouldShowSkuFocusCheckSection(tab);
  const showTrend = shouldShowSkuFocusTrend(tab);
  const showFooter = (!isOrderDrop && (showCheckSection || cta != null)) || (isOrderDrop && cta != null);

  return (
    <article
      className={cn(
        "flex h-full flex-col rounded-2xl border border-[#E2E8F0] bg-white p-4 transition duration-150",
        "hover:border-[#CBD5E1] hover:shadow-[0_6px_18px_rgba(15,23,42,0.06)]",
      )}
      data-testid="sku-focus-card"
    >
      <SkuFocusCardHeader item={item} badge={badge} skuLabel={skuLabel} imageUrl={imageUrl} />

      {kpis.length > 0 ? (
        <SkuFocusMetricGrid kpis={kpis} isOrderDrop={isOrderDrop} />
      ) : isOrderDrop ? (
        <p className="mt-3 text-sm text-slate-500" data-testid="sku-focus-metrics-empty">
          Нет подтверждённых метрик просадки
        </p>
      ) : null}

      <SkuFocusTrend
        points={trendPoints}
        hasTrend={hasTrend}
        variant={trendVariant}
        tab={tab}
        showTrend={showTrend}
        emptyMessage={getSkuFocusTrendEmptyMessage(item)}
      />

      {showFooter ? (
        <div
          className={cn(
            "mt-auto",
            isOrderDrop ? "border-t border-[#E2E8F0] pt-3.5" : showCheckSection ? "border-t border-[#F1F5F9] pt-3" : cta ? "pt-1" : "",
          )}
        >
          {showCheckSection ? <SkuFocusCheckSection text={checkText} /> : null}
          {cta ? (
            <SkuFocusCardFooter
              cta={cta}
              showTopSpacing={showCheckSection}
              emphasized={isOrderDrop}
            />
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

function SkuFocusCardHeader({
  item,
  badge,
  skuLabel,
  imageUrl,
}: {
  item: SKUFocusCard;
  badge: { label: string; className: string } | null;
  skuLabel: string;
  imageUrl: string | null;
}) {
  return (
    <header className="flex items-start gap-3" data-testid="sku-focus-card-header">
      <ProductImage
        imageUrl={imageUrl}
        sku={item.sku}
        offerId={item.offer_id}
        ozonProductId={item.ozon_product_id}
        productName={item.product_name}
        alt={item.product_name}
        imgClassName="h-full w-full object-cover"
        placeholder="thumb"
        variant="product-card"
          useDemoFallback={true}
      />
      <div className="min-w-0 flex-1">
        <p className={cn("whitespace-normal break-words", dashboardTypography.listItemTitle)}>
          {item.product_name}
        </p>
        {skuLabel ? (
          <p className={cn("mt-0.5", dashboardTypography.metadata)}>{skuLabel}</p>
        ) : null}
        {badge ? <SkuFocusScenarioBadge badge={badge} /> : null}
      </div>
    </header>
  );
}

function SkuFocusScenarioBadge({ badge }: { badge: { label: string; className: string } }) {
  return (
    <span
      className={cn(
        "mt-1.5 inline-flex max-w-full items-center rounded-md border px-2 py-0.5",
        dashboardTypography.badge,
        badge.className,
      )}
      data-testid="sku-focus-scenario-badge"
    >
      {badge.label}
    </span>
  );
}

function SkuFocusMetricGrid({
  kpis,
  isOrderDrop,
}: {
  kpis: ReturnType<typeof buildSkuFocusKpiItems>;
  isOrderDrop: boolean;
}) {
  const columnClass =
    kpis.length >= 3 ? "grid-cols-3" : kpis.length === 2 ? "grid-cols-2" : "grid-cols-1";

  return (
    <div
      className={cn(
        "mt-3 grid gap-3",
        columnClass,
        isOrderDrop && kpis.length >= 2 ? "sm:grid-cols-3" : "",
        isOrderDrop ? "min-h-[72px]" : "",
      )}
      data-testid="sku-focus-kpi-row"
      data-layout="grid"
    >
      {kpis.map((kpi) => (
        <div key={kpi.key} className="min-w-0" data-testid={`sku-focus-kpi-${kpi.key}`}>
          <p className={dashboardTypography.kpiLabelPlain}>{kpi.label}</p>
          {kpi.sublabel ? (
            <p className={cn("mt-0.5", dashboardTypography.metadata)}>{kpi.sublabel}</p>
          ) : null}
          <p
            className={cn(
              "mt-1",
              dashboardTypography.bodyEmphasis,
              "font-bold",
              kpi.tone === "negative"
                ? "text-[#DC2626]"
                : kpi.tone === "positive"
                  ? "text-[#16A34A]"
                  : kpi.tone === "warning"
                    ? "text-[#EA580C]"
                    : kpi.tone === "attention"
                      ? "text-[#CA8A04]"
                      : kpi.tone === "muted"
                        ? "text-[#94A3B8]"
                        : "text-[#1E293B]",
            )}
          >
            {kpi.value}
          </p>
        </div>
      ))}
    </div>
  );
}

function SkuFocusTrend({
  points,
  hasTrend,
  variant,
  tab,
  showTrend,
  emptyMessage,
}: {
  points: ReturnType<typeof normalizeSkuFocusTrendPoints>;
  hasTrend: boolean;
  variant: ReturnType<typeof getSkuFocusTrendVariant>;
  tab: SKUFocusGroupKey;
  showTrend: boolean;
  emptyMessage: string;
}) {
  if (!showTrend) {
    return null;
  }
  const stroke = variant === "orders" ? "#EA580C" : variant === "revenue" ? "#2563EB" : "#2563EB";
  const fill =
    variant === "orders"
      ? "rgba(251, 146, 60, 0.12)"
      : variant === "revenue"
        ? "rgba(37, 99, 235, 0.1)"
        : "rgba(37, 99, 235, 0.1)";

  return (
    <div className="mt-3" data-testid="sku-focus-trend">
      <p className={cn("mb-1.5", dashboardTypography.kpiLabelPlain)}>{getSkuFocusTrendLabel(tab)}</p>
      <div
        className="flex h-[80px] w-full items-center rounded-lg border border-[#F1F5F9] bg-[#FAFAFA] px-1"
        data-testid="sku-focus-trend-chart-wrap"
      >
        {hasTrend ? (
          <SkuFocusTrendChart
            points={points}
            variant={variant}
            width={320}
            height={72}
            stroke={stroke}
            fill={fill}
            className="h-[72px] w-full max-w-full"
          />
        ) : (
          <p className={cn("w-full px-2 text-center", dashboardTypography.secondary)} data-testid="sku-focus-trend-empty">
            {emptyMessage}
          </p>
        )}
      </div>
    </div>
  );
}

function SkuFocusCheckSection({ text }: { text: string }) {
  return (
    <div
      className="rounded-lg border border-[#F1F5F9] bg-[#F8FAFC] px-2.5 py-2"
      data-testid="sku-focus-check-section"
    >
      <p className={dashboardTypography.sectionTitle}>Что проверить</p>
      <p className={cn("mt-1 whitespace-normal break-words", dashboardTypography.body)}>{text}</p>
    </div>
  );
}

function SkuFocusCardFooter({
  cta,
  showTopSpacing,
  emphasized = false,
}: {
  cta: { href: string; label: string; kind: "direct" | "search" };
  showTopSpacing: boolean;
  emphasized?: boolean;
}) {
  return (
    <Link
      href={cta.href}
      className={cn(
        "inline-flex items-center gap-1",
        dashboardTypography.ctaLink,
        showTopSpacing ? "mt-2.5" : emphasized ? "mt-0.5" : "",
      )}
      data-testid="sku-focus-card-cta"
      data-cta-kind={cta.kind}
    >
      {cta.label}
      <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
    </Link>
  );
}

function skuFocusGridClass(count: number): string {
  if (count <= 1) return "grid-cols-1";
  if (count === 2) return "grid-cols-1 md:grid-cols-2";
  if (count === 4) return "grid-cols-1 md:grid-cols-2 xl:grid-cols-4";
  if (count === 6) return "grid-cols-1 md:grid-cols-2 xl:grid-cols-3";
  if (count === 8 || count === 9) return "grid-cols-1 md:grid-cols-2 xl:grid-cols-3";
  return "grid-cols-1 md:grid-cols-2 xl:grid-cols-3";
}
