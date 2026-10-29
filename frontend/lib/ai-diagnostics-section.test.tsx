import { renderToStaticMarkup } from "react-dom/server";
import { AIDiagnosticsSection } from "@/components/dashboard/ai-diagnostics-section";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function makeAIDiagnostics(): DashboardHomeResponse["ai_diagnostics"] {
  return {
    account_summary: {
      status: "available",
      severity: "high",
      executive_summary: "Нужно проверить рекламу.",
      narrative_source: "ai",
      freshness: "current",
      narrative_run_id: "run-1",
      prompt_version: "dashboard_narrative_v2",
      validation_status: "completed",
    },
    what_changed: {
      status: "available",
      summary: "Изменения относительно прошлой сводки.",
      groups: [
        { key: "worsened", label: "Ухудшилось", tone: "negative", summary: "Ухудшение.", items: [] },
        { key: "new_risks", label: "Новые риски", tone: "warning", summary: "Новые риски.", items: [] },
        { key: "unchanged_risks", label: "Сохраняются", tone: "neutral", summary: "Сохраняются.", items: [] },
        { key: "improved", label: "Улучшилось", tone: "positive", summary: "Без улучшений.", items: [] },
      ],
    },
    top_actions: [
      {
        id: "top-action-1",
        action_id: 76,
        action_key: "unified:pack:1:2026-:action_001",
        source: "recommendation",
        priority: "critical",
        priority_label: "Критический",
        category: "advertising",
        category_label: "Реклама",
        title: "Остановите неэффективную рекламную кампанию",
        reason: "Кампания «Поиск — бренд» потратила 1 250 ₽ за день, но не принесла заказов и выручки.",
        next_step: "Проверьте поисковые запросы, карточки товаров и площадки показа. Если причин продолжать рекламу нет — остановите кампанию или резко сократите бюджет.",
        expected_effect: "Снизить риск рекламного перерасхода.",
        supporting_metrics: [
          { label: "Расход", value: "1 250 ₽" },
          { label: "Заказы", value: "0" },
          { label: "Выручка", value: "0 ₽" },
        ],
      },
    ],
    category_summaries: [],
    sales_funnel: { status: "not_enough_data", current_completed_date: "", previous_completed_date: "", stages: [], daily_series: [] },
    funnel_analysis: {
      status: "not_enough_data",
      ai_analysis: { status: "not_enough_data", related_signals: [], recommended_focus: [] },
    },
    stock_coverage: { status: "not_enough_data", segments: [], total_skus: 0 },
    ad_efficiency: { status: "not_enough_data" },
    risk_distribution: { status: "not_enough_data", groups: [], total: 0 },
  };
}

function run() {
  const html = renderToStaticMarkup(<AIDiagnosticsSection data={makeAIDiagnostics()} />);

  assert(html.includes("lg:grid-cols-[minmax(0,34%)_minmax(0,1fr)]"), "account health + what-changed top row grid");
  assert(html.includes("ключевые риски и изменения за неделю"), "section subtitle updated");
  assert(html.includes("text-xl"), "section title uses module typography scale");
  assert(html.includes("lg:text-[22px]"), "section title has desktop module size");
  assert(html.includes("Что изменилось за неделю"), "what changed block heading");
  assert(html.includes("Приоритетное действие"), "top actions title rendered for single action");
  assert(
    html.includes(
      "Приоритетные шаги, которые помогут быстрее сократить потери и стабилизировать продажи.",
    ),
    "top actions subtitle rendered",
  );
  assert(html.includes("rounded-3xl"), "top actions module container rendered");
  assert(html.includes("grid grid-cols-1 gap-5"), "single top action uses one-column grid");
  assert(html.includes("Приоритет 1"), "priority badge rendered");
  assert(html.includes("Остановите неэффективную рекламную кампанию"), "title rendered");
  assert(html.includes("Критический"), "severity badge rendered");
  assert(html.includes("Реклама"), "category badge rendered");
  assert(html.includes("Почему это важно"), "reason section label rendered");
  assert(html.includes("Кампания «Поиск — бренд»"), "reason text rendered");
  assert(html.includes("Что сделать"), "next step heading rendered");
  assert(html.includes("Проверьте поисковые запросы"), "next step text rendered");
  assert(html.includes("Ожидаемый эффект"), "expected effect section rendered");
  assert(html.includes("Снизить риск рекламного перерасхода."), "advertising expected effect rendered");
  assert(html.includes("Доказательство"), "evidence section label rendered");
  assert(html.includes("Расход"), "supporting metrics label rendered");
  assert(html.includes("1 250 ₽"), "supporting metrics value rendered");
  assert(html.includes("Открыть действие"), "action detail CTA rendered");
  assert(!html.includes("/app/advertising"), "legacy advertising route not used");
  assert(!html.includes(">recommendation<"), "technical source hidden");
  assert(!html.includes("line-clamp-2"), "top action text is not line-clamped");
  for (const hiddenDebugField of ["run-1", "dashboard_narrative_v1", "completed"]) {
    assert(!html.includes(hiddenDebugField), `debug field hidden from seller UI: ${hiddenDebugField}`);
  }
  for (const forbidden of ["0.12x", "0,12x", "ad_waste", "campaign"]) {
    assert(!html.includes(forbidden), `forbidden token hidden: ${forbidden}`);
  }

  const noOptionalBlocksHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        top_actions: [
          {
            id: "top-action-2",
            source: "alert",
            priority: "high",
            priority_label: "Высокий",
            category: "other",
            category_label: "",
            title: "Проверьте приоритетное действие",
            reason: "Есть сигнал, который стоит проверить в ближайшее время.",
            next_step: "",
            supporting_metrics: [],
          },
        ],
      }}
    />,
  );
  assert(!noOptionalBlocksHtml.includes(">Что сделать<"), "next step block hidden when empty");
  assert(!noOptionalBlocksHtml.includes(">Доказательство<"), "evidence block hidden when no metrics");
  assert(noOptionalBlocksHtml.includes(">Почему это важно<"), "reason block shown when reason present");
  assert(!noOptionalBlocksHtml.includes(">Ожидаемый эффект<"), "expected effect hidden when empty");
  assert(
    !noOptionalBlocksHtml.includes(
      'rounded-full bg-aivion-surface-soft px-2.5 py-0.5 text-xs font-medium text-aivion-text ring-1 ring-inset ring-aivion-border">Другое</span>',
    ),
    "empty category badge hidden",
  );

  const emptyTopActionsHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        top_actions: [],
      }}
    />,
  );
  assert(emptyTopActionsHtml.includes("Приоритетных действий пока нет"), "empty state title rendered");
  assert(
    emptyTopActionsHtml.includes("Система не обнаружила критичных задач на сегодня."),
    "empty state message rendered",
  );
  assert(!emptyTopActionsHtml.includes("xl:grid-cols-3"), "empty state hides action grid");

  const mixedCategoriesHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        top_actions: [
          {
            id: "funnel-action",
            driver_id: "driver-funnel",
            source: "risk_driver",
            priority: "high",
            priority_label: "Высокий",
            category: "funnel",
            category_label: "Воронка",
            title: "Проверьте провал воронки продаж",
            reason: "Карточка → корзина просела.",
            next_step: "Проверьте этап с наибольшим падением.",
            supporting_metrics: [],
            linked_alert_ids: [],
            linked_recommendation_ids: [],
          },
          {
            id: "cancel-action",
            driver_id: "driver-cancellations",
            source: "risk_driver",
            priority: "critical",
            priority_label: "Критический",
            category: "cancellations",
            category_label: "Отмены",
            title: "Разберите рост отмен",
            reason: "Доля отмен выросла.",
            next_step: "Проверьте причины отмен.",
            supporting_metrics: [],
            linked_alert_ids: [],
            linked_recommendation_ids: [],
          },
          {
            id: "sales-action",
            driver_id: "driver-sales-revenue",
            source: "risk_driver",
            priority: "medium",
            priority_label: "Средний",
            category: "sales",
            category_label: "Продажи",
            title: "Разберите причину падения продаж",
            reason: "Выручка и заказы снизились.",
            next_step: "Проверьте SKU с просадкой.",
            supporting_metrics: [],
            linked_alert_ids: [],
            linked_recommendation_ids: [],
          },
        ],
      }}
    />,
  );
  for (const label of ["Воронка", "Отмены", "Продажи"]) {
    assert(mixedCategoriesHtml.includes(label), `category label rendered: ${label}`);
  }
  assert(mixedCategoriesHtml.includes("Открыть действия"), "fallback actions CTA rendered without action_key");
  assert(!mixedCategoriesHtml.includes("/app/products"), "sales action does not use retired products route");
  assert(!mixedCategoriesHtml.includes("Открыть товар"), "legacy product CTA removed");

  const fallbackTopActionsHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        account_summary: {
          ...makeAIDiagnostics().account_summary,
          narrative_source: "fallback",
          freshness: "fallback",
          validation_status: "fallback_stale",
        },
        top_actions: [
          {
            id: "fallback-action",
            source: "recommendation",
            priority: "high",
            priority_label: "Высокий",
            category: "advertising",
            category_label: "Реклама",
            title: "Fallback action",
            reason: "Fallback reason.",
            next_step: "Fallback step.",
            expected_effect: "Fallback effect.",
          },
        ],
      }}
    />,
  );
  assert(
    !fallbackTopActionsHtml.includes("AI-действия обновляются"),
    "pending stub must not appear when fallback top actions exist",
  );
  assert(
    fallbackTopActionsHtml.includes("Fallback action"),
    "fallback top actions rendered even when narrative_source=fallback",
  );
  assert(
    fallbackTopActionsHtml.includes("Fallback effect."),
    "fallback expected_effect rendered from API",
  );
  assert(!fallbackTopActionsHtml.includes(">recommendation<"), "technical source hidden in fallback cards");

  const healthBlockHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        account_summary: {
          status: "available",
          health_score: 41,
          severity: "high",
          improvement_summary: "Индекс вырастет после снижения 6 артикулов с коротким покрытием.",
          risk_drivers: [
            {
              id: "driver-cancellations",
              category: "cancellations",
              category_label: "Отмены",
              severity: "high",
              impact_score: 80,
              title: "Рост отмен",
              reason:
                "На 21.05.2026 отмены составили 13,6% от заказов: 12 отмен при 76 заказах.",
              linked_alert_ids: [],
              linked_recommendation_ids: [],
            },
          ],
        },
        what_changed: {
          status: "available",
          summary: "Сводка изменений.",
          groups: [
            {
              key: "worsened",
              label: "Ухудшилось",
              tone: "negative",
              summary: "Ухудшение.",
              items: [
                {
                  label: "Отмены",
                  text: "На 21.05.2026 отмены составили 13,6% от заказов: 12 отмен при 76 заказах.",
                  highlights: [],
                },
              ],
            },
            { key: "new_risks", label: "Новые риски", tone: "warning", summary: "Новые риски.", items: [] },
            { key: "unchanged_risks", label: "Активные проблемы", tone: "neutral", summary: "Активные.", items: [] },
            { key: "improved", label: "Улучшилось", tone: "positive", summary: "Без улучшений.", items: [] },
          ],
        },
      }}
    />,
  );
  assert(healthBlockHtml.includes("Основные причины снижения"), "health drivers section rendered");
  assert(healthBlockHtml.includes("Рост отмен"), "cancellation driver title visible");
  assert(healthBlockHtml.includes("отмен"), "cancellation driver reason visible");
  assert(healthBlockHtml.includes("<strong") || healthBlockHtml.includes("<span"), "metric highlights rendered in health block");
  assert(!healthBlockHtml.includes("артикул"), "articul wording hidden from seller-facing health block");
  assert(healthBlockHtml.includes("6 SKU"), "articul normalized to SKU in improvement summary");
  for (const forbiddenClamp of ["line-clamp-2", "line-clamp-3"]) {
    const healthCardMatch = healthBlockHtml.match(
      /Основные причины снижения[\s\S]{0,1200}/u,
    );
    assert(
      healthCardMatch != null && !healthCardMatch[0].includes(forbiddenClamp),
      `health drivers not clamped: ${forbiddenClamp}`,
    );
  }

  assert(!html.includes("Кампании: расход vs ДРР"), "old scatter title removed");
  assert(html.includes("Реклама: эффективность кампаний"), "new ad efficiency module title rendered");
  assert(!html.includes("data-testid=\"ad-efficiency-kpi-high-drr\""), "not_enough_data ad widget hides KPI zeros");
  assert(!html.includes("Распределение рисков"), "risk distribution block removed from dashboard");

  const legacyRiskDistributionHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        risk_distribution: {
          status: "available",
          groups: [
            { group: "stock", label: "Остатки", count: 3, severity: "high" },
            { group: "advertising", label: "Реклама", count: 2, severity: "medium" },
          ],
          total: 5,
        },
      }}
    />,
  );
  assert(
    !legacyRiskDistributionHtml.includes("Распределение рисков"),
    "legacy risk_distribution payload does not render removed block",
  );
  assert(
    legacyRiskDistributionHtml.includes("Индекс состояния кабинета"),
    "health score block still renders with legacy payload",
  );

  const categoryCardsHtml = renderToStaticMarkup(
    <AIDiagnosticsSection
      data={{
        ...makeAIDiagnostics(),
        category_summaries: [
          {
            category_id: "cat-1",
            category_name: "Текстиль для дома",
            category_score: 6,
            severity: "medium",
            headline: "Категория держится на ограниченном числе SKU с устойчивым спросом.",
            key_risk: "Риск просадки по ключевым SKU; зависимость от сезонного спроса",
            recommended_action: "Проверьте остатки. Затем обновите цены.",
          },
          {
            category_id: "cat-2",
            category_name: "Кухня и хранение",
            category_score: 7,
            severity: "high",
            headline: "Категория требует внимания.",
            key_risk: "Просадка по ключевым SKU",
            recommended_action: "Проверьте остатки.",
          },
          {
            category_id: "cat-3",
            category_name: "",
            category_score: null,
            severity: "",
            headline: "",
            key_risk: "",
            recommended_action: "",
          },
        ],
      }}
    />,
  );

  assert(categoryCardsHtml.includes("Карточки состояния категорий"), "category cards title rendered");
  assert(
    categoryCardsHtml.includes("AI-сводка по категориям, ключевые риски и действия для улучшения состояния."),
    "category cards subtitle rendered",
  );
  assert(categoryCardsHtml.includes(">6<"), "category score 58 rendered as 6");
  assert(categoryCardsHtml.includes("> / 10<"), "category score suffix rendered");
  assert(!categoryCardsHtml.includes("/ 100"), "category cards do not show / 100");
  assert(
    categoryCardsHtml.includes('aria-label="Оценка состояния категории: 6 из 10"'),
    "category score aria-label rendered",
  );
  assert(categoryCardsHtml.includes("Средний риск"), "score 58 risk badge rendered");
  assert(categoryCardsHtml.includes(">7<"), "category score 68 rendered as 7");
  assert(categoryCardsHtml.includes("Небольшой риск"), "score 68 risk badge rendered despite legacy high severity");
  assert(categoryCardsHtml.includes("Состояние"), "headline section rendered");
  assert(categoryCardsHtml.includes("Риски"), "risk section rendered");
  assert(categoryCardsHtml.includes("Что сделать"), "action section rendered");
  assert(!categoryCardsHtml.includes("Открыть категорию"), "legacy category CTA removed");
  assert(!categoryCardsHtml.includes("/app/products"), "category cards do not link to retired products routes");
  assert(categoryCardsHtml.includes("—"), "empty score placeholder rendered");
  assert(categoryCardsHtml.includes("Без оценки"), "empty severity badge rendered");
  assert(categoryCardsHtml.includes("Категория без названия"), "empty category name fallback rendered");
  assert(categoryCardsHtml.includes("Недостаточно данных для краткого вывода."), "empty headline fallback rendered");
  assert(categoryCardsHtml.includes("Явные риски не выделены."), "empty risk fallback rendered");
  assert(categoryCardsHtml.includes("Действия не сформированы."), "empty action fallback rendered");
  assert(categoryCardsHtml.includes("xl:grid-cols-3"), "category cards desktop grid rendered");
  assert(!categoryCardsHtml.includes("lg:grid-cols-4"), "old 4-column category grid removed");

  console.log("ai-diagnostics-section.test.tsx: ok");
}

run();

