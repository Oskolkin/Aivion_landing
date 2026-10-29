"use client";

import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";
import { renderMetricHighlightText } from "@/lib/metric-highlight";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { WhatChangedTone } from "@/lib/what-changed-view";

type Props = {
  tone: WhatChangedTone;
  icon: ReactNode;
  title: string;
  subtitle?: string;
  fullText?: string;
  className?: string;
};

const ITEM_ICON_WRAP: Record<WhatChangedTone, string> = {
  negative: "bg-[#fef3f2] text-[#b42318]",
  warning: "bg-[#fffaeb] text-[#b54708]",
  neutral: "bg-slate-100 text-slate-600",
  positive: "bg-[#ecfdf3] text-[#027a48]",
  empty_positive: "bg-slate-100 text-slate-500",
};

export function ChangeItemCard({ tone, icon, title, subtitle, fullText, className }: Props) {
  const tooltip = fullText ?? (subtitle ? `${title}: ${subtitle}` : title);

  return (
    <div
      className={cn(
        "flex items-start gap-2.5 rounded-xl border border-white/80 bg-white/95 px-3 py-2.5 shadow-[0_1px_2px_rgb(6_27_69/0.04)]",
        className,
      )}
      title={tooltip}
    >
      <span
        className={cn(
          "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg",
          ITEM_ICON_WRAP[tone] ?? ITEM_ICON_WRAP.neutral,
        )}
        aria-hidden="true"
      >
        {icon}
      </span>
      <div className="min-w-0 flex-1">
        <p className={cn("whitespace-normal break-words", dashboardTypography.listItemTitle)}>
          {renderMetricHighlightText(title)}
        </p>
        {subtitle ? (
          <p className={cn("mt-1 whitespace-normal break-words", dashboardTypography.listItemBody)}>
            {renderMetricHighlightText(subtitle)}
          </p>
        ) : null}
      </div>
    </div>
  );
}
