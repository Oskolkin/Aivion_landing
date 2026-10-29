"use client";

import { useMemo, useState } from "react";
import { formatMetric } from "@/lib/metric-format";

type PreviewRow = Record<string, unknown>;

type Props = {
  rows: PreviewRow[];
};

const TABS = [
  { id: "ORDERS", label: "Заказы" },
  { id: "KEY", label: "Ключевые" },
] as const;

function tabsOf(row: PreviewRow): string[] {
  const raw = row.focus_tabs;
  if (Array.isArray(raw)) {
    return raw.map((v) => String(v));
  }
  return [];
}

export function OverviewSkuFocus({ rows }: Props) {
  const [tab, setTab] = useState<(typeof TABS)[number]["id"]>("ORDERS");
  const classified = rows.some((row) => tabsOf(row).length > 0);
  const visible = useMemo(() => {
    if (!classified) return rows;
    return rows.filter((row) => tabsOf(row).includes(tab));
  }, [classified, rows, tab]);

  return (
    <section className="rounded-[14px] border border-aivion-border bg-white p-4" data-testid="sku-focus-section" id="sku-focus">
      <h2 className="text-base font-semibold">SKU-фокус</h2>
      <div className="mt-3 flex gap-2" data-testid="sku-focus-tabs">
        {TABS.map((item) => (
          <button
            key={item.id}
            type="button"
            className={`rounded-full border px-3 py-1 text-sm ${tab === item.id ? "border-blue-600 text-blue-700" : "border-slate-200"}`}
            data-testid={`sku-focus-tab-${item.id}`}
            onClick={() => setTab(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>
      <ul className="mt-3 divide-y" data-testid="sku-focus-grid">
        {visible.map((row) => {
          const state = String(row.sellable_units_state ?? "");
          const units = state === "KNOWN" ? formatMetric("COUNT", String(row.sellable_units ?? 0)) : "—";
          return (
            <li key={String(row.sku_id)} className="flex justify-between py-2 text-sm">
              <span>SKU {String(row.sku_id)}</span>
              <span className="tabular-nums">{units}</span>
            </li>
          );
        })}
      </ul>
      {visible.length === 0 ? <p className="mt-2 text-sm text-slate-500">Нет SKU в этой вкладке.</p> : null}
    </section>
  );
}
