export type FunnelStageCanonicalId =
  | "impressions"
  | "product_views"
  | "add_to_cart"
  | "orders"
  | "delivered_units";

export type FunnelStageCopy = {
  title: string;
  eventName: string;
  changeSubject: string;
  /** Genitive/plural for compact badges, e.g. «открытий карточек». */
  compactChangeLabel: string;
  /** Ultra-short for narrow funnel badges, e.g. «открытий». */
  funnelBadgeLabel: string;
  /** Short conversion noun for transition cards, e.g. «в карточку». */
  conversionIntoLabel: string;
};

export const FUNNEL_STAGE_COPY: Record<FunnelStageCanonicalId, FunnelStageCopy> = {
  impressions: {
    title: "Увидели товар",
    eventName: "показы",
    changeSubject: "Показов",
    compactChangeLabel: "показов",
    funnelBadgeLabel: "показов",
    conversionIntoLabel: "в показы",
  },
  product_views: {
    title: "Открыли карточку",
    eventName: "открытия карточек",
    changeSubject: "Открытий карточек",
    compactChangeLabel: "открытий карточек",
    funnelBadgeLabel: "открытий",
    conversionIntoLabel: "в карточку",
  },
  add_to_cart: {
    title: "Добавили в корзину",
    eventName: "добавления в корзину",
    changeSubject: "Добавлений в корзину",
    compactChangeLabel: "добавлений в корзину",
    funnelBadgeLabel: "добавлений",
    conversionIntoLabel: "в корзину",
  },
  orders: {
    title: "Заказанные единицы",
    eventName: "заказанные единицы",
    changeSubject: "Заказанных единиц",
    compactChangeLabel: "заказанных единиц",
    funnelBadgeLabel: "заказанных единиц",
    conversionIntoLabel: "в заказ",
  },
  delivered_units: {
    title: "Доставленные единицы",
    eventName: "доставленные единицы",
    changeSubject: "Доставленных единиц",
    compactChangeLabel: "доставленных единиц",
    funnelBadgeLabel: "доставленных единиц",
    conversionIntoLabel: "в доставку",
  },
};

const LEGACY_STAGE_KEY_ALIASES: Record<string, FunnelStageCanonicalId> = {
  hits_view: "impressions",
  hits_view_pdp: "product_views",
  hits_tocart: "add_to_cart",
  ordered_units: "orders",
  impressions: "impressions",
  product_views: "product_views",
  add_to_cart: "add_to_cart",
  orders: "orders",
  delivered_units: "delivered_units",
};

function isFiniteNumber(value: number | null | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export function formatPercentMagnitude(value: number): string {
  return new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 1,
  }).format(Math.abs(value));
}

export function formatSignedPercent(value: number): string {
  if (value === 0) return "0,0%";
  const sign = value > 0 ? "+" : "−";
  return `${sign}${formatPercentMagnitude(value)}%`;
}

export function formatSignedPp(value: number): string {
  const sign = value > 0 ? "+" : value < 0 ? "−" : "";
  const formatted = new Intl.NumberFormat("ru-RU", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  }).format(Math.abs(value));
  return `${sign}${formatted} п.п.`;
}

export function resolveFunnelStageCanonicalId(stageKey: string): FunnelStageCanonicalId | null {
  const key = stageKey.trim().toLowerCase();
  return LEGACY_STAGE_KEY_ALIASES[key] ?? null;
}

export function getFunnelStageCopy(stageKey: string): FunnelStageCopy {
  const canonical = resolveFunnelStageCanonicalId(stageKey);
  if (canonical) return FUNNEL_STAGE_COPY[canonical];
  const humanized = stageKey.trim().replace(/_/g, " ") || "событий этапа";
  const lower = humanized.toLowerCase();
  return {
    title: humanized.charAt(0).toUpperCase() + humanized.slice(1),
    eventName: lower,
    changeSubject: humanized.charAt(0).toUpperCase() + humanized.slice(1),
    compactChangeLabel: lower,
    funnelBadgeLabel: lower,
    conversionIntoLabel: `в ${lower}`,
  };
}

export type FunnelComparisonPeriodKind = "previous_day" | "previous_week" | "previous_period";

/**
 * Resolves user-facing comparison period labels from API period_type / explicit label.
 * Does not hardcode «к прошлой неделе» unless the period is weekly.
 */
export function resolveFunnelComparisonPeriod(input?: {
  periodType?: string | null;
  comparisonLabel?: string | null;
}): {
  kind: FunnelComparisonPeriodKind;
  /** Prefixed form: «к прошлой неделе» / «к предыдущему периоду». */
  compactSuffix: string;
  /** Clause form: «чем неделю назад» / «чем в предыдущем периоде». */
  fullClause: string;
  /** Neutral phrase for titles. */
  relativeLabel: string;
} {
  const explicit = (input?.comparisonLabel ?? "").trim();
  if (explicit) {
    const compactSuffix = explicit.startsWith("к ") || explicit.startsWith("по ") ? explicit : `к ${explicit}`;
    let fullClause: string;
    if (explicit.startsWith("чем ")) {
      fullClause = explicit;
    } else if (explicit.startsWith("к ")) {
      fullClause = `чем ${explicit.slice(2)}`;
    } else if (explicit.startsWith("по сравнению с ")) {
      fullClause = explicit.replace(/^по сравнению с /, "чем ");
    } else {
      fullClause = `чем ${explicit}`;
    }
    return {
      kind: /предыдущему\s+дню|вчера/i.test(explicit)
        ? "previous_day"
        : /недел/i.test(explicit)
          ? "previous_week"
          : "previous_period",
      compactSuffix,
      fullClause,
      relativeLabel: explicit.replace(/^к /, "").replace(/^по сравнению с /, ""),
    };
  }

  const periodType = (input?.periodType ?? "").trim().toLowerCase();
  if (
    periodType === "previous_day" ||
    periodType === "day_over_day" ||
    periodType === "daily" ||
    periodType === "completed_day"
  ) {
    return {
      kind: "previous_day",
      compactSuffix: "к предыдущему дню",
      fullClause: "чем днём ранее",
      relativeLabel: "предыдущему дню",
    };
  }
  const isWeekly =
    periodType === "current_7d" ||
    periodType === "previous_7d" ||
    periodType === "week" ||
    periodType === "weekly" ||
    periodType.includes("7d") ||
    periodType.includes("week");

  // Default for this dashboard: weekly seller summaries (period_type current_7d on stages).
  // When period_type is absent, keep weekly wording that matches current facts pipeline.
  if (!periodType || isWeekly) {
    return {
      kind: "previous_week",
      compactSuffix: "к прошлой неделе",
      fullClause: "чем неделю назад",
      relativeLabel: "прошлой неделе",
    };
  }

  return {
    kind: "previous_period",
    compactSuffix: "к предыдущему периоду",
    fullClause: "чем в предыдущем периоде",
    relativeLabel: "предыдущему периоду",
  };
}

export type StageCountDeltaLabels = {
  /** ±N% or «Без изменений» / «Нет данных для сравнения». */
  primary: string;
  /** Event + period line for badges/cards; null when unavailable. */
  secondary: string | null;
  /** Full seller-facing sentence. */
  full: string;
  /** Accessible / title text. */
  title: string;
};

export function formatStageCountDelta(params: {
  stageKey: string;
  deltaPercent: number | null | undefined;
  periodType?: string | null;
  comparisonLabel?: string | null;
}): StageCountDeltaLabels {
  const copy = getFunnelStageCopy(params.stageKey);
  const period = resolveFunnelComparisonPeriod({
    periodType: params.periodType,
    comparisonLabel: params.comparisonLabel,
  });

  if (!isFiniteNumber(params.deltaPercent)) {
    return {
      primary: "Нет данных для сравнения",
      secondary: null,
      full: "Нет данных для сравнения",
      title: `Нет данных для сравнения количества (${copy.eventName})`,
    };
  }

  if (params.deltaPercent === 0) {
    return {
      primary: "Без изменений",
      secondary: period.compactSuffix,
      full: `Количество ${copy.eventName} не изменилось относительно ${period.relativeLabel}`,
      title: `Количество ${copy.eventName} не изменилось (${period.relativeLabel})`,
    };
  }

  const absPct = formatPercentMagnitude(params.deltaPercent) + "%";
  const direction = params.deltaPercent > 0 ? "больше" : "меньше";
  const signed = formatSignedPercent(params.deltaPercent);

  return {
    primary: signed,
    secondary: `${copy.compactChangeLabel} ${period.compactSuffix}`,
    full: `${copy.changeSubject} стало на ${absPct} ${direction}, ${period.fullClause}`,
    title: `${copy.changeSubject} стало на ${absPct} ${direction}, ${period.fullClause}`,
  };
}

/** Compact funnel-segment badge: shorter event noun when space is tight. */
export function formatCompactStageCountDelta(params: {
  stageKey: string;
  deltaPercent: number | null | undefined;
  periodType?: string | null;
  comparisonLabel?: string | null;
}): StageCountDeltaLabels {
  const base = formatStageCountDelta(params);
  if (base.secondary == null || !isFiniteNumber(params.deltaPercent) || params.deltaPercent === 0) {
    return base;
  }
  const copy = getFunnelStageCopy(params.stageKey);
  const period = resolveFunnelComparisonPeriod({
    periodType: params.periodType,
    comparisonLabel: params.comparisonLabel,
  });
  return {
    ...base,
    secondary: `${copy.funnelBadgeLabel} ${period.compactSuffix}`,
  };
}

export function formatConversionDeltaLabels(params: {
  conversionDeltaPp: number | null | undefined;
  periodType?: string | null;
  comparisonLabel?: string | null;
}): { primary: string | null; secondary: string | null; full: string | null } {
  if (!isFiniteNumber(params.conversionDeltaPp)) {
    return { primary: null, secondary: null, full: null };
  }
  const formatted = formatSignedPp(params.conversionDeltaPp);
  const period = resolveFunnelComparisonPeriod({
    periodType: params.periodType,
    comparisonLabel: params.comparisonLabel,
  });
  const value = params.conversionDeltaPp;
  const direction = value > 0 ? "выросла" : value < 0 ? "снизилась" : "не изменилась";
  const abs = formatted.replace(/^[+−]/, "");
  return {
    primary: formatted,
    secondary: `Конверсия ${period.compactSuffix}`,
    full:
      value === 0
        ? `Конверсия не изменилась относительно ${period.relativeLabel}`
        : `Конверсия ${direction} на ${abs} ${period.fullClause}`,
  };
}

export function formatConversionCurrentCaption(toStageKey: string): string {
  const copy = getFunnelStageCopy(toStageKey);
  return `Конверсия ${copy.conversionIntoLabel}`;
}
