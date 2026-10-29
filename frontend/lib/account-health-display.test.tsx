import { renderToStaticMarkup } from "react-dom/server";
import { AccountHealthCard } from "@/components/dashboard/account-health-card";
import {
  buildAccountHealthDisplayView,
  buildHealthDriverViews,
  buildHealthImprovementSummary,
  formatHealthScore,
  getHealthScorePercent,
  getHealthSeverityLabel,
} from "@/lib/account-health-display";
import type { AccountHealthDisplayInput } from "@/lib/account-health-display";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function makeInput(overrides: Partial<AccountHealthDisplayInput> = {}): AccountHealthDisplayInput {
  return {
    accountSummary: {
      status: "available",
      health_score: 41,
      health_score_delta: -6,
      severity: "high",
      executive_summary: "Выручка снизилась до 101721 ₽, это на 20845 ₽ ниже 20.05.2026. Часть рекламного бюджета работает слабо.",
      improvement_summary:
        "Индекс вырастет, если снизить неэффективные рекламные расходы и закрыть SKU с коротким покрытием.",
      improvement_levers: [
        {
          title: "Снизить потери рекламного бюджета",
          explanation: "Неэффективные кампании давят на индекс через блок рекламы.",
          factor_id: "advertising",
          linked_action_id: "action-ad-1",
          expected_direction: "снижение рекламной нагрузки",
        },
        {
          title: "Восстановить покрытие ключевых SKU",
          explanation: "Короткое покрытие усиливает риск падения продаж.",
          factor_id: "stock",
          linked_action_id: null,
          expected_direction: "рост покрытия остатков",
        },
      ],
      risk_drivers: [
        {
          id: "driver-advertising",
          category: "advertising",
          category_label: "Реклама",
          severity: "high",
          impact_score: 88,
          title: "Слабая отдача рекламы",
          reason: "6 кампаний потратили 295 178 ₽ без достаточной отдачи.",
          linked_alert_ids: [],
          linked_recommendation_ids: [],
        },
        {
          id: "driver-stock",
          category: "stock",
          category_label: "Остатки",
          severity: "high",
          impact_score: 75,
          title: "Критичные остатки",
          reason: "6 SKU имеют критичный или короткий остаток.",
          linked_alert_ids: [],
          linked_recommendation_ids: [],
        },
      ],
    },
    whatChanged: {
      status: "available",
      groups: [
        {
          key: "worsened",
          label: "Ухудшилось",
          tone: "negative",
          summary: "Выручка снизилась до 101721 ₽, это на 20845 ₽ ниже 20.05.2026.",
          items: [{ label: "Отмены", text: "Отмены выросли до 12 против 4 днём ранее." }],
        },
        {
          key: "new_risks",
          label: "Новые риски",
          tone: "warning",
          summary: "Есть товары без остатка и с покрытием менее 4 дней.",
          items: [],
        },
      ],
    },
    stockCoverage: {
      status: "available",
      segments: [
        { key: "critical_0_3", label: "0-3 дня", sku_count: 2, severity: "critical" },
        { key: "shortage_4_7", label: "4-7 дней", sku_count: 4, severity: "high" },
      ],
      total_skus: 10,
    },
    adEfficiency: {
      status: "available",
      points: [
        {
          campaign_external_id: 1,
          campaign_name: "Кампания 1",
          spend: 2000,
          drr: 0.8,
          revenue: 1000,
          orders: 0,
          efficiency_signal: "weak_efficiency",
        },
      ],
    },
    topActions: [
      {
        id: "top-1",
        source: "ai_summary",
        priority: "critical",
        priority_label: "Критический",
        category: "advertising",
        category_label: "Реклама",
        title: "Остановить кампанию без заказов.",
        reason: "Кампания потратила бюджет без результата.",
        next_step: "Остановите кампанию и проверьте запросы.",
        supporting_metrics: [],
      },
      {
        id: "top-2",
        source: "ai_summary",
        priority: "high",
        priority_label: "Высокий",
        category: "advertising",
        category_label: "Реклама",
        title: "Сократить расход по слабым рекламным кампаниям.",
        reason: "Часть расходов не окупается.",
        next_step: "Снизьте бюджет до проверки.",
        supporting_metrics: [],
      },
      {
        id: "top-3",
        source: "ai_summary",
        priority: "medium",
        priority_label: "Средний",
        category: "advertising",
        category_label: "Реклама",
        title: "Сократить расход по слабым рекламным кампаниям.",
        reason: "Часть расходов не окупается.",
        next_step: "Снизьте бюджет до проверки.",
        supporting_metrics: [],
      },
    ],
    ...overrides,
  };
}

const LONG_ACTION =
  "Остановите или резко сократите кампанию «Холодный трафик — без заказов» до завершения проверки запросов, ставок и карточек товара.";

const LONG_DRIVER =
  "Рекламный расход растет быстрее выручки в нескольких кампаниях, поэтому текущий бюджет не окупается и дополнительно давит на итоговый индекс кабинета.";

function run() {
  assert(formatHealthScore(41) === "41 / 100", "score 41 formatted");
  assert(formatHealthScore(0) === "0 / 100", "score 0 formatted");
  assert(formatHealthScore(100) === "100 / 100", "score 100 formatted");
  assert(formatHealthScore(undefined) === null, "undefined score hidden");
  assert(formatHealthScore(null) === null, "null score hidden");
  assert(formatHealthScore(-10) === "0 / 100", "negative score clamped");
  assert(formatHealthScore(120) === "100 / 100", "large score clamped");

  assert(getHealthSeverityLabel("critical") === "Критическое состояние", "critical label localized");
  assert(getHealthSeverityLabel("high") === "Зона риска", "high label localized");
  assert(getHealthSeverityLabel("medium") === "Требует внимания", "medium label localized");
  assert(getHealthSeverityLabel("attention") === "Есть зоны внимания", "attention label localized");
  assert(getHealthSeverityLabel("low") === "Стабильное состояние", "low label localized");
  assert(getHealthSeverityLabel(undefined) === "Статус не определён", "missing label handled");
  assert(getHealthSeverityLabel("warning", "Требует внимания") === "Требует внимания", "backend status label wins");

  assert(getHealthScorePercent(41) === 41, "percent from score");
  assert(getHealthScorePercent(-1) === 0, "negative percent clamped");
  assert(getHealthScorePercent(150) === 100, "large percent clamped");

  const drivers = buildHealthDriverViews(makeInput());
  assert((drivers[0]?.reason ?? "").includes("кампаний"), "backend risk driver reason used");
  assert(drivers.length <= 4, "drivers limited");
  assert(drivers.some((item) => item.reason.includes("SKU")), "stock driver uses SKU wording");

  const improvementSummary = buildHealthImprovementSummary(makeInput());
  assert(improvementSummary.includes("неэффективные рекламные расходы"), "backend improvement summary used");
  assert(!improvementSummary.includes("заметных улучшений"), "improvement summary not about lack of improvements");

  const inappropriateSummary = buildHealthImprovementSummary(makeInput({
    accountSummary: {
      ...makeInput().accountSummary,
      improvement_summary: "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
      improvement_levers: [],
    },
  }));
  assert(inappropriateSummary === "", "inappropriate improvement summary hidden");

  const view = buildAccountHealthDisplayView(makeInput());
  assert(view.scoreLabel === "41 / 100", "view score label built");
  assert(view.deltaText === "-6 пунктов к прошлой сводке", "delta text built");
  assert(view.severityLabel === "Критическое состояние", "severity label derived from score bands");

  const backendDrivenView = buildAccountHealthDisplayView(makeInput({
    accountSummary: {
      status: "available",
      health_score: 75,
      health_score_delta: -4,
      severity: "attention",
      improvement_summary: "Индекс улучшится после снижения доли отмен и устранения провала воронки. Конкретные шаги — в блоке «Топ-3 действия».",
      risk_drivers: [
        {
          id: "driver-cancellations",
          category: "cancellations",
          category_label: "Отмены",
          severity: "high",
          impact_score: 78,
          title: "Растет доля отмен",
          reason: "Отмены заняли 18,0% от заказов.",
          linked_alert_ids: [],
          linked_recommendation_ids: [],
        },
        {
          id: "driver-funnel",
          category: "funnel",
          category_label: "Воронка",
          severity: "high",
          impact_score: 71,
          title: "Ослабла воронка продаж",
          reason: "Этап карточка → корзина заметно просел.",
          linked_alert_ids: [],
          linked_recommendation_ids: [],
        },
      ],
    },
    whatChanged: {
      status: "available",
      groups: [
        {
          key: "worsened",
          label: "Ухудшилось",
          tone: "negative",
          items: [{ label: "Fallback", text: "Старый fallback driver." }],
        },
      ],
    },
  }));
  assert(backendDrivenView.drivers[0]?.reason === "Отмены заняли 18,0% от заказов.", "backend risk drivers preferred");
  assert(backendDrivenView.drivers[0]?.title === "Растет доля отмен", "backend risk driver title preserved");
  assert(!backendDrivenView.drivers.some((driver) => driver.reason.includes("Старый fallback driver.")), "fallback driver hidden when backend drivers exist");
  assert(backendDrivenView.severityLabel === "Требует внимания", "score-derived medium severity label preferred");
  assert(
    backendDrivenView.improvementSummary ===
      "Индекс улучшится после снижения доли отмен и устранения провала воронки. Конкретные шаги — в блоке «Топ-3 действия».",
    "backend improvement summary preferred",
  );

  const html = renderToStaticMarkup(<AccountHealthCard data={makeInput()} />);
  assert(html.includes("Индекс состояния кабинета"), "title rendered");
  assert(html.includes("41 / 100"), "formatted score rendered");
  assert(html.includes("Критическое состояние"), "localized severity rendered");
  assert(!html.includes(">high<"), "raw severity hidden");
  assert(html.includes("Индекс учитывает продажи, рекламу, остатки, сигналы и активные риски."), "description rendered");
  assert(html.includes("Основные причины снижения"), "drivers heading rendered");
  assert(html.includes("Слабая отдача рекламы"), "driver title rendered");
  assert(html.includes("Что улучшит индекс"), "summary heading rendered");
  assert(html.includes('data-testid="health-improvement-intro"'), "AI intro rendered separately");
  assert(
    html.includes("Индекс вырастет, если снизить неэффективные рекламные расходы"),
    "improvement intro text rendered",
  );
  assert(!html.includes("Конкретные шаги — в блоке «Топ-3 действия»."), "top actions pointer hidden");
  assert(html.includes('data-testid="health-improvement-items"'), "structured improvement items rendered");
  assert(html.includes("Снизить потери рекламного бюджета"), "improvement item title rendered");
  assert(html.includes("Реклама"), "improvement factor label rendered");
  assert(!html.includes("Остановить кампанию без заказов."), "top action not rendered in health card");
  assert(!html.includes("Пополнить товары с критичным остатком."), "recommended action not rendered in health card");
  assert(html.includes("width:41%"), "progress width rendered");
  assert(html.includes(">50<"), "progress midpoint label rendered");

  const longTextHtml = renderToStaticMarkup(
    <AccountHealthCard
      data={makeInput({
        accountSummary: {
          ...makeInput().accountSummary,
          risk_drivers: [
            {
              id: "driver-advertising",
              category: "advertising",
              category_label: "Реклама",
              severity: "high",
              impact_score: 88,
              title: "Слабая реклама",
              reason: LONG_DRIVER,
              linked_alert_ids: [],
              linked_recommendation_ids: [],
            },
          ],
        },
        topActions: [
          {
            id: "long-top-action",
            source: "ai_summary",
            priority: "critical",
            priority_label: "Критический",
            category: "advertising",
            category_label: "Реклама",
            title: LONG_ACTION,
            reason: "Длинное пояснение.",
            next_step: "Проверить кампанию.",
            supporting_metrics: [],
          },
        ],
      })}
    />,
  );
  assert(longTextHtml.includes(LONG_DRIVER), "long driver rendered in full");
  assert(longTextHtml.includes("Слабая реклама"), "driver title rendered separately");
  assert(!longTextHtml.includes("line-clamp"), "driver text not clamped");
  assert(!longTextHtml.includes("truncate"), "driver text not truncated");
  assert(!longTextHtml.includes(LONG_ACTION), "long action still hidden from health card");
  assert(longTextHtml.includes("Что улучшит индекс"), "improvement block still rendered");

  const missingScoreHtml = renderToStaticMarkup(
    <AccountHealthCard
      data={makeInput({
        accountSummary: {
          status: "available",
          health_score: null,
          severity: "high",
        },
      })}
    />,
  );
  assert(missingScoreHtml.includes("Индекс пока не рассчитан"), "missing score empty state rendered");

  console.log("account-health-display.test.tsx: ok");
}

run();
