"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { buttonClassNames } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { MetricCard } from "@/components/ui/metric-card";
import { PageHeader } from "@/components/ui/page-header";
import {
  getCriticalSKUs,
  type CriticalSKUItem,
  type CriticalSKUsResponse,
} from "@/lib/analytics-api";

const FETCH_LIMIT = 200;
const HIGH_PROBLEM_SCORE = 70;

type CriticalSKUsScreenProps = {
  initialAsOfDate?: string;
};

function fmtMoney(value: number): string {
  return new Intl.NumberFormat("ru-RU", {
    style: "currency",
    currency: "RUB",
    maximumFractionDigits: 0,
  }).format(value);
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat("ru-RU").format(value);
}

function fmtDateTime(value: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function badgeLabel(badge: string): string {
  const key = badge.toLowerCase();
  if (key.includes("revenue_down")) return "падение выручки";
  if (key.includes("sales_ops_down")) return "падение операций";
  if (key.includes("out_of_stock")) return "нет в наличии";
  if (key.includes("low_stock")) return "мало на складе";
  if (key.includes("high_importance")) return "высокая важность";
  return badge.replaceAll("_", " ").toLowerCase();
}

function badgeClass(badge: string): string {
  const key = badge.toLowerCase();
  if (key.includes("out_of_stock")) return "bg-red-100 text-red-800";
  if (key.includes("low_stock")) return "bg-orange-100 text-orange-800";
  if (key.includes("revenue_down") || key.includes("sales_ops_down")) {
    return "bg-amber-100 text-amber-800";
  }
  if (key.includes("high_importance")) return "bg-purple-100 text-purple-800";
  return "bg-aivion-surface-soft text-aivion-text";
}

function rowHasHighProblemScore(row: CriticalSKUItem): boolean {
  return row.problem_score >= HIGH_PROBLEM_SCORE;
}

function rowHasLowStock(row: CriticalSKUItem): boolean {
  if (row.stock_available <= 3) return true;
  if (row.days_of_cover != null && row.days_of_cover <= 7) return true;
  const hay = [...(row.badges ?? []), ...(row.signals ?? [])].join(" ").toLowerCase();
  return hay.includes("low_stock") || hay.includes("out_of_stock");
}

function rowHasSalesDrop(row: CriticalSKUItem): boolean {
  if (row.revenue_delta_day < 0 || row.orders_delta_day < 0) return true;
  const hay = [...(row.badges ?? []), ...(row.signals ?? [])].join(" ").toLowerCase();
  return hay.includes("revenue_down") || hay.includes("sales_ops_down") || hay.includes("падение");
}

function collectBadgeAndSignalOptions(rows: CriticalSKUItem[]): string[] {
  const set = new Set<string>();
  for (const row of rows) {
    for (const b of row.badges ?? []) {
      if (b.trim()) set.add(b.trim());
    }
    for (const s of row.signals ?? []) {
      if (s.trim()) set.add(s.trim());
    }
  }
  return [...set].sort((a, b) => a.localeCompare(b));
}

function rowMatchesSearch(row: CriticalSKUItem, q: string): boolean {
  if (!q) return true;
  const n = q.toLowerCase();
  const name = (row.product_name ?? "").toLowerCase();
  const offer = (row.offer_id ?? "").toLowerCase();
  const sku = row.sku != null ? String(row.sku) : "";
  return name.includes(n) || offer.includes(n) || sku.includes(n);
}

function rowMatchesSignalFilter(row: CriticalSKUItem, tag: string): boolean {
  if (!tag) return true;
  if (row.badges?.includes(tag)) return true;
  return (row.signals ?? []).some((s) => s === tag);
}

export default function CriticalSKUsScreen({ initialAsOfDate }: CriticalSKUsScreenProps) {
  const [response, setResponse] = useState<CriticalSKUsResponse | null>(null);
  const [allRows, setAllRows] = useState<CriticalSKUItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [signalFilter, setSignalFilter] = useState("");

  useEffect(() => {
    async function bootstrap() {
      try {
        setLoading(true);
        setError("");

        const data = await getCriticalSKUs({
          asOfDate: initialAsOfDate,
          limit: FETCH_LIMIT,
          offset: 0,
          sortBy: "problem_score",
          sortOrder: "desc",
        });
        setResponse(data);
        setAllRows(data.items ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Не удалось загрузить критические SKU");
        setResponse(null);
        setAllRows([]);
      } finally {
        setLoading(false);
      }
    }

    bootstrap();
  }, [initialAsOfDate]);

  const filterOptions = useMemo(() => collectBadgeAndSignalOptions(allRows), [allRows]);

  const filteredRows = useMemo(() => {
    const q = search.trim();
    return allRows.filter((row) => rowMatchesSearch(row, q) && rowMatchesSignalFilter(row, signalFilter));
  }, [allRows, search, signalFilter]);

  const summary = useMemo(() => {
    const total = filteredRows.length;
    const highProblem = filteredRows.filter(rowHasHighProblemScore).length;
    const lowStock = filteredRows.filter(rowHasLowStock).length;
    const salesDrop = filteredRows.filter(rowHasSalesDrop).length;
    return { total, highProblem, lowStock, salesDrop };
  }, [filteredRows]);

  if (loading) {
    return (
      <main className="space-y-6 p-6">
        <PageHeader
          title="Критические SKU"
          subtitle="Оперативный рейтинг проблемных SKU по индексу проблемы."
        />
        <LoadingState message="Загрузка критических SKU…" />
      </main>
    );
  }

  if (error || !response) {
    return (
      <main className="space-y-6 p-6">
        <PageHeader
          title="Критические SKU"
          subtitle="Оперативный рейтинг проблемных SKU по индексу проблемы."
        />
        <ErrorState
          title="Не удалось загрузить «Критические SKU»"
          message={error || "Неизвестная ошибка"}
          action={
            <Link href="/app/dashboard" className={buttonClassNames("secondary")}>
              На дашборд
            </Link>
          }
        />
      </main>
    );
  }

  return (
    <main className="space-y-6 p-6">
      <PageHeader
        title="Критические SKU"
        subtitle="Оперативный рейтинг проблемных SKU по индексу проблемы. Поиск и фильтры в загруженном окне."
        className="border-0 pb-0"
      >
        <Link href="/app/dashboard" className={buttonClassNames("secondary")}>
          Дашборд
        </Link>
      </PageHeader>

      <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Строк в выборке"
          value={fmtNumber(summary.total)}
          hint="После поиска и фильтра по метке"
        />
        <MetricCard
          title="Высокий индекс проблемы"
          value={fmtNumber(summary.highProblem)}
          hint={`Индекс проблемы ≥ ${HIGH_PROBLEM_SCORE}`}
        />
        <MetricCard
          title="Признаки малого запаса"
          value={fmtNumber(summary.lowStock)}
          hint="Остаток, покрытие или метки"
        />
        <MetricCard
          title="Давление по продажам"
          value={fmtNumber(summary.salesDrop)}
          hint="Отрицательные дельты или метки"
        />
      </section>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Окно данных</CardTitle>
          <CardDescription>Мета-сведения сервера для текущего запроса.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-1 text-sm text-aivion-text">
          <p>
            <span className="font-medium text-aivion-muted">Дата отчёта:</span> {response.meta.as_of_date}
          </p>
          <p>
            <span className="font-medium text-aivion-muted">Метка времени данных:</span>{" "}
            {fmtDateTime(response.meta.latest_data_timestamp)}
          </p>
          <p>
            <span className="font-medium text-aivion-muted">Сортировка:</span> {response.meta.sort_by} ({response.meta.sort_order}
            ) · <span className="font-medium text-aivion-muted">Всего в отчёте:</span> {fmtNumber(response.meta.total)} ·{" "}
            <span className="font-medium text-aivion-muted">Загружено:</span> {fmtNumber(allRows.length)} (макс. {FETCH_LIMIT})
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Поиск и фильтры</CardTitle>
          <CardDescription>На стороне клиента для загруженных строк. Очистите поля для сброса.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 md:flex-row md:flex-wrap md:items-end">
          <label className="min-w-[200px] flex-1 text-sm">
            <span className="mb-1 block font-medium text-aivion-text">Поиск</span>
            <input
              type="search"
              className="w-full rounded-lg border border-aivion-border px-3 py-2"
              placeholder="Название товара, ID предложения или SKU"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              autoComplete="off"
            />
          </label>
          {filterOptions.length > 0 ? (
            <label className="min-w-[200px] text-sm">
              <span className="mb-1 block font-medium text-aivion-text">Метка / сигнал</span>
              <select
                className="w-full rounded-lg border border-aivion-border px-3 py-2"
                value={signalFilter}
                onChange={(e) => setSignalFilter(e.target.value)}
              >
                <option value="">Все</option>
                {filterOptions.map((opt) => (
                  <option key={opt} value={opt}>
                    {opt}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Проблемные SKU</CardTitle>
          <CardDescription>
            {filteredRows.length === allRows.length
              ? `Показаны все ${fmtNumber(filteredRows.length)} загруженных строк.`
              : `Показано ${fmtNumber(filteredRows.length)} из ${fmtNumber(allRows.length)} загруженных строк.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {filteredRows.length === 0 ? (
            <EmptyState
              title={allRows.length === 0 ? "Нет критических SKU" : "Нет подходящих строк"}
              message={
                allRows.length === 0
                  ? "Нет критических SKU за выбранный период."
                  : "Попробуйте сбросить поиск или фильтр по метке."
              }
              action={
                allRows.length > 0 ? (
                  <button
                    type="button"
                    className={buttonClassNames("secondary")}
                    onClick={() => {
                      setSearch("");
                      setSignalFilter("");
                    }}
                  >
                    Сбросить фильтры
                  </button>
                ) : (
                  <Link href="/app/dashboard" className={buttonClassNames("secondary")}>
                    На дашборд
                  </Link>
                )
              }
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b text-left">
                    <th className="px-2 py-2">SKU / товар</th>
                    <th className="px-2 py-2">Индекс проблемы</th>
                    <th className="px-2 py-2">Выручка</th>
                    <th className="px-2 py-2">Операции продаж</th>
                    <th className="px-2 py-2">Δ выручки</th>
                    <th className="px-2 py-2">Δ операций</th>
                    <th className="px-2 py-2">Остаток</th>
                    <th className="px-2 py-2">Дней покрытия</th>
                    <th className="px-2 py-2">Риск</th>
                    <th className="px-2 py-2">Важность</th>
                    <th className="px-2 py-2">Метки / сигналы</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredRows.map((row) => (
                    <tr key={`${row.ozon_product_id}-${row.offer_id ?? ""}-${row.sku ?? ""}`} className="border-b align-top">
                      <td className="px-2 py-2">
                        <div className="font-medium">{row.product_name || "—"}</div>
                        <div className="text-xs text-aivion-muted-soft">
                          product_id={row.ozon_product_id} | предложение={row.offer_id || "—"} | SKU=
                          {row.sku ?? "—"}
                        </div>
                      </td>
                      <td className="px-2 py-2 font-semibold">{row.problem_score.toFixed(2)}</td>
                      <td className="px-2 py-2">{fmtMoney(row.revenue)}</td>
                      <td className="px-2 py-2">{fmtNumber(row.sales_ops)}</td>
                      <td className="px-2 py-2">{fmtMoney(row.revenue_delta_day)}</td>
                      <td className="px-2 py-2">{fmtNumber(row.orders_delta_day)}</td>
                      <td className="px-2 py-2">{fmtNumber(row.stock_available)}</td>
                      <td className="px-2 py-2">
                        {row.days_of_cover == null ? "—" : row.days_of_cover.toFixed(2)}
                      </td>
                      <td className="px-2 py-2">{row.out_of_stock_risk.toFixed(2)}</td>
                      <td className="px-2 py-2">{row.importance.toFixed(2)}</td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap gap-1">
                          {(row.badges ?? []).map((badge) => (
                            <span
                              key={`${row.ozon_product_id}-${badge}`}
                              className={`rounded px-2 py-0.5 text-xs font-medium ${badgeClass(badge)}`}
                            >
                              {badgeLabel(badge)}
                            </span>
                          ))}
                        </div>
                        {(row.signals ?? []).length > 0 ? (
                          <p className="mt-1 text-xs text-aivion-muted-soft">
                            сигналы: {(row.signals ?? []).join(", ")}
                          </p>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </main>
  );
}
