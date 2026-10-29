import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

export type PageHeaderProps = {
  title: string;
  subtitle?: string;
  children?: ReactNode;
  className?: string;
};

export function PageHeader({ title, subtitle, children, className }: PageHeaderProps) {
  return (
    <header
      className={cn(
        "flex flex-col gap-4 pb-2 md:flex-row md:items-start md:justify-between",
        className,
      )}
    >
      <div className="min-w-0">
        <h1 className={dashboardTypography.pageTitle}>{title}</h1>
        {subtitle ? <p className={dashboardTypography.pageDescription}>{subtitle}</p> : null}
      </div>
      {children ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2">{children}</div>
      ) : null}
    </header>
  );
}

export type SectionHeaderProps = {
  title: ReactNode;
  subtitle?: ReactNode;
  children?: ReactNode;
  className?: string;
};

export function SectionHeader({ title, subtitle, children, className }: SectionHeaderProps) {
  return (
    <div className={cn("flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between", className)}>
      <div className="min-w-0">
        <h2 className="text-lg font-semibold text-aivion-text">{title}</h2>
        {subtitle ? <p className="mt-0.5 text-sm text-aivion-muted">{subtitle}</p> : null}
      </div>
      {children ? <div className="flex shrink-0 flex-wrap items-center gap-2">{children}</div> : null}
    </div>
  );
}
