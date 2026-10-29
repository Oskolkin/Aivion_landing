"use client";

import { useState } from "react";
import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  ClipboardCheck,
} from "lucide-react";
import { cn } from "@/components/ui/cn";
import { asArray } from "@/lib/as-array";
import type { CategorySummaryCard, NullableArray } from "@/lib/dashboard-home-types";
import {
  formatCategoryConditionScore,
  getCategoryConditionRiskBadge,
  getCategoryCTA,
  getCategoryConditionScoreColor,
  getCategoryDisplayName,
  getCategorySectionText,
  getCategoryVisual,
} from "@/lib/category-status-display";
import { dashboardTypography } from "@/lib/dashboard-typography";

const SECTION_SUBTITLE =
  "AI-сводка по категориям, ключевые риски и действия для улучшения состояния.";

type Props = {
  categories: NullableArray<CategorySummaryCard>;
};

export function CategoryStatusCardsSection({ categories: categoriesProp }: Props) {
  const categorySummaries = asArray(categoriesProp);
  const [showAllCategories, setShowAllCategories] = useState(false);

  if (categorySummaries.length === 0) {
    return null;
  }

  const categories = showAllCategories ? categorySummaries : categorySummaries.slice(0, 8);

  return (
    <section className="rounded-[22px] border border-[#E2E8F0] bg-white p-5 shadow-[0_8px_24px_rgba(15,23,42,0.04)] sm:p-6">
      <header className="mb-5">
        <h2 className={dashboardTypography.moduleTitle}>Карточки состояния категорий</h2>
        <p className={cn("mt-1", dashboardTypography.moduleDescription)}>{SECTION_SUBTITLE}</p>
      </header>

      <div className="grid grid-cols-1 items-stretch gap-5 md:grid-cols-2 xl:grid-cols-3">
        {categories.map((category) => (
          <CategoryStatusCard key={category.category_id ?? category.category_name} category={category} />
        ))}
      </div>

      {categorySummaries.length > 8 ? (
        <button
          type="button"
          className={cn("mt-5", dashboardTypography.ctaLink)}
          onClick={() => setShowAllCategories((value) => !value)}
        >
          {showAllCategories ? "Свернуть категории" : "Показать все категории"}
        </button>
      ) : null}
    </section>
  );
}

function CategoryStatusCard({ category }: { category: CategorySummaryCard }) {
  const displayName = getCategoryDisplayName(category.category_name);
  const visual = getCategoryVisual(displayName);
  const conditionScore = formatCategoryConditionScore(
    category.health_score,
    category.severity,
    category.category_score,
  );
  const scoreColor = getCategoryConditionScoreColor(conditionScore.tone);
  const riskBadge = getCategoryConditionRiskBadge(
    category.health_score,
    category.severity,
    category.category_score,
  );
  const headline = getCategorySectionText("headline", category.headline, {
    categoryName: displayName,
  });
  const risks = getCategorySectionText("key_risk", category.key_risk);
  const actions = getCategorySectionText("recommended_action", category.recommended_action);

  return (
    <article
      className={cn(
        "flex h-full min-h-0 flex-col rounded-[17px] border border-[#E2E8F0] bg-white p-4 shadow-[0_4px_14px_rgba(15,23,42,0.04)] transition duration-150",
        "hover:-translate-y-px hover:border-[#CBD5E1] hover:shadow-[0_10px_24px_rgba(15,23,42,0.07)]",
      )}
    >
      <header className="mb-3.5 flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <CategoryIconBadge icon={visual.icon} iconClassName={visual.iconClassName} badgeClassName={visual.badgeClassName} />
          <h3 className={cn("whitespace-normal break-words", dashboardTypography.listItemTitle)}>{displayName}</h3>
        </div>

        <div className="flex shrink-0 flex-col items-end gap-1">
          <div
            className={cn("tabular-nums leading-none", scoreColor)}
            aria-label={conditionScore.ariaLabel}
          >
            <span className={cn(dashboardTypography.kpiValueCompact, "text-lg")}>{conditionScore.value ?? "—"}</span>
            <span className={dashboardTypography.metadata}> / 10</span>
          </div>
          <CategoryRiskBadge label={riskBadge.label} className={riskBadge.className} />
        </div>
      </header>

      <div className="flex flex-1 flex-col gap-4">
        <CategoryCardSection
          icon={Activity}
          title="Состояние"
          iconClassName="text-[#2563EB]"
          items={headline.items}
          isFallback={headline.isFallback}
          itemClampClass=""
        />

        <CategorySectionDivider />

        <CategoryCardSection
          icon={AlertTriangle}
          title="Риски"
          iconClassName="text-[#EA580C]"
          items={risks.items}
          isFallback={risks.isFallback}
          itemClampClass=""
          asList
        />

        <CategorySectionDivider />

        <CategoryCardSection
          icon={ClipboardCheck}
          title="Что сделать"
          iconClassName="text-[#16A34A]"
          items={actions.items}
          isFallback={actions.isFallback}
          itemClampClass=""
          asList
        />
      </div>

      <CategoryCardCTA category={category} />
    </article>
  );
}

function CategoryIconBadge({
  icon: Icon,
  iconClassName,
  badgeClassName,
}: {
  icon: LucideIcon;
  iconClassName: string;
  badgeClassName: string;
}) {
  return (
    <div
      className={cn(
        "flex h-10 w-10 shrink-0 items-center justify-center rounded-[11px]",
        badgeClassName,
      )}
      aria-hidden="true"
    >
      <Icon className={cn("h-5 w-5", iconClassName)} />
    </div>
  );
}

function CategoryRiskBadge({ label, className }: { label: string; className: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center whitespace-nowrap rounded-full border px-2 py-0.5",
        dashboardTypography.badge,
        className,
      )}
    >
      {label}
    </span>
  );
}

function CategorySectionDivider() {
  return <div className="my-3 h-px bg-[#F1F5F9]" aria-hidden="true" />;
}

type CategoryCardSectionProps = {
  icon: LucideIcon;
  title: string;
  iconClassName: string;
  items: string[];
  isFallback: boolean;
  itemClampClass: string;
  asList?: boolean;
};

function CategoryCardSection({
  icon: Icon,
  title,
  iconClassName,
  items,
  isFallback,
  itemClampClass,
  asList = false,
}: CategoryCardSectionProps) {
  const showList = asList && !isFallback && items.length > 1;

  return (
    <section>
      <div className="mb-2 flex items-center gap-1.5">
        <Icon className={cn("h-[15px] w-[15px] shrink-0", iconClassName)} aria-hidden="true" />
        <h4 className={dashboardTypography.sectionTitle}>{title}</h4>
      </div>

      {showList ? (
        <ul className="space-y-2 pl-6">
          {items.map((item, index) => (
            <li
              key={`${title}-${index}`}
              className={cn("list-disc marker:text-slate-300", dashboardTypography.body)}
            >
              {item}
            </li>
          ))}
        </ul>
      ) : (
        <p
          className={cn(
            dashboardTypography.body,
            isFallback ? "text-slate-500" : "text-slate-600",
            itemClampClass,
          )}
        >
          {items[0]}
        </p>
      )}
    </section>
  );
}

function CategoryCardCTA({ category }: { category: CategorySummaryCard }) {
  const displayName = getCategoryDisplayName(category.category_name);
  const cta = getCategoryCTA(category);
  if (!cta) {
    return null;
  }

  return (
    <Link
      href={cta.href}
      className={cn("mt-auto inline-flex items-center gap-1 pt-4", dashboardTypography.ctaLink)}
      aria-label={`Открыть категорию ${displayName}`}
    >
      Открыть категорию
      <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
    </Link>
  );
}
