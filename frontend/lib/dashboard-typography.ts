/**
 * Unified typography scale for the Dashboard page.
 * Use these constants instead of ad-hoc text sizes in dashboard components.
 */
export const dashboardTypography = {
  pageTitle: "text-2xl font-bold leading-tight tracking-tight text-slate-950 sm:text-3xl",
  pageDescription: "mt-1.5 max-w-2xl text-[15px] leading-relaxed text-slate-600 sm:text-base",

  moduleTitle: "text-xl font-bold leading-snug text-slate-950 lg:text-[22px]",
  moduleDescription: "max-w-3xl text-sm leading-relaxed text-slate-600 sm:text-[15px]",

  sectionTitle: "text-base font-bold leading-snug text-slate-800 lg:text-[17px]",
  sectionDescription: "text-sm leading-relaxed text-slate-600 sm:text-[15px]",

  cardTitle: "text-base font-semibold leading-snug text-slate-800 sm:text-lg",
  cardTitleCompact: "text-[15px] font-semibold leading-snug text-slate-800 sm:text-base",

  aiSummary: "text-[15px] font-medium leading-[1.6] text-slate-800 sm:text-base",
  aiSummaryLabel: "text-xs font-semibold uppercase tracking-wide text-slate-600 sm:text-[13px]",

  listItemTitle: "text-[15px] font-semibold leading-snug text-slate-800 sm:text-base",
  listItemBody: "text-sm leading-[1.6] text-slate-600 sm:text-[15px]",

  body: "text-sm leading-[1.6] text-slate-600 sm:text-[15px]",
  bodyEmphasis: "text-[15px] font-medium leading-snug text-slate-800 sm:text-base",
  secondary: "text-[13px] leading-relaxed text-slate-600 sm:text-sm",
  metadata: "text-xs leading-normal text-slate-500 sm:text-[13px]",

  kpiLabel: "text-xs font-semibold uppercase tracking-wide text-slate-600 sm:text-sm",
  kpiLabelPlain: "text-xs font-semibold text-slate-600 sm:text-sm",
  kpiValue: "text-3xl font-bold tabular-nums tracking-tight text-slate-950",
  kpiValueCompact: "text-2xl font-bold tabular-nums tracking-tight text-slate-950 sm:text-3xl",
  kpiDelta: "text-[13px] font-medium leading-snug tabular-nums sm:text-sm",

  badge: "text-[11px] font-semibold leading-none sm:text-xs",
  button: "text-sm font-semibold",
  ctaLink: "text-sm font-semibold text-blue-600 hover:text-blue-700",

  chartAxis: "fill-slate-500 text-[10px] sm:text-[11px]",
  chartRef: "fill-slate-500 text-[10px] sm:text-[11px]",
  chartTooltip: "text-xs leading-snug sm:text-[13px]",
  chartTooltipValue: "text-sm font-semibold tabular-nums text-slate-800",

  emptyState: "text-sm leading-relaxed text-slate-600",
  warningBanner: "text-sm leading-relaxed text-sky-950",
  footerNote: "text-xs leading-relaxed text-slate-500 sm:text-[13px]",
} as const;

export const dashboardTextColor = {
  primary: "text-slate-950",
  strongSecondary: "text-slate-800",
  body: "text-slate-600",
  bodyEmphasis: "text-slate-700",
  muted: "text-slate-500",
  veryMuted: "text-slate-400",
  positive: "text-emerald-700",
  negative: "text-red-600",
  warning: "text-amber-700",
} as const;
