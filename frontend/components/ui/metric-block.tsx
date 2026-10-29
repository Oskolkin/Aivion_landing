import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";

export type MetricBlockProps = {
  label?: string;
  value: ReactNode;
  className?: string;
};

export function MetricBlock({ label, value, className }: MetricBlockProps) {
  return (
    <div className={className}>
      {label ? (
        <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-400">{label}</p>
      ) : null}
      <p className={cn("text-sm font-medium leading-5 text-slate-700", label && "mt-1")}>{value}</p>
    </div>
  );
}
