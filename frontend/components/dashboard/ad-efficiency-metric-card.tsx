"use client";

import type { LucideIcon } from "lucide-react";
import { CircleHelp } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";

export type AdvertisingMetricCardProps = {
  testId: string;
  label: string;
  value: string;
  icon: LucideIcon;
  valueTone?: "primary" | "default";
  supportingText?: ReactNode;
  infoTooltip?: ReactNode;
};

function InfoHint({ children }: { children: ReactNode }) {
  return (
    <span className="group relative inline-flex shrink-0 text-slate-400">
      <button type="button" aria-label="Как рассчитывается показатель" className="rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500" onKeyDown={(event) => { if (event.key === "Escape") event.currentTarget.blur(); }}>
        <CircleHelp className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
      <span role="tooltip" className="pointer-events-none absolute right-0 top-6 z-30 hidden w-[min(23rem,calc(100vw-2rem))] rounded-xl border border-slate-700 bg-slate-900 p-4 text-left text-xs leading-relaxed text-slate-100 shadow-xl group-focus-within:block group-hover:block">{children}</span>
    </span>
  );
}

export function AdvertisingMetricCard({
  testId,
  label,
  value,
  icon: Icon,
  valueTone = "default",
  supportingText,
  infoTooltip,
}: AdvertisingMetricCardProps) {
  const displayValue = value === "—" ? "Нет данных" : value;

  return (
    <div
      className="flex min-h-[184px] flex-col rounded-[20px] border border-slate-200 bg-white p-[22px] shadow-[0_4px_18px_rgba(15,23,42,0.035)] sm:p-6"
      data-testid={testId}
      data-component="advertising-metric-card"
    >
      <div className="flex items-start gap-3">
        <div
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[13px] bg-blue-50 text-blue-600"
          aria-hidden="true"
        >
          <Icon className="h-5 w-5" />
        </div>
        <div className="flex min-w-0 items-start gap-1.5">
          <p className="min-w-0 text-[15px] font-semibold leading-[1.35] text-slate-800 sm:text-base">{label}</p>
          {infoTooltip ? <InfoHint>{infoTooltip}</InfoHint> : null}
        </div>
      </div>

      <p
        className={cn(
          "mt-5 text-[36px] font-bold leading-none tracking-tight tabular-nums sm:text-[40px]",
          valueTone === "primary" ? "text-blue-600" : "text-slate-950",
        )}
        data-testid={`${testId}-value`}
      >
        {displayValue}
      </p>

      {supportingText != null ? (
        <div
          className="mt-auto pt-4 text-[14px] font-medium leading-[1.45] text-slate-600 sm:text-[15px]"
          data-testid={`${testId}-supporting`}
        >
          {supportingText}
        </div>
      ) : null}
    </div>
  );
}
