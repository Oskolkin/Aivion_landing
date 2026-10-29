import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

export type SectionHeadingProps = {
  title: ReactNode;
  description?: ReactNode;
  className?: string;
};

export function SectionHeading({ title, description, className }: SectionHeadingProps) {
  return (
    <div className={cn("min-w-0", className)}>
      <h4 className={dashboardTypography.sectionTitle}>{title}</h4>
      {description ? <p className={cn("mt-0.5", dashboardTypography.secondary)}>{description}</p> : null}
    </div>
  );
}
