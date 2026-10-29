"use client";

import Link from "next/link";
import { OverviewMiniLineChart } from "@/components/dashboard/overview/overview-mini-line-chart";
import { cn } from "@/components/ui/cn";
import { dashboardTextColor, dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewAdvertisingPreviewView } from "@/lib/overview-view-model";
import { formatPulseChartDate } from "@/lib/seller-pulse-view";

type Props = {
  advertising: OverviewAdvertisingPreviewView;
};

const pctFmt = new Intl.NumberFormat("ru-RU", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

function DeltaTone({
  tone,
  children,
}: {
  tone: "warning" | "success" | "neutral";
  children: React.ReactNode;
}) {
  return (
    <p
      className={cn(
        "mt-1 text-sm font-medium",
        tone === "success"
          ? dashboardTextColor.positive
          : tone === "warning"
            ? dashboardTextColor.warning
            : dashboardTextColor.muted,
      )}
    >
      {children}
    </p>
  );
}

export function OverviewAdvertisingPreview({ advertising }: Props) {
  const limitedEmpty =
    advertising.limited &&
    advertising.weekValueDisplay === "—" &&
    advertising.yesterdayValueDisplay === "—";

  return (
    <section
      className="flex h-full flex-col rounded-[14px] border border-aivion-border bg-white p-4"
      data-testid="overview-advertising-preview"
      aria-labelledby="overview-advertising-preview-title"
    >
      <h2 id="overview-advertising-preview-title" className={dashboardTypography.cardTitleCompact}>
        Реклама
      </h2>

      {limitedEmpty ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{advertising.limitedMessage}</p>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className="mt-2.5 grid grid-cols-2 gap-3">
            <div data-testid="overview-ad-week-drr">
              <p className={dashboardTypography.metadata}>{advertising.weekMetricLabel}</p>
              <p className="mt-1 text-2xl font-bold tabular-nums text-aivion-text">
                {advertising.weekValueDisplay}
              </p>
              {advertising.weekDeltaText ? (
                <DeltaTone tone={advertising.weekDeltaTone}>{advertising.weekDeltaText}</DeltaTone>
              ) : null}
            </div>
            <div data-testid="overview-ad-yesterday-drr">
              <p className={dashboardTypography.metadata}>{advertising.yesterdayMetricLabel}</p>
              <p className="mt-1 text-2xl font-bold tabular-nums text-aivion-text">
                {advertising.yesterdayValueDisplay}
              </p>
              {advertising.yesterdayDeltaText ? (
                <DeltaTone tone={advertising.yesterdayDeltaTone}>
                  {advertising.yesterdayDeltaText}
                </DeltaTone>
              ) : null}
            </div>
          </div>

          {advertising.hasChart ? (
            <div className="mt-3 flex min-h-0 flex-1 items-stretch">
              <OverviewMiniLineChart
                points={advertising.chartPoints}
                stroke="#0873f6"
                size="panel"
                formatYTick={(value) => `${pctFmt.format(value)}%`}
                formatTooltipValue={(value) => `${pctFmt.format(value)}%`}
                formatTooltipDate={formatPulseChartDate}
                ariaLabel="Динамика ДРР по дням"
                className="h-full w-full"
                heightClassName="h-full min-h-[12rem]"
              />
            </div>
          ) : null}
        </div>
      )}

      <div className="mt-auto pt-3">
        <Link
          href={advertising.href}
          className={cn(dashboardTypography.ctaLink, "inline-flex items-center gap-1")}
          data-testid="overview-open-advertising"
        >
          Открыть раздел
          <span aria-hidden>→</span>
        </Link>
      </div>
    </section>
  );
}
