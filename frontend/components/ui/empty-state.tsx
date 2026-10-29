import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";

export type EmptyStateProps = {
  title: string;
  /** Optional supporting line. Omit for short, single-line empty states. */
  message?: string;
  action?: ReactNode;
  icon?: ReactNode;
  className?: string;
};

export function EmptyState({ title, message, action, icon, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        "rounded-aivion-lg border border-dashed border-aivion-border bg-aivion-surface-soft px-6 py-10 text-center",
        className,
      )}
    >
      {icon ? <div className="mx-auto mb-3 flex justify-center text-aivion-muted-soft">{icon}</div> : null}
      <p className="text-sm font-semibold text-aivion-text">{title}</p>
      {message ? <p className="mx-auto mt-1.5 max-w-md text-sm text-aivion-muted">{message}</p> : null}
      {action ? <div className="mt-5 flex justify-center">{action}</div> : null}
    </div>
  );
}
