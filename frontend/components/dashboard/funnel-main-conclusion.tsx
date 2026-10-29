import { Lightbulb } from "lucide-react";
import { cn } from "@/components/ui/cn";
import type { FunnelMainConclusionSource } from "@/lib/funnel-transitions-display";
import { dashboardTypography } from "@/lib/dashboard-typography";

type FunnelMainConclusionProps = {
  text: string;
  source?: FunnelMainConclusionSource;
  highlightLabel?: string;
  /** Optional structured sections — only render three columns when all are present. */
  problem?: string | null;
  meaning?: string | null;
  recommendation?: string | null;
};

function renderConclusionText(text: string, highlightLabel?: string) {
  const trimmed = text.trim();
  if (!highlightLabel || !trimmed.includes(highlightLabel)) {
    return trimmed;
  }
  const parts = trimmed.split(highlightLabel);
  if (parts.length < 2) return trimmed;
  return (
    <>
      {parts[0]}
      <strong className="font-semibold text-sky-950">{highlightLabel}</strong>
      {parts.slice(1).join(highlightLabel)}
    </>
  );
}

export function FunnelMainConclusion({
  text,
  source,
  highlightLabel,
  problem,
  meaning,
  recommendation,
}: FunnelMainConclusionProps) {
  const trimmed = text.trim();
  const structured =
    Boolean(problem?.trim()) && Boolean(meaning?.trim()) && Boolean(recommendation?.trim());

  if (!trimmed && !structured) {
    return null;
  }

  return (
    <section aria-label="Главный вывод по воронке">
      <div
        className="rounded-2xl border border-sky-200 bg-sky-50/80 px-4 py-4 sm:px-5 sm:py-5"
        data-funnel-main-conclusion-source={source}
      >
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-sky-100">
            <Lightbulb className="h-4 w-4 text-sky-600" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <h4 className={cn(dashboardTypography.sectionTitle, "text-sky-900")}>Главный вывод</h4>

            {structured ? (
              <div className="mt-3 grid gap-3 sm:grid-cols-3 sm:gap-0">
                <div className="sm:border-r sm:border-sky-200/80 sm:pr-4">
                  <p className="text-xs font-semibold uppercase tracking-wide text-sky-700">Проблема</p>
                  <p className={cn("mt-1", dashboardTypography.aiSummary, "text-sky-950")}>{problem}</p>
                </div>
                <div className="border-t border-sky-200/80 pt-3 sm:border-t-0 sm:border-r sm:px-4 sm:pt-0">
                  <p className="text-xs font-semibold uppercase tracking-wide text-sky-700">Что это значит</p>
                  <p className={cn("mt-1", dashboardTypography.aiSummary, "text-sky-950")}>{meaning}</p>
                </div>
                <div className="border-t border-sky-200/80 pt-3 sm:border-t-0 sm:pl-4 sm:pt-0">
                  <p className="text-xs font-semibold uppercase tracking-wide text-sky-700">Что делать</p>
                  <p className={cn("mt-1", dashboardTypography.aiSummary, "text-sky-950")}>{recommendation}</p>
                </div>
              </div>
            ) : (
              <p className={cn("mt-1.5", dashboardTypography.aiSummary, "text-sky-950")}>
                {renderConclusionText(trimmed, highlightLabel)}
              </p>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
