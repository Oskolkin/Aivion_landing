"use client";

import Link from "next/link";
import { ArrowDown, ArrowRight, ArrowUp, ChevronRight } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { OverviewWhatChangedRowView } from "@/lib/overview-view-model";
import type { WhatChangedTone } from "@/lib/what-changed-view";

type Props = {
  row: OverviewWhatChangedRowView;
};

const TONE_ICON: Record<WhatChangedTone, typeof ArrowDown> = {
  negative: ArrowDown,
  warning: ArrowDown,
  neutral: ArrowRight,
  positive: ArrowUp,
  empty_positive: ArrowRight,
};

const TONE_ICON_CLASS: Record<WhatChangedTone, string> = {
  negative: "text-[#b42318]",
  warning: "text-[#b54708]",
  neutral: "text-aivion-muted",
  positive: "text-[#027a48]",
  empty_positive: "text-aivion-muted-soft",
};

const STATUS_CLASS: Record<WhatChangedTone, string> = {
  negative: "text-[#b42318]",
  warning: "text-[#b54708]",
  neutral: "text-aivion-muted",
  positive: "text-[#027a48]",
  empty_positive: "text-aivion-muted",
};

export function OverviewWhatChangedRow({ row }: Props) {
  const Icon = TONE_ICON[row.tone] ?? ArrowRight;
  const content = (
    <>
      <span className={cn("mt-0.5 shrink-0", TONE_ICON_CLASS[row.tone])} aria-hidden>
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium leading-[1.45] text-aivion-text sm:text-[15px]">
          {row.title}
        </span>
        {row.context ? (
          <span className={cn("mt-1 block leading-[1.45]", dashboardTypography.secondary)}>
            {row.context}
          </span>
        ) : null}
      </span>
      <span
        className={cn(
          "w-[6.75rem] shrink-0 self-start pt-0.5 text-right text-xs font-semibold leading-snug",
          STATUS_CLASS[row.tone],
        )}
      >
        {row.statusLabel}
      </span>
      {row.href ? (
        <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-aivion-muted-soft" aria-hidden />
      ) : (
        <span className="w-4 shrink-0" aria-hidden />
      )}
    </>
  );

  const className = cn(
    "flex items-start gap-2.5 rounded-lg px-1.5 py-2.5 transition-colors sm:gap-3 sm:px-2 sm:py-3",
    row.href
      ? "hover:bg-aivion-surface-soft focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-aivion-blue"
      : "cursor-default",
  );

  if (row.href) {
    return (
      <Link href={row.href} className={className} data-testid="overview-what-changed-row">
        {content}
      </Link>
    );
  }

  return (
    <div className={className} data-testid="overview-what-changed-row">
      {content}
    </div>
  );
}
