"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ProductSparkline } from "@/components/product-sparkline";
import { buttonClassNames } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  getProducts,
  type ProductCatalogItem,
  type ProductsListResponse,
} from "@/lib/products-api";
import { ProductImage } from "@/components/product-image";
import { ui } from "@/lib/ui-copy";

const PAGE_SIZE = 50;

const SORT_OPTIONS = [
  { value: "revenue_last_day", label: "Выручка за день" },
  { value: "orders_last_day", label: "Заказы за день" },
  { value: "current_price_desc", label: "Цена ↓" },
  { value: "current_price_asc", label: "Цена ↑" },
  { value: "stock_available_asc", label: "Остаток ↑" },
  { value: "product_name_asc", label: "Название А–Я" },
] as const;

function fmtMoney(value: number | null | undefined): string {
  if (value == null) return "—";
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(value);
}

function fmtNumber(value: number | null | undefined): string {
  if (value == null) return "—";
  return new Intl.NumberFormat("ru-RU").format(value);
}

function discountPercent(current: number, old: number): number | null {
  if (old <= current || old <= 0) return null;
  return Math.round(((old - current) / old) * 100);
}

function stockBucketLabel(bucket: string | null | undefined): string {
  switch (bucket) {
    case "out_of_stock":
      return "Нет в наличии";
    case "critical_shortage":
      return "Критический дефицит";
    case "shortage_7d":
      return "Дефицит ~7 дн.";
    case "planning_14d":
      return "Планирование ~14 дн.";
    case "healthy":
      return "Норма";
    case "surplus_120d":
      return "Избыток";
    case "no_sales":
      return "Нет продаж";
    default:
      return bucket ?? "—";
  }
}

function lastDayLabel(item: ProductCatalogItem, latestMetricDate?: string | null): string {
  if (!item.last_day?.date) {
    return "Метрики продаж пока загружаются.";
  }
  const d = item.last_day.date;
  if (latestMetricDate) {
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    const yesterdayKey = yesterday.toISOString().slice(0, 10);
    if (d === yesterdayKey && d === latestMetricDate) {
      return "Вчера";
    }
  }
  return `За ${d}`;
}

function ProductCard({
  item,
  latestMetricDate,
}: {
  item: ProductCatalogItem;
  latestMetricDate?: string | null;
}) {
  const hasDiscount =
    item.old_price != null &&
    item.current_price != null &&
    item.old_price > item.current_price;
  const discount = hasDiscount
    ? discountPercent(item.current_price!, item.old_price!)
    : null;

  return (
    <article className="flex flex-col overflow-hidden rounded-aivion-lg border border-aivion-border bg-aivion-surface shadow-aivion-sm transition-shadow hover:shadow-aivion-md">
      <div className="relative aspect-[4/3] w-full bg-aivion-surface-soft">
        <ProductImage
          imageUrl={item.image_url}
          sku={item.sku}
          offerId={item.offer_id}
          productName={item.product_name}
          alt={item.product_name}
          imgClassName="h-full w-full object-contain p-2"
          placeholder="card"
        />
      </div>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <h3 className="line-clamp-2 text-sm font-semibold text-aivion-text">{item.product_name}</h3>

        <div className="text-xs text-aivion-muted">
          {item.sku != null ? <div>SKU: {item.sku}</div> : null}
          {item.offer_id ? <div className="truncate">Артикул: {item.offer_id}</div> : null}
        </div>

        <div className="space-y-1">
          <div className="flex flex-wrap items-baseline gap-2">
            <span className="text-lg font-semibold text-aivion-text">
              {fmtMoney(item.current_price)}
            </span>
            {hasDiscount ? (
              <>
                <span className="text-sm text-aivion-muted-soft line-through">
                  {fmtMoney(item.old_price)}
                </span>
                {discount != null ? (
                  <span className="rounded-aivion-sm bg-[#fef3f2] px-1.5 py-0.5 text-xs font-medium text-aivion-danger">
                    −{discount}%
                  </span>
                ) : null}
              </>
            ) : null}
          </div>
          {item.min_price != null ? (
            <p className="text-xs text-aivion-muted-soft">Мин. цена Ozon: {fmtMoney(item.min_price)}</p>
          ) : null}
        </div>

        <div className="rounded-aivion-md bg-aivion-surface-soft p-2.5 text-xs text-aivion-text">
          {item.last_day ? (
            <>
              <p className="font-medium text-aivion-text">{lastDayLabel(item, latestMetricDate)}</p>
              <div className="mt-1 grid grid-cols-3 gap-2">
                <div>
                  <span className="text-aivion-muted">Заказы</span>
                  <div className="font-medium">{fmtNumber(item.last_day.orders_count)}</div>
                </div>
                <div>
                  <span className="text-aivion-muted-soft">Выручка</span>
                  <div className="font-medium">{fmtMoney(item.last_day.revenue)}</div>
                </div>
                <div>
                  <span className="text-aivion-muted-soft">Штук</span>
                  <div className="font-medium">{fmtNumber(item.last_day.units_sold)}</div>
                </div>
              </div>
            </>
          ) : (
            <p>Метрики продаж пока загружаются.</p>
          )}
        </div>

        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-aivion-muted-soft">Заказы, 7 дн.</span>
          <ProductSparkline points={item.sparkline_7d ?? []} />
        </div>

        {item.stock ? (
          <div className="text-xs text-aivion-muted">
            <span className="font-medium text-aivion-text">
              Остаток: {fmtNumber(item.stock.stock_available)}
            </span>
            {item.stock.days_of_cover != null ? (
              <span className="ml-2">Покрытие: {item.stock.days_of_cover.toFixed(1)} дн.</span>
            ) : null}
            {item.stock.stock_status_bucket ? (
              <span className="mt-1 block text-aivion-muted-soft">
                {stockBucketLabel(item.stock.stock_status_bucket)}
              </span>
            ) : null}
          </div>
        ) : null}

        {item.ozon_url ? (
          <a
            href={item.ozon_url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-auto text-sm font-medium text-blue-700 hover:text-blue-900"
          >
            Открыть на Ozon →
          </a>
        ) : null}
      </div>
    </article>
  );
}

export default function ProductsScreen() {
  const searchParams = useSearchParams();
  const [response, setResponse] = useState<ProductsListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<string>("revenue_last_day");
  const [salesFilter, setSalesFilter] = useState<"" | "had_sales_last_day" | "no_sales_last_day">("");
  const [keySkusFilter, setKeySkusFilter] = useState(false);
  const [offset, setOffset] = useState(0);

  useEffect(() => {
    const filter = searchParams.get("filter");
    if (filter === "no-sales" || filter === "no_sales") {
      setSalesFilter("no_sales_last_day");
      setKeySkusFilter(false);
      setOffset(0);
    } else if (filter === "key-skus" || filter === "key_skus") {
      setSalesFilter("");
      setKeySkusFilter(true);
      setSort("revenue_last_day");
      setOffset(0);
    } else {
      setKeySkusFilter(false);
    }
  }, [searchParams]);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const hasSales =
        salesFilter === "had_sales_last_day"
          ? true
          : salesFilter === "no_sales_last_day"
            ? false
            : undefined;
      const data = await getProducts({
        search: search || undefined,
        limit: PAGE_SIZE,
        offset,
        sort,
        has_sales_last_day: hasSales,
        filter: keySkusFilter ? "key-skus" : undefined,
      });
      setResponse(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : ui.requestFailed);
      setResponse(null);
    } finally {
      setLoading(false);
    }
  }, [offset, salesFilter, search, sort, keySkusFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  const total = response?.total ?? 0;
  const canPrev = offset > 0;
  const canNext = response != null && offset + PAGE_SIZE < total;

  const pageLabel = useMemo(() => {
    if (total === 0) return "0 товаров";
    const from = offset + 1;
    const to = Math.min(offset + PAGE_SIZE, total);
    return `${from}–${to} из ${total}`;
  }, [offset, total]);

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-4 md:p-6">
      <PageHeader
        title="Товары"
        subtitle="Витрина SKU: цены, продажи за последний день с данными и динамика заказов за 7 дней. Управление ценами — в кабинете Ozon."
      />

      <div className="flex flex-col gap-3 rounded-lg border border-aivion-border bg-aivion-surface p-4 shadow-sm md:flex-row md:flex-wrap md:items-end">
        <label className="flex min-w-[200px] flex-1 flex-col gap-1 text-sm">
          <span className="text-aivion-muted">{ui.search}</span>
          <input
            type="search"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setOffset(0);
                setSearch(searchInput.trim());
              }
            }}
            placeholder="Название, SKU или артикул"
            className="rounded-md border border-aivion-border px-3 py-2"
          />
        </label>
        <button
          type="button"
          className={buttonClassNames("primary")}
          onClick={() => {
            setOffset(0);
            setSearch(searchInput.trim());
          }}
        >
          Найти
        </button>
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-aivion-muted">Сортировка</span>
          <select
            value={sort}
            onChange={(e) => {
              setOffset(0);
              setSort(e.target.value);
            }}
            className="rounded-md border border-aivion-border px-3 py-2"
          >
            {SORT_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-aivion-muted">Продажи</span>
          <select
            value={salesFilter}
            onChange={(e) => {
              setOffset(0);
              setSalesFilter(e.target.value as typeof salesFilter);
            }}
            className="rounded-md border border-aivion-border px-3 py-2"
          >
            <option value="">Все</option>
            <option value="had_sales_last_day">Были продажи</option>
            <option value="no_sales_last_day">Без продаж</option>
          </select>
        </label>
        <button
          type="button"
          className={buttonClassNames("secondary")}
          disabled={loading}
          onClick={() => void load()}
        >
          {loading ? ui.refreshing : ui.refresh}
        </button>
      </div>

      {loading && !response ? (
        <LoadingState message="Загрузка товаров…" />
      ) : error ? (
        <ErrorState
          title={ui.error}
          message={error}
          action={
            <button type="button" className={buttonClassNames("secondary")} onClick={() => void load()}>
              {ui.refresh}
            </button>
          }
        />
      ) : !response?.items.length ? (
        <EmptyState
          title="Товары пока не найдены"
          message="Данные появятся после загрузки товаров из кабинета Ozon."
        />
      ) : (
        <>
          <p className="text-sm text-aivion-muted">{pageLabel}</p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {response.items.map((item) => (
              <ProductCard
                key={`${item.sku ?? item.offer_id ?? item.product_name}`}
                item={item}
                latestMetricDate={response.latest_metric_date}
              />
            ))}
          </div>
          <div className="flex items-center justify-between gap-2">
            <button
              type="button"
              className={buttonClassNames("secondary")}
              disabled={!canPrev || loading}
              onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
            >
              {ui.prev}
            </button>
            <span className="text-sm text-aivion-muted">{pageLabel}</span>
            <button
              type="button"
              className={buttonClassNames("secondary")}
              disabled={!canNext || loading}
              onClick={() => setOffset((o) => o + PAGE_SIZE)}
            >
              {ui.next}
            </button>
          </div>
        </>
      )}
    </div>
  );
}
