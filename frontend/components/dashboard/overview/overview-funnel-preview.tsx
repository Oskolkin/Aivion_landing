"use client";

import Link from "next/link";
import { cn } from "@/components/ui/cn";
import { dashboardTextColor, dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewFunnelPreviewView, OverviewFunnelStageView } from "@/lib/overview-view-model";

type Props = {
  funnel: OverviewFunnelPreviewView;
};

/** Compact preview funnel: top step ~180–200px; column ~40–45% of card. */
const FUNNEL_WIDTHS = [100, 82, 64, 46];
const FUNNEL_FILLS = ["#b2d4fb", "#7eb6f7", "#4a97f2", "#0873f6"];

function MiniFunnelVisual({ stages }: { stages: OverviewFunnelStageView[] }) {
  return (
    <ul
      className="mt-4 flex w-full flex-col gap-3.5"
      aria-label="Этапы воронки"
      data-testid="overview-mini-funnel"
    >
      {stages.map((stage, index) => {
        const width = FUNNEL_WIDTHS[Math.min(index, FUNNEL_WIDTHS.length - 1)] ?? 46;
        const fill = FUNNEL_FILLS[Math.min(index, FUNNEL_FILLS.length - 1)] ?? "#0873f6";
        return (
          <li key={stage.key} className="flex items-center gap-3 sm:gap-3.5">
            <div
              className="flex w-[190px] max-w-[48%] shrink-0 justify-center"
              aria-hidden
            >
              <div
                className="h-[22px] rounded-[2px]"
                style={{
                  width: `${width}%`,
                  background: fill,
                  clipPath: "polygon(6% 0, 94% 0, 100% 100%, 0 100%)",
                }}
              />
            </div>
            <div className="min-w-0 flex-1 leading-[1.35]">
              <p className="text-[13px] font-semibold text-aivion-text sm:text-sm">{stage.label}</p>
              <p className="mt-0.5 text-[13px] tabular-nums text-aivion-text sm:text-sm">
                <span>{stage.valueLabel}</span>
                {stage.conversionLabel ? (
                  <span className="ml-1.5 text-aivion-muted">{stage.conversionLabel}</span>
                ) : null}
              </p>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

export function OverviewFunnelPreview({ funnel }: Props) {
  return (
    <section
      className="flex h-full flex-col rounded-[14px] border border-aivion-border bg-white p-4"
      data-testid="overview-funnel-preview"
      aria-labelledby="overview-funnel-preview-title"
    >
      <h2 id="overview-funnel-preview-title" className={dashboardTypography.cardTitleCompact}>
        Воронка продаж
      </h2>

      {funnel.limited && funnel.stages.length === 0 ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{funnel.limitedMessage}</p>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <p className={cn("mt-2.5", dashboardTypography.metadata)}>{funnel.conversionLabel}</p>
          <p className="mt-1 text-2xl font-bold tabular-nums text-aivion-text">
            {funnel.conversionValue ?? "—"}
          </p>
          {funnel.deltaText ? (
            <p
              className={cn(
                "mt-1 text-sm font-medium",
                funnel.deltaPositive === true
                  ? dashboardTextColor.positive
                  : funnel.deltaPositive === false
                    ? dashboardTextColor.negative
                    : dashboardTextColor.muted,
              )}
            >
              {funnel.deltaText}
            </p>
          ) : null}

          <MiniFunnelVisual stages={funnel.stages} />
        </div>
      )}

      <div className="mt-auto pt-3">
        <Link
          href={funnel.href}
          className={cn(dashboardTypography.ctaLink, "inline-flex items-center gap-1")}
          data-testid="overview-open-funnel"
        >
          Открыть раздел
          <span aria-hidden>→</span>
        </Link>
      </div>
    </section>
  );
}
