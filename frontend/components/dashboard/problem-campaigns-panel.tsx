"use client";

import { useCallback, useId, useState, type KeyboardEvent } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "@/components/ui/cn";
import type { AdCampaignDisplayRow } from "@/lib/dashboard-home-types";
import { formatDrrPercent, formatMoneyRub, formatOrders } from "@/lib/ad-efficiency-display";

type TabKey = "high_drr" | "no_orders";

type Props = {
  highDrrRows: AdCampaignDisplayRow[];
  noOrderRows: AdCampaignDisplayRow[];
  highDrrCount: number;
  noOrdersCount: number;
  thresholdPct: number;
};

const TAB_ORDER: TabKey[] = ["high_drr", "no_orders"];

function tabLabel(key: TabKey, highCount: number, noOrderCount: number): string {
  if (key === "high_drr") return `Высокий ДРР · ${highCount}`;
  return `Без заказов · ${noOrderCount}`;
}

function emptyMessage(key: TabKey, thresholdPct: number): string {
  if (key === "high_drr") return `Нет кампаний с ДРР выше ${formatDrrPercent(thresholdPct).replace("%", "")}%.`;
  return "Нет кампаний с расходом без заказов.";
}

export function ProblemCampaignsPanel({
  highDrrRows,
  noOrderRows,
  highDrrCount,
  noOrdersCount,
  thresholdPct,
}: Props) {
  const [activeTab, setActiveTab] = useState<TabKey>("high_drr");
  const [expanded, setExpanded] = useState(false);
  const tablistId = useId();

  const rows = activeTab === "high_drr" ? highDrrRows : noOrderRows;
  const totalCount = activeTab === "high_drr" ? highDrrCount : noOrdersCount;
  const visibleRows = expanded ? rows : rows.slice(0, 3);
  const canExpand = rows.length > 3;

  const onTabKeyDown = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>, current: TabKey) => {
      const idx = TAB_ORDER.indexOf(current);
      if (event.key === "ArrowRight" || event.key === "ArrowDown") {
        event.preventDefault();
        setActiveTab(TAB_ORDER[(idx + 1) % TAB_ORDER.length]);
      } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        event.preventDefault();
        setActiveTab(TAB_ORDER[(idx - 1 + TAB_ORDER.length) % TAB_ORDER.length]);
      }
    },
    [],
  );

  return (
    <div
      className="rounded-[20px] border border-slate-200 bg-white p-5 shadow-[0_4px_18px_rgba(15,23,42,0.035)]"
      data-testid="problem-campaigns-panel"
    >
      <h3 className="text-lg font-bold text-slate-950">Проблемные кампании</h3>

      <div
        className="mt-4 flex flex-wrap gap-2"
        role="tablist"
        aria-label="Проблемные кампании"
        id={tablistId}
      >
        {TAB_ORDER.map((key) => {
          const selected = activeTab === key;
          return (
            <button
              key={key}
              type="button"
              role="tab"
              id={`${tablistId}-${key}`}
              aria-selected={selected}
              aria-controls={`${tablistId}-panel`}
              tabIndex={selected ? 0 : -1}
              onClick={() => {
                setActiveTab(key);
                setExpanded(false);
              }}
              onKeyDown={(e) => onTabKeyDown(e, key)}
              className={cn(
                "inline-flex h-9 items-center rounded-full border px-3.5 text-[13px] font-semibold transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600",
                selected
                  ? "border-blue-600 bg-blue-50 text-blue-600"
                  : "border-slate-200 bg-white text-slate-600 hover:border-slate-300",
              )}
              data-testid={`ad-efficiency-tab-${key}`}
            >
              {tabLabel(key, highDrrCount, noOrdersCount)}
            </button>
          );
        })}
      </div>

      <div
        role="tabpanel"
        id={`${tablistId}-panel`}
        aria-labelledby={`${tablistId}-${activeTab}`}
        className="mt-4"
        data-testid={`ad-efficiency-campaign-table-${activeTab}`}
      >
        {rows.length === 0 && totalCount === 0 ? (
          <p className="text-sm leading-relaxed text-slate-600">{emptyMessage(activeTab, thresholdPct)}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full table-fixed text-sm">
              <colgroup>
                <col className="w-[46%]" />
                <col className="w-[14%]" />
                <col className="w-[16%]" />
                <col className="w-[10%]" />
                <col className="w-[14%]" />
              </colgroup>
              <thead>
                <tr className="border-b border-slate-200 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  <th className="pb-2 pr-3 font-semibold">Кампания</th>
                  <th className="pb-2 pr-2 text-right font-semibold">ДРР</th>
                  <th className="pb-2 pr-2 text-right font-semibold">Расход</th>
                  <th className="pb-2 pr-2 text-right font-semibold">Заказы</th>
                  <th className="pb-2 text-right font-semibold">Выручка</th>
                </tr>
              </thead>
              <tbody>
                {visibleRows.map((row) => {
                  const drrHigh =
                    row.drr_pct != null && thresholdPct != null && row.drr_pct > thresholdPct;
                  return (
                    <tr
                      key={row.campaign_external_id}
                      className="border-b border-slate-100 last:border-0 hover:bg-slate-50/80"
                    >
                      <td className="min-h-[48px] py-3 pr-3 text-[15px] font-medium text-slate-800">
                        <span className="whitespace-normal break-words" title={row.campaign_name}>
                          {row.campaign_name}
                        </span>
                      </td>
                      <td
                        className={cn(
                          "py-3 pr-2 text-right text-[14px] tabular-nums",
                          drrHigh ? "font-semibold text-red-600" : "text-slate-700",
                        )}
                      >
                        {formatDrrPercent(row.drr_pct)}
                      </td>
                      <td className="py-3 pr-2 text-right text-[14px] tabular-nums text-slate-700">
                        {formatMoneyRub(row.spend)}
                      </td>
                      <td className="py-3 pr-2 text-right text-[14px] tabular-nums text-slate-700">
                        {formatOrders(row.orders)}
                      </td>
                      <td className="py-3 text-right text-[14px] tabular-nums text-slate-700">
                        {formatMoneyRub(row.revenue)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {canExpand ? (
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            className="mt-4 inline-flex items-center gap-1 text-sm font-semibold text-blue-600 hover:text-blue-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
            data-testid={`ad-efficiency-toggle-${activeTab}`}
            aria-expanded={expanded}
          >
            {expanded ? "Скрыть список" : `Показать все ${Math.min(totalCount, rows.length)} кампаний`}
            <ChevronRight className={cn("h-4 w-4 transition-transform", expanded ? "rotate-90" : "")} aria-hidden="true" />
          </button>
        ) : null}
        {!canExpand && totalCount > rows.length ? (
          <p className="mt-3 text-[13px] text-slate-500" data-testid={`ad-efficiency-limited-${activeTab}`}>
            Показаны первые {rows.length} из {totalCount} кампаний
          </p>
        ) : null}
      </div>
    </div>
  );
}
