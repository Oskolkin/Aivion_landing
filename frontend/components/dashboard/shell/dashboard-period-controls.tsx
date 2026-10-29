"use client";

import { CalendarDays, Clock3, Link2 } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewPeriodControlsView } from "@/lib/overview-view-model";

type Props = {
  period: OverviewPeriodControlsView | null;
  className?: string;
  variant?: "card" | "split";
};

export function DashboardPeriodControls({ period, className, variant = "card" }: Props) {
  if (!period || period.groups.length === 0) {
    return (
      <div
        className={cn(
          "rounded-[13px] border border-aivion-border bg-white px-3.5 py-2.5 text-sm text-aivion-muted",
          className,
        )}
        data-testid="dashboard-period-controls"
      >
        Период анализа появится после загрузки данных
      </div>
    );
  }

  if (variant === "split") {
    return (
      <dl
        className={cn("flex max-w-full flex-wrap items-stretch justify-end gap-2", className)}
        data-testid="dashboard-period-controls"
        data-variant="split"
      >
        {period.analysisPeriod ? (
          <div className="flex min-h-10 items-center gap-2 rounded-[11px] border border-aivion-border bg-white px-3 py-2">
            <CalendarDays className="h-4 w-4 shrink-0 text-aivion-muted" aria-hidden />
            <dt className="sr-only">Период анализа</dt>
            <dd className="whitespace-nowrap text-xs font-semibold tabular-nums text-aivion-text">
              {period.analysisPeriod}
            </dd>
          </div>
        ) : null}
        {period.hasComparison && period.comparisonPeriod ? (
          <div className="flex min-h-10 items-center gap-2 rounded-[11px] border border-aivion-border bg-white px-3 py-2">
            <Link2 className="h-4 w-4 shrink-0 text-aivion-muted" aria-hidden />
            <dt className="sr-only">Сравнение</dt>
            <dd className="whitespace-nowrap text-xs tabular-nums text-aivion-text">
              Сравнение: {period.comparisonPeriod}
            </dd>
          </div>
        ) : null}
        {period.updatedAt ? (
          <div className="flex min-h-10 items-center gap-2 rounded-[11px] border border-aivion-border bg-white px-3 py-2">
            <Clock3 className="h-4 w-4 shrink-0 text-aivion-muted" aria-hidden />
            <dt className="sr-only">Обновлено</dt>
            <dd className="whitespace-nowrap text-xs tabular-nums text-aivion-text">
              Обновлено: {period.updatedAt}
            </dd>
          </div>
        ) : null}
      </dl>
    );
  }

  return (
    <div
      className={cn(
        "flex items-start gap-2.5 rounded-[13px] border border-aivion-border bg-white px-3.5 py-2.5 shadow-none sm:min-w-[15.5rem]",
        className,
      )}
      data-testid="dashboard-period-controls"
    >
      <CalendarDays
        className="mt-0.5 h-4 w-4 shrink-0 text-aivion-muted"
        aria-hidden
      />
      <dl className="min-w-0 flex-1 text-left sm:text-right">
        {period.analysisPeriod ? (
          <div>
            <dt className="sr-only">Период анализа</dt>
            <dd className="text-sm font-semibold tabular-nums leading-snug text-aivion-text">
              {period.analysisPeriod}
            </dd>
          </div>
        ) : null}
        {period.hasComparison && period.comparisonPeriod ? (
          <div className="mt-0.5">
            <dt className="sr-only">Сравнение</dt>
            <dd className={cn(dashboardTypography.metadata, "tabular-nums")}>
              Сравнение: {period.comparisonPeriod}
            </dd>
          </div>
        ) : (
          <div className={cn("mt-0.5", dashboardTypography.metadata, "text-aivion-muted-soft")}>
            Нет сопоставимого периода
          </div>
        )}
        {period.updatedAt ? (
          <div className="mt-0.5">
            <dt className="sr-only">Обновлено</dt>
            <dd className={cn(dashboardTypography.metadata, "tabular-nums")}>
              Обновлено: {period.updatedAt}
            </dd>
          </div>
        ) : null}
      </dl>
    </div>
  );
}
