import type { ReactNode } from "react";
import { AlertTriangle, Cable, CircleHelp, Info, LockKeyhole, RefreshCw, type LucideIcon } from "lucide-react";
import type { WidgetBase, WidgetStatus } from "@/lib/dashboard-home-types";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/components/ui/cn";
import { dashboardTypography } from "@/lib/dashboard-typography";

const STATUS_COPY: Record<WidgetStatus, string> = {
  available: "",
  partial: "Данные частично доступны. Часть показателей может быть ограничена.",
  not_enough_data: "Данных пока недостаточно.",
  source_not_available: "Источник данных не подключён.",
  source_access_denied: "Доступ к данным ограничен.",
  premium_required: "Требуется расширенный тариф Ozon.",
  source_pending: "Данные скоро появятся.",
  error: "Не удалось загрузить блок.",
};

type WidgetStateTone = "neutral" | "info" | "warning" | "danger";

type WidgetStateSemantic = { tone: WidgetStateTone; Icon: LucideIcon };

/**
 * Maps each blocked widget status to a semantic tone + icon.
 * `partial` is handled separately — it renders content with an inline notice.
 */
const STATUS_SEMANTIC: Record<Exclude<WidgetStatus, "available" | "partial">, WidgetStateSemantic> = {
  not_enough_data: { tone: "neutral", Icon: CircleHelp },
  source_pending: { tone: "info", Icon: RefreshCw },
  source_access_denied: { tone: "warning", Icon: LockKeyhole },
  premium_required: { tone: "warning", Icon: LockKeyhole },
  source_not_available: { tone: "warning", Icon: Cable },
  error: { tone: "danger", Icon: AlertTriangle },
};

const TONE_ICON_CLASS: Record<WidgetStateTone, string> = {
  neutral: "text-aivion-muted-soft",
  info: "text-sky-600",
  warning: "text-amber-600",
  danger: "text-rose-600",
};

export function isWidgetAvailable(status?: WidgetStatus | string): boolean {
  // "ok" is a legacy alias historically emitted by dashboardfacts for ready
  // factual blocks; treat it as available so published Unified snapshots render.
  return !status || status === "available" || status === "ok";
}

/** Widgets that may render factual content (full or limited). */
export function isWidgetRenderable(status?: WidgetStatus | string): boolean {
  return !status || status === "available" || status === "ok" || status === "partial";
}

export function WidgetState({ widget, children }: { widget: WidgetBase; children?: ReactNode }) {
  const status = (widget.status ?? "available") as WidgetStatus;
  if (status === "partial") {
    const message = widget.message?.trim() || STATUS_COPY.partial;
    return (
      <div className="space-y-3">
        <div
          className={cn("flex items-start gap-2 rounded-lg border border-sky-200 bg-sky-50 px-3 py-2", dashboardTypography.warningBanner)}
          data-widget-state-tone="info"
          role="status"
        >
          <Info className={cn("mt-0.5 h-4 w-4 shrink-0", TONE_ICON_CLASS.info)} aria-hidden="true" />
          <p className="text-left">{message}</p>
        </div>
        {children}
      </div>
    );
  }
  if (isWidgetAvailable(status)) {
    return <>{children}</>;
  }
  const semantic = STATUS_SEMANTIC[status as Exclude<WidgetStatus, "available" | "partial">] ?? STATUS_SEMANTIC.error;
  const message = widget.message || STATUS_COPY[status as WidgetStatus] || STATUS_COPY.error;
  const Icon = semantic.Icon;
  return (
    <Card
      className="border-dashed border-aivion-border bg-aivion-surface-soft"
      data-widget-state-tone={semantic.tone}
      role={semantic.tone === "danger" ? "status" : undefined}
    >
      <CardContent className="flex flex-col items-center gap-2 py-8 text-center">
        <Icon className={cn("h-5 w-5 shrink-0", TONE_ICON_CLASS[semantic.tone])} aria-hidden="true" />
        <p className={dashboardTypography.emptyState}>{message}</p>
      </CardContent>
    </Card>
  );
}

export function severityColor(severity?: string): string {
  switch ((severity ?? "").toLowerCase()) {
    case "critical":
      return "text-[#b42318] bg-[#fef3f2] ring-[#fecdca]";
    case "high":
      return "text-[#c4320a] bg-[#fff4ed] ring-[#fddcab]";
    case "medium":
      return "text-[#b54708] bg-[#fffaeb] ring-[#fedf89]";
    case "attention":
      return "text-[#0369a1] bg-[#f0f9ff] ring-[#bae6fd]";
    default:
      return "text-[#027a48] bg-[#ecfdf3] ring-[#abefc6]";
  }
}

export function healthScoreColor(score?: number | null): string {
  if (score == null) return "text-aivion-muted";
  if (score >= 85) return "text-aivion-success";
  if (score >= 70) return "text-sky-700";
  if (score >= 50) return "text-aivion-warning";
  return "text-aivion-danger";
}

export function severityBadgeClass(severity?: string): string {
  return cn(
    "inline-flex items-center rounded-full px-2 py-0.5 font-medium ring-1 ring-inset text-[11px] sm:text-xs",
    severityColor(severity),
  );
}
