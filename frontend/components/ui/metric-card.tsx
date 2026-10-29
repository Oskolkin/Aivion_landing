import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";
import { Card, CardContent } from "@/components/ui/card";

export type MetricCardProps = {
  title: string;
  value: ReactNode;
  hint?: string;
  trend?: ReactNode;
  className?: string;
};

export function MetricCard({ title, value, hint, trend, className }: MetricCardProps) {
  return (
    <Card className={cn("overflow-hidden", className)}>
      <CardContent className="py-4">
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-sm font-medium text-aivion-muted">{title}</h3>
          {trend}
        </div>
        <p className="mt-1 text-2xl font-semibold tabular-nums text-aivion-text">{value}</p>
        {hint ? <p className="mt-2 text-xs text-aivion-muted-soft">{hint}</p> : null}
      </CardContent>
    </Card>
  );
}

export type ActionCardProps = {
  title: string;
  description?: string;
  badge?: ReactNode;
  children?: ReactNode;
  className?: string;
  onClick?: () => void;
};

export function ActionCard({
  title,
  description,
  badge,
  children,
  className,
  onClick,
}: ActionCardProps) {
  const content = (
    <>
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-aivion-text">{title}</h3>
        {badge}
      </div>
      {description ? <p className="mt-1 text-sm text-aivion-muted">{description}</p> : null}
      {children ? <div className="mt-3">{children}</div> : null}
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "w-full rounded-aivion-lg border border-aivion-border bg-aivion-surface p-4 text-left shadow-aivion-sm transition-shadow hover:shadow-aivion-md focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-aivion-blue",
          className,
        )}
      >
        {content}
      </button>
    );
  }

  return (
    <Card className={cn("transition-shadow hover:shadow-aivion-md", className)}>
      <CardContent className="py-4">{content}</CardContent>
    </Card>
  );
}
