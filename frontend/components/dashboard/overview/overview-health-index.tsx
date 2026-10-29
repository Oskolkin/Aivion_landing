"use client";

import {
  AlertTriangle,
  ArrowDown,
  Filter,
  Megaphone,
  Package,
  TrendingDown,
} from "lucide-react";
import { cn } from "@/components/ui/cn";
import { StatusPill } from "@/components/ui/status-pill";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewHealthIndexView } from "@/lib/overview-view-model";

type Props = {
  health: OverviewHealthIndexView;
};

function factorIcon(category: string | null | undefined, title: string) {
  const key = `${category ?? ""} ${title}`.toLowerCase();
  if (key.includes("stock") || key.includes("остат") || key.includes("sku") || key.includes("surplus") || key.includes("дефицит")) {
    return Package;
  }
  if (key.includes("ad") || key.includes("реклам") || key.includes("advertising") || key.includes("дрр")) {
    return Megaphone;
  }
  if (key.includes("funnel") || key.includes("ворон") || key.includes("конверс")) {
    return Filter;
  }
  if (key.includes("sales") || key.includes("продаж") || key.includes("выруч")) {
    return TrendingDown;
  }
  if (key.includes("cancel") || key.includes("alert") || key.includes("avail")) {
    return AlertTriangle;
  }
  return ArrowDown;
}

export function OverviewHealthIndex({ health }: Props) {
  return (
    <section
      id="overview-health-index"
      className="flex h-fit self-start flex-col rounded-[14px] border border-aivion-border bg-white p-3.5 sm:p-4"
      data-testid="overview-health-index"
      aria-labelledby="overview-health-index-title"
    >
      <h2 id="overview-health-index-title" className={dashboardTypography.cardTitleCompact}>
        Индекс состояния
      </h2>

      {health.limited && health.scoreLabel == null ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{health.limitedMessage}</p>
      ) : (
        <div className="mt-3 grid min-w-0 grid-cols-1 gap-5 xl:grid-cols-[minmax(0,0.38fr)_minmax(0,0.62fr)] xl:gap-7">
          <div className="min-w-0">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-[1.75rem] font-bold tabular-nums tracking-tight text-aivion-text sm:text-3xl">
              {health.scoreValue != null ? (
                <>
                  {health.scoreValue}
                  <span className="ml-1 text-sm font-semibold text-aivion-muted sm:text-base">
                    / 100
                  </span>
                </>
              ) : (
                health.scoreLabel
              )}
            </p>
            <StatusPill variant={health.severityPillVariant} size="sm">
              {health.severityLabel}
            </StatusPill>
          </div>

          <div className="mt-2.5 h-1.5 overflow-hidden rounded-full bg-[#eef2f6]" aria-hidden>
            <div
              className="h-full rounded-full bg-aivion-blue transition-[width] duration-300"
              style={{ width: `${health.scorePercent}%` }}
            />
          </div>

          </div>
          {health.factors.length > 0 ? (
            <div className="min-w-0">
              <p className={dashboardTypography.metadata}>Ключевые факторы снижения</p>
              <ul className="mt-1.5 space-y-1.5">
                {health.factors.map((factor) => {
                  const Icon = factorIcon(factor.category, factor.title);
                  return (
                    <li
                      key={factor.id}
                      className="flex items-start gap-2 text-sm"
                      data-testid="overview-health-factor"
                    >
                      <span
                        className="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-md bg-[#fef3f2] text-[#b42318]"
                        aria-hidden
                      >
                        <Icon className="h-3.5 w-3.5" />
                      </span>
                      <span className="min-w-0 text-aivion-text">{factor.title}</span>
                    </li>
                  );
                })}
              </ul>
            </div>
          ) : null}
        </div>
      )}
    </section>
  );
}
