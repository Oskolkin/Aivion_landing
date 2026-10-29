"use client";

import { Lightbulb } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import { domainIcon } from "@/lib/domain-presentation";
import type { OverviewAiSummaryView } from "@/lib/overview-view-model";

type Props = {
  summary: OverviewAiSummaryView;
};

export function OverviewAiSummary({ summary }: Props) {
  return (
    <section
      className="rounded-[14px] border border-aivion-border bg-white p-3.5 pb-5 sm:p-4 sm:pb-5"
      data-testid="overview-ai-summary"
      aria-labelledby="overview-ai-summary-title"
    >
      <div className="flex items-center gap-2">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-[#eff8ff] text-aivion-blue">
          <Lightbulb className="h-3.5 w-3.5" aria-hidden />
        </span>
        <h2 id="overview-ai-summary-title" className={dashboardTypography.cardTitleCompact}>
          AI-сводка
        </h2>
      </div>

      {summary.limited && !summary.headline ? (
        <p className={cn("mt-3", dashboardTypography.emptyState)}>{summary.limitedMessage}</p>
      ) : (
        <>
          {summary.headline ? (
            <p className="mt-3 text-base font-semibold leading-snug text-aivion-text sm:text-lg">
              {summary.headline}
            </p>
          ) : null}

          {summary.points.length > 0 ? (
            <ul className="mt-3 space-y-2.5">
              {summary.points.map((point) => {
                const Icon = domainIcon(point.category);
                return (
                  <li key={point.id} className="flex gap-2.5">
                    <span className="mt-0.5 inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-aivion-surface-soft text-aivion-blue">
                      <Icon className="h-3.5 w-3.5" aria-hidden />
                    </span>
                    <div className="min-w-0">
                      <p className="text-sm font-semibold leading-snug text-aivion-text sm:text-[15px]">
                        {point.title}
                      </p>
                      {point.detail ? (
                        <p className={cn("mt-0.5 line-clamp-2", dashboardTypography.secondary)}>
                          {point.detail}
                        </p>
                      ) : null}
                    </div>
                  </li>
                );
              })}
            </ul>
          ) : null}
        </>
      )}
    </section>
  );
}
