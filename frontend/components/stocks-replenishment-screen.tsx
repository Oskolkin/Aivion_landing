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
  getStocksReplenishment,
  type StocksReplenishmentItem,
  type StocksReplenishmentResponse,
} from "@/lib/analytics-api";

type StocksReplenishmentScreenProps = {
  initialAsOfDate?: string;
};

function fmtNumber(value: number): string {
  return new Intl.NumberFormat("ru-RU").format(value);
}

function fmtDateTime(value: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function riskLabel(risk: string): string {
  const normalized = risk.replaceAll("_", " ").toLowerCase();
  const map: Record<string, string> = {
    "out of stock": "нет в наличии",
    "low stock": "мало на складе",
    critical: "критический",
    high: "высокий",
    medium: "средний",
    low: "низкий",
  };
  return map[normalized] ?? risk.replaceAll("_", " ");
}

function translateReplenishmentPriority(priority: string): string {
  const p = priority.toLowerCase();
  const m: Record<string, string> = {
    critical: "Критический",
    high: "Высокий",
    medium: "Средний",
    low: "Низкий",
  };
  return m[p] ?? priority;
}

function priorityClass(priority: string): string {
  const p = priority.toLowerCase();
  if (p === "critical" || p === "high") return "bg-red-100 text-red-800";
  if (p === "medium") return "bg-amber-100 text-amber-800";
  return "bg-green-100 text-green-800";
}

function isHighReplenishmentPriority(priority: string): boolean {
  const p = priority.toLowerCase();
  return p === "high" || p === "critical";
}

function isOutOrCriticalDepletion(risk: string): boolean {
  const r = risk.toLowerCase();
  return r.includes("out_of_stock") || r.includes("out of stock") || r === "critical" || r.includes("critical");
}

function averageDaysOfCover(rows: StocksReplenishmentItem[]): number | null {
  const vals = rows.map((r) => r.days_of_cover).filter((d): d is number => d != null && Number.isFinite(d));
  if (vals.length === 0) return null;
  const sum = vals.reduce((a, b) => a + b, 0);
  return sum / vals.length;
}

function distinctSorted(values: string[]): string[] {
  return [...new Set(values.map((v) => v.trim()).filter(Boolean))].sort((a, b) => a.localeCompare(b));
}

export default function StocksReplenishmentScreen({
  initialAsOfDate,
}: StocksReplenishmentScreenProps) {
  const [response, setResponse] = useState<StocksReplenishmentResponse | null>(null);
  const [allRows, setAllRows] = useState<StocksReplenishmentItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [priorityFilter, setPriorityFilter] = useState("");
  const [riskFilter, setRiskFilter] = useState("");

  useEffect(() => {
    async function bootstrap() {
      try {
        setLoading(true);
        setError("");

        const data = await getStocksReplenishment(initialAsOfDate);
        setResponse(data);
        setAllRows(data.items ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Не удалось загрузить данные пополнения складов");
        setResponse(null);
        setAllRows([]);
      } finally {
        setLoading(false);
      }
    }

    bootstrap();
  }, [initialAsOfDate]);

  const priorityOptions = useMemo(
    () => distinctSorted(allRows.map((r) => r.replenishment_priority)),
    [allRows],
  );
  const riskOptions = useMemo(() => distinctSorted(allRows.map((r) => r.depletion_risk)), [allRows]);

  const filteredRows = useMemo(() => {
    return allRows.filter((row) => {
      if (priorityFilter && row.replenishment_priority !== priorityFilter) return false;
      if (riskFilter && row.depletion_risk !== riskFilter) return false;
      return true;
    });
  }, [allRows, priorityFilter, riskFilter]);

  const summary = useMemo(() => {
    const total = filteredRows.length;
    const highPriority = filteredRows.filter((r) => isHighReplenishmentPriority(r.replenishment_priority)).length;
    const outOrCritical = filteredRows.filter((r) => isOutOrCriticalDepletion(r.depletion_risk)).length;
    const avgCover = averageDaysOfCover(filteredRows);
    return { total, highPriority, outOrCritical, avgCover };
  }, [filteredRows]);

  if (loading) {
    return (
      <main className="space-y-6 p-6">
        <PageHeader
          title="Склады и пополнение"
          subtitle="Оперативный снимок остатков с риском истощения и приоритетом пополнения."
        />
        <LoadingState message="Загрузка складов и пополнения…" />
      </main>
    );
  }

  if (error || !response) {
    return (
      <main className="space-y-6 p-6">
        <PageHeader
          title="Склады и пополнение"
          subtitle="Оперативный снимок остатков с риском истощения и приоритетом пополнения."
        />
        <ErrorState
          title="Не удалось загрузить данные складов"
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
        title="Склады и пополнение"
        subtitle="Представление складских остатков с приоритетом пополнения и риском истощения."
        className="border-0 pb-0"
      >
        <Link href="/app/dashboard" className={buttonClassNames("secondary")}>
          Дашборд
        </Link>
        <Link href="/app/critical-skus" className={buttonClassNames("secondary")}>
          Критические SKU
        </Link>
      </PageHeader>

      <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard title="Строк в выборке" value={fmtNumber(summary.total)} hint="После фильтров" />
        <MetricCard
          title="Высокий / критический приоритет"
          value={fmtNumber(summary.highPriority)}
          hint="Приоритет пополнения"
        />
        <MetricCard
          title="Критическое истощение"
          value={fmtNumber(summary.outOrCritical)}
          hint="Нет в наличии или критический риск"
        />
        <MetricCard
          title="Средние дни покрытия"
          value={summary.avgCover == null ? "—" : summary.avgCover.toFixed(1)}
          hint={summary.avgCover == null ? "Нет значений покрытия в выборке" : "Среднее по строкам с данными"}
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
            <span className="font-medium text-aivion-muted">Последнее обновление остатков:</span>{" "}
            {fmtDateTime(response.meta.last_stock_update)}
          </p>
          <p>
            <span className="font-medium text-aivion-muted">Семантика остатков:</span> {response.meta.stock_semantics}
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Фильтры</CardTitle>
          <CardDescription>На стороне клиента для загруженных строк.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
          <label className="min-w-[180px] text-sm">
            <span className="mb-1 block font-medium text-aivion-text">Приоритет пополнения</span>
            <select
              className="w-full rounded-lg border border-aivion-border px-3 py-2"
              value={priorityFilter}
              onChange={(e) => setPriorityFilter(e.target.value)}
            >
              <option value="">Все приоритеты</option>
              {priorityOptions.map((p) => (
                <option key={p} value={p}>
                  {translateReplenishmentPriority(p)}
                </option>
              ))}
            </select>
          </label>
          <label className="min-w-[180px] text-sm">
            <span className="mb-1 block font-medium text-aivion-text">Риск истощения</span>
            <select
              className="w-full rounded-lg border border-aivion-border px-3 py-2"
              value={riskFilter}
              onChange={(e) => setRiskFilter(e.target.value)}
            >
              <option value="">Все риски</option>
              {riskOptions.map((r) => (
                <option key={r} value={r}>
                  {riskLabel(r)}
                </option>
              ))}
            </select>
          </label>
          {(priorityFilter || riskFilter) && (
            <button
              type="button"
              className={buttonClassNames("secondary")}
              onClick={() => {
                setPriorityFilter("");
                setRiskFilter("");
              }}
            >
              Сбросить фильтры
            </button>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Список пополнения</CardTitle>
          <CardDescription>
            {filteredRows.length === allRows.length
              ? `Показаны все ${fmtNumber(filteredRows.length)} строк из API.`
              : `Показано ${fmtNumber(filteredRows.length)} из ${fmtNumber(allRows.length)} строк.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {filteredRows.length === 0 ? (
            <EmptyState
              title={allRows.length === 0 ? "Нет строк по остаткам" : "Нет подходящих строк"}
              message={
                allRows.length === 0
                  ? "Нет строк за выбранный период."
                  : "Попробуйте сбросить фильтры, чтобы увидеть полный список."
              }
              action={
                allRows.length > 0 ? (
                  <button
                    type="button"
                    className={buttonClassNames("secondary")}
                    onClick={() => {
                      setPriorityFilter("");
                      setRiskFilter("");
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
                    <th className="px-2 py-2">Доступно</th>
                    <th className="px-2 py-2">Зарезервировано</th>
                    <th className="px-2 py-2">Всего</th>
                    <th className="px-2 py-2">Дней покрытия</th>
                    <th className="px-2 py-2">Риск отсутствия</th>
                    <th className="px-2 py-2">Приоритет пополнения</th>
                    <th className="px-2 py-2">Число складов</th>
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
                      <td className="px-2 py-2">{fmtNumber(row.current_available_stock)}</td>
                      <td className="px-2 py-2">{fmtNumber(row.current_reserved_stock)}</td>
                      <td className="px-2 py-2">{fmtNumber(row.current_total_stock)}</td>
                      <td className="px-2 py-2">
                        {row.days_of_cover == null ? "—" : row.days_of_cover.toFixed(2)}
                      </td>
                      <td className="px-2 py-2">{riskLabel(row.depletion_risk)}</td>
                      <td className="px-2 py-2">
                        <span
                          className={`rounded px-2 py-0.5 text-xs font-medium ${priorityClass(row.replenishment_priority)}`}
                        >
                          {translateReplenishmentPriority(row.replenishment_priority)}
                        </span>
                      </td>
                      <td className="px-2 py-2">{fmtNumber(row.warehouse_count)}</td>
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
