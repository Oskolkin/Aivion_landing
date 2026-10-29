"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertCircle,
  ArrowRight,
  BarChart3,
  CalendarDays,
  CircleDollarSign,
  ClipboardCheck,
  Filter,
  LineChart,
  ListChecks,
  Megaphone,
  Package,
  ShoppingCart,
  Star,
  Target,
  TrendingUp,
} from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/components/ui/cn";
import { asArray } from "@/lib/as-array";
import type { NullableArray, TopActionItem } from "@/lib/dashboard-home-types";
import {
  getActionVisualTone,
  getEvidenceGridClass,
  getTopActionCTA,
  getTopActionMetricKey,
  normalizeMetricLabel,
  type ActionVisualTone,
} from "@/lib/top-actions-display";
import { dashboardTypography } from "@/lib/dashboard-typography";

type Props = {
  actions: NullableArray<TopActionItem>;
};

const BLOCK_SUBTITLE =
  "Приоритетные шаги, которые помогут быстрее сократить потери и стабилизировать продажи.";

export function TopActionsSection({ actions: actionsProp }: Props) {
  const actions = asArray(actionsProp);

  return (
    <section className="rounded-3xl border border-aivion-border bg-white p-4 shadow-sm sm:p-6">
      <div className="mb-6 flex items-start gap-4">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-red-100 bg-red-50 text-red-500">
          <Target className="h-6 w-6" aria-hidden="true" />
        </div>
        <div>
          <h2 className={dashboardTypography.moduleTitle}>
            {actions.length === 1
              ? "Приоритетное действие"
              : actions.length === 3
                ? "Топ 3 действия"
                : "Приоритетные действия"}
          </h2>
          <p className={cn("mt-1", dashboardTypography.moduleDescription)}>{BLOCK_SUBTITLE}</p>
        </div>
      </div>

      {actions.length === 0 ? (
        <EmptyState
          icon={<ListChecks className="h-5 w-5" aria-hidden="true" />}
          title="Приоритетных действий пока нет"
          message="Система не обнаружила критичных задач на сегодня. Продолжайте следить за продажами, остатками и рекламой."
          className="border-solid bg-aivion-surface-soft"
        />
      ) : (
        <div
          className={
            actions.length <= 1
              ? "grid grid-cols-1 gap-5"
              : actions.length === 2
                ? "grid grid-cols-1 gap-5 md:grid-cols-2"
                : "grid grid-cols-1 gap-5 xl:grid-cols-3"
          }
        >
          {actions.map((action, index) => (
            <TopActionCard key={action.id} action={action} priorityIndex={index + 1} />
          ))}
        </div>
      )}
    </section>
  );
}

function SectionDivider() {
  return <div className="my-5 border-t border-dashed border-aivion-border" aria-hidden="true" />;
}

type ActionSectionProps = {
  icon: LucideIcon;
  title: string;
  tone: ActionVisualTone;
  children: string;
};

function ActionSection({ icon: Icon, title, tone, children }: ActionSectionProps) {
  return (
    <div className="flex gap-4">
      <div
        className={cn(
          "flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border",
          tone.softBg,
          tone.softBorder,
          tone.text,
        )}
      >
        <Icon className="h-5 w-5" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className={cn(dashboardTypography.sectionTitle)}>{title}</div>
        <p className={cn("mt-1 whitespace-normal break-words", dashboardTypography.listItemBody)}>{children}</p>
      </div>
    </div>
  );
}

function CategoryIconGraphic({ action, className }: { action: TopActionItem; className?: string }) {
  const text = `${action.category ?? ""} ${action.category_label ?? ""} ${action.title ?? ""}`.toLowerCase();
  const rawCategory = (action.category ?? "").trim().toLowerCase();

  if (
    text.includes("реклам") ||
    text.includes("ads") ||
    text.includes("advertising") ||
    rawCategory === "advertising"
  ) {
    return <Megaphone className={className} aria-hidden="true" />;
  }

  if (
    text.includes("остат") ||
    text.includes("stock") ||
    text.includes("inventory") ||
    rawCategory === "stock"
  ) {
    return <Package className={className} aria-hidden="true" />;
  }

  if (
    text.includes("ворон") ||
    text.includes("funnel") ||
    text.includes("conversion") ||
    rawCategory === "funnel"
  ) {
    return <Filter className={className} aria-hidden="true" />;
  }

  if (
    text.includes("sku") ||
    text.includes("продаж") ||
    text.includes("sales") ||
    rawCategory === "sales" ||
    rawCategory === "products"
  ) {
    return <BarChart3 className={className} aria-hidden="true" />;
  }

  return <ShoppingCart className={className} aria-hidden="true" />;
}

function MetricEvidenceIcon({ label, className }: { label: string; className?: string }) {
  const normalized = normalizeMetricLabel(label);

  if (normalized.includes("расход") || normalized.includes("spend") || normalized.includes("cost")) {
    return <CircleDollarSign className={className} aria-hidden="true" />;
  }
  if (normalized.includes("выруч") || normalized.includes("revenue")) {
    return <LineChart className={className} aria-hidden="true" />;
  }
  if (normalized.includes("заказ") || normalized.includes("order")) {
    return <ShoppingCart className={className} aria-hidden="true" />;
  }
  if (normalized.includes("остат") || normalized.includes("stock")) {
    return <Package className={className} aria-hidden="true" />;
  }
  if (normalized.includes("покрыт") || normalized.includes("coverage") || normalized.includes("days")) {
    return <CalendarDays className={className} aria-hidden="true" />;
  }
  if (
    normalized.includes("конвер") ||
    normalized.includes("conversion") ||
    normalized.includes("ctr") ||
    normalized.includes("cr")
  ) {
    return <Activity className={className} aria-hidden="true" />;
  }

  return <BarChart3 className={className} aria-hidden="true" />;
}

type TopActionCardProps = {
  action: TopActionItem;
  priorityIndex: number;
};

function TopActionCard({ action, priorityIndex }: TopActionCardProps) {
  const metrics = asArray(action.supporting_metrics);
  const categoryBadge = (action.category_label ?? "").trim();
  const title = (action.title ?? "").trim();
  const reason = (action.reason ?? "").trim();
  const nextStep = (action.next_step ?? "").trim();
  const expectedEffect = (action.expected_effect ?? "").trim();
  const cta = getTopActionCTA(action);
  const tone = getActionVisualTone(action);

  const sections: ReactNode[] = [];

  if (reason) {
    sections.push(
      <ActionSection key="reason" icon={AlertCircle} title="Почему это важно" tone={tone}>
        {reason}
      </ActionSection>,
    );
  }

  if (nextStep) {
    sections.push(
      <ActionSection key="next-step" icon={ClipboardCheck} title="Что сделать" tone={tone}>
        {nextStep}
      </ActionSection>,
    );
  }

  if (expectedEffect) {
    sections.push(
      <ActionSection key="expected-effect" icon={TrendingUp} title="Ожидаемый эффект" tone={tone}>
        {expectedEffect}
      </ActionSection>,
    );
  }

  return (
    <article
      className={cn(
        "relative flex min-h-[420px] flex-col rounded-2xl border border-aivion-border bg-white p-5 shadow-sm transition-shadow hover:shadow-md sm:min-h-0 sm:p-6",
        tone.border,
      )}
    >
      <div className="mb-6 flex flex-wrap items-center gap-2">
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5",
            dashboardTypography.badge,
            tone.priorityBadge,
          )}
        >
          <Star className="h-3.5 w-3.5 fill-current" aria-hidden="true" />
          Приоритет {priorityIndex}
          {action.priority_label ? ` · ${action.priority_label}` : null}
        </span>
        {categoryBadge ? (
          <span
            className={cn(
              "inline-flex max-w-full items-center gap-1.5 rounded-full border px-2.5 py-0.5",
              dashboardTypography.badge,
              tone.badge,
            )}
          >
            <CategoryIconGraphic action={action} className="h-3.5 w-3.5 shrink-0" />
            {categoryBadge}
          </span>
        ) : null}
      </div>

      {title ? (
        <h3 className={cn("mb-7", dashboardTypography.listItemTitle, "text-lg sm:text-xl")}>{title}</h3>
      ) : null}

      <div className="flex-1">
        {sections.map((section, index) => (
          <div key={index}>
            {index > 0 ? <SectionDivider /> : null}
            {section}
          </div>
        ))}
      </div>

      {(metrics.length > 0 || cta) && (
        <div className={cn("mt-auto", metrics.length > 0 ? "pt-5" : "pt-4")}>
          {metrics.length > 0 ? (
            <>
              <SectionDivider />
              <div>
                <div className={dashboardTypography.sectionTitle}>Доказательство</div>
                <div className={cn("mt-3 grid gap-3", getEvidenceGridClass(metrics.length))}>
                  {metrics.map((metric, index) => (
                    <div
                      key={getTopActionMetricKey(action, metric, index)}
                      className={cn("rounded-xl border p-3", tone.softBg, tone.softBorder)}
                    >
                      <div className="flex items-center gap-2">
                        <MetricEvidenceIcon label={metric.label} className={cn("h-5 w-5 shrink-0", tone.text)} />
                        <div className="min-w-0">
                          <div className={cn(dashboardTypography.kpiValueCompact, "text-base sm:text-lg")}>{metric.value}</div>
                          <div className={dashboardTypography.kpiLabelPlain}>{metric.label}</div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          ) : null}

          {cta ? (
            <Link
              href={cta.href}
              className={cn("mt-5 inline-flex items-center gap-1", dashboardTypography.ctaLink)}
            >
              {cta.label}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Link>
          ) : null}
        </div>
      )}
    </article>
  );
}
