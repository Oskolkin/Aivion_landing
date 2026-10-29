"use client";

import { useEffect, useState } from "react";
import { useDashboardData } from "@/components/dashboard/shell/dashboard-data-provider";
import { getStockSKUs } from "@/lib/publication-dashboard-api";
import { formatMetric } from "@/lib/metric-format";
import { cn } from "@/components/ui/cn";

const BANDS = [
  { key: "out_of_stock", label: "Нет остатка" },
  { key: "coverage_0_3", label: "0–3 дня" },
  { key: "coverage_4_7", label: "4–7 дней" },
  { key: "coverage_8_14", label: "8–14 дней" },
] as const;

export function DashboardStockPage() {
  const { data } = useDashboardData();
  const stock = data?.stock;
  const publicationId = data?.publication?.id;
  const [filter, setFilter] = useState<string>("");
  const [rows, setRows] = useState<Record<string, unknown>[]>([]);

  useEffect(() => {
    if (!publicationId) return;
    void getStockSKUs({
      publication_id: publicationId,
      coverage_band: filter && filter !== "no_sales" ? filter : undefined,
      no_sales: filter === "no_sales",
      limit: 50,
    }).then((r) => setRows(r.items));
  }, [publicationId, filter]);

  if (!stock) return null;
  const bands = stock.coverage_bands ?? [];

  return (
    <div className="space-y-5" data-testid="dashboard-stock-page">
      <section className="rounded-[22px] border border-slate-200 bg-white p-5">
        <h1 className="text-xl font-bold">Остатки</h1>
        <p className="mt-1 text-sm text-slate-600">UNKNOWN не равен нулю. Известный ноль — «Нет остатка».</p>
      </section>
      <section className="rounded-[22px] border border-slate-200 bg-white p-5" data-testid="stock-low-coverage-chart">
        <h2 className="text-xl font-bold">Покрытие</h2>
        <div className="mt-6 flex items-end justify-around gap-3">
          {BANDS.map((b) => {
            const count = bands.find((x) => x.key === b.key)?.count ?? 0;
            return (
              <div key={b.key} className="flex flex-1 flex-col items-center">
                <p className="mb-2 text-sm font-bold tabular-nums">{count} SKU</p>
                <div className="h-40 w-full max-w-24 rounded-t-lg bg-blue-500" style={{ height: `${Math.max(8, count * 8)}px` }} />
                <p className="mt-2 text-xs">{b.label}</p>
              </div>
            );
          })}
        </div>
      </section>
      <div className="flex flex-wrap gap-2" data-testid="stock-filters">
        {BANDS.map((b) => (
          <button key={b.key} type="button" className={cn("rounded-full border px-3 py-1 text-sm", filter === b.key ? "border-blue-600 text-blue-700" : "border-slate-200")} onClick={() => setFilter(b.key)}>
            {b.label}
          </button>
        ))}
        <button type="button" className={cn("rounded-full border px-3 py-1 text-sm", filter === "no_sales" ? "border-blue-600 text-blue-700" : "border-slate-200")} onClick={() => setFilter("no_sales")}>
          Без продаж
        </button>
      </div>
      <div className="overflow-x-auto rounded-[14px] border border-slate-200 bg-white">
        <table className="min-w-full text-sm" data-testid="stock-sku-table">
          <thead>
            <tr className="border-b text-left text-slate-500">
              <th scope="col" className="px-3 py-2">SKU</th>
              <th scope="col" className="px-3 py-2">Остаток</th>
              <th scope="col" className="px-3 py-2">Покрытие</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const state = String(row.sellable_units_state ?? "");
              const units = row.sellable_units;
              const display = state !== "KNOWN" ? "—" : formatMetric("COUNT", String(units ?? 0));
              return (
                <tr key={String(row.sku_id)} className="border-b">
                  <td className="px-3 py-2">{String(row.sku_id)}</td>
                  <td className="px-3 py-2 tabular-nums">{display}</td>
                  <td className="px-3 py-2">{row.coverage_band ? String(row.coverage_band) : "—"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <section className="rounded-[14px] border border-slate-200 bg-white p-4">
        <h3 className="font-semibold">Что делать</h3>
        <ul className="mt-2 list-disc pl-5 text-sm">
          {(stock.findings ?? []).map((f) => (
            <li key={f.ref ?? f.title}>{f.title}</li>
          ))}
        </ul>
      </section>
    </div>
  );
}
