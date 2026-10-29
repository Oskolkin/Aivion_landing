import { AlertTriangle } from "lucide-react";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

type FunnelInsightBannerProps = {
  text: string;
};

export function FunnelInsightBanner({ text }: FunnelInsightBannerProps) {
  const trimmed = text.trim();
  if (!trimmed) return null;

  return (
    <div
      className="flex items-start gap-2.5 rounded-xl border border-amber-300/80 bg-amber-50/90 px-3.5 py-3 sm:px-4"
      role="status"
      data-funnel-insight-banner=""
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" aria-hidden="true" />
      <p className={cn(dashboardTypography.body, "text-amber-950")}>
        <span className="font-bold">Главный инсайт:</span> {trimmed}
      </p>
    </div>
  );
}
