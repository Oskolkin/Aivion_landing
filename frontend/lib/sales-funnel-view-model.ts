import type { FunnelAIAnalysis, FunnelStage, WidgetStatus } from "@/lib/dashboard-home-types";
import {
  formatConversionDeltaLabels,
  formatStageCountDelta,
  getFunnelStageCopy,
} from "@/lib/funnel-stage-copy";
import {
  buildFunnelMainConclusionDisplay,
  buildFunnelTransitions,
  markWeakestTransition,
  resolveHighlightedStageKey,
  type FunnelMainConclusionDisplay,
  type FunnelTransitionDisplay,
} from "@/lib/funnel-transitions-display";
import {
  buildSalesFunnelDisplay,
  formatFunnelPercent,
  type FunnelStageDisplay,
} from "@/lib/sales-funnel-display";

export type SalesFunnelViewModel = {
  stages: FunnelStageDisplay[];
  transitions: FunnelTransitionDisplay[];
  highlightedStageKey: string | null;
  highlightKind: "weak_spot" | null;
  weakTransition: FunnelTransitionDisplay | null;
  insightText: string | null;
  mainConclusion: FunnelMainConclusionDisplay | null;
  hasConversionDelta: boolean;
  hasVolumeDelta: boolean;
  comparisonMode: "default" | "day_over_day";
};

function cleanText(value?: string | null): string {
  return (value ?? "").trim();
}

function explanationUsesAbstractVolume(text: string): boolean {
  return /по\s+объ[её]му|объ[её]м\s+этапа|динамика\s+объ[её]ма|изменение\s+объ[её]ма/i.test(text);
}

/**
 * Builds the top insight banner from authoritative weak-transition metrics.
 * Returns null when there is no weak spot or insufficient metrics to describe it.
 */
export function buildFunnelInsightText(input: {
  weakTransition?: FunnelTransitionDisplay | null;
  funnelAnalysis?: FunnelAIAnalysis | null;
  comparisonMode?: "default" | "day_over_day";
}): string | null {
  const explanation = cleanText(input.funnelAnalysis?.weakest_stage?.explanation);
  if (explanation && explanation.length <= 220 && !explanationUsesAbstractVolume(explanation)) {
    return explanation;
  }

  const weak = input.weakTransition;
  if (!weak || weak.highlightKind !== "weak_spot") {
    return null;
  }

  const eventCopy = getFunnelStageCopy(weak.toStageKey);
  const parts: string[] = [`основная проблема находится на этапе «${weak.compactLabel}»`];

  if (weak.conversionPct != null) {
    let conversionPart = `конверсия составила ${formatFunnelPercent(weak.conversionPct)}`;
    const convDelta = formatConversionDeltaLabels({
      conversionDeltaPp: weak.conversionDeltaPp,
    });
    if (weak.conversionDeltaPp != null && convDelta.primary) {
      const direction =
        weak.conversionDeltaPp > 0 ? "выросла" : weak.conversionDeltaPp < 0 ? "снизилась" : "не изменилась";
      if (weak.conversionDeltaPp === 0) {
        conversionPart += ` и не изменилась`;
      } else {
        conversionPart += ` и ${direction} на ${convDelta.primary.replace(/^[+−]/, "")}`;
      }
    }
    parts.push(conversionPart);
  }

  if (weak.volumeDeltaPct != null) {
    const countDelta = formatStageCountDelta({
      stageKey: weak.toStageKey,
      deltaPercent: weak.volumeDeltaPct,
      periodType: input.comparisonMode === "day_over_day" ? "previous_day" : undefined,
    });
    // Prefer the natural sentence without repeating the subject twice.
    parts.push(
      countDelta.full.startsWith(eventCopy.changeSubject)
        ? `а ${countDelta.full.charAt(0).toLowerCase()}${countDelta.full.slice(1)}`
        : countDelta.full,
    );
  } else if (weak.conversionDeltaPp != null && weak.conversionDeltaLabel) {
    parts.push(`изменение конверсии ${weak.conversionDeltaLabel}`);
  }

  if (parts.length < 2) {
    return null;
  }

  if (parts.length === 2) {
    return `${parts[0]}: ${parts[1]}.`;
  }

  return `${parts[0]}: ${parts[1]}, ${parts.slice(2).join(", ")}.`;
}

export function mapSalesFunnelToViewModel(input: {
  status?: WidgetStatus | string;
  stages: FunnelStage[];
  funnelAnalysis?: FunnelAIAnalysis | null;
  primaryDropStage?: string;
  comparisonMode?: "default" | "day_over_day";
}): SalesFunnelViewModel {
  const { status, stages, funnelAnalysis, primaryDropStage, comparisonMode = "default" } = input;
  const displayStages = buildSalesFunnelDisplay(status, stages);
  const comparisonAnalysis = comparisonMode === "day_over_day" ? undefined : funnelAnalysis;
  const transitions = markWeakestTransition(
    buildFunnelTransitions(stages, comparisonAnalysis, { comparisonMode }),
    comparisonAnalysis,
    comparisonMode === "day_over_day" ? undefined : primaryDropStage,
    stages,
  );
  const highlighted = resolveHighlightedStageKey(transitions);
  const weakTransition = transitions.find((t) => t.highlightKind === "weak_spot") ?? null;
  let insightText = buildFunnelInsightText({
    weakTransition,
    funnelAnalysis: comparisonMode === "day_over_day" ? undefined : funnelAnalysis,
    comparisonMode,
  });
  if (
    insightText == null &&
    comparisonMode === "day_over_day" &&
    stages.some((stage) => stage.change_vs_previous_day_pct != null)
  ) {
    insightText = "По сравнению с предыдущим завершённым днём явного ухудшения воронки не выявлено.";
  }
  const conclusion = buildFunnelMainConclusionDisplay({
    funnelAnalysis,
    primaryDropStage,
  });
  const mainConclusion = conclusion.source === "empty" ? null : conclusion;

  return {
    stages: displayStages,
    transitions,
    highlightedStageKey: highlighted?.stageKey ?? null,
    highlightKind: highlighted?.kind ?? null,
    weakTransition,
    insightText,
    mainConclusion,
    hasConversionDelta: transitions.some((t) => t.conversionDeltaPp != null),
    hasVolumeDelta: transitions.some((t) => t.volumeDeltaPct != null) || displayStages.some((s) => s.previousDayChangeTone !== "unavailable"),
    comparisonMode,
  };
}
