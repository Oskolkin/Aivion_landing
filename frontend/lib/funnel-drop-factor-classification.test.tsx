import { renderToStaticMarkup } from "react-dom/server";
import { FunnelAIAnalysisContent } from "@/components/dashboard/funnel-ai-analysis";
import {
  buildDropFactorSections,
  buildFunnelAnalysisDisplayModel,
  classifyDropFactorGroup,
} from "@/lib/funnel-ai-analysis-view";
import type { FunnelAIAnalysis } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function assertRemovedSectionsAbsent(html: string) {
  const forbidden = [
    "Сигналы просадки и возможные усилители",
    "Сигналы просадки",
    "Факторы, которые могли усилить эффект",
    "Направления проверки",
    "Подробнее",
  ];
  for (const text of forbidden) {
    assert(!html.includes(text), `removed section must not render: ${text}`);
  }
}

function run() {
  assert(classifyDropFactorGroup("cart_add_decline") === "funnel_signal", "cart_add_decline group");
  assert(classifyDropFactorGroup("aov_drop") === "business_signal", "aov_drop group");
  assert(classifyDropFactorGroup("stock_pressure") === "possible_amplifier", "stock_pressure group");
  assert(classifyDropFactorGroup("inefficient_ads") === "context_hypothesis", "inefficient_ads group");
  assert(classifyDropFactorGroup("mystery_key") === "context_hypothesis", "unknown key fallback");

  const analysis: FunnelAIAnalysis = {
    status: "available",
    drop_factors: [
      {
        key: "inefficient_ads",
        group: "funnel_signal",
        evidence_level: "confirmed",
        label: "Реклама",
      },
      { key: "views_decline", evidence_level: "confirmed", label: "Просмотры" },
      { key: "aov_drop", evidence_level: "likely", label: "Средний чек", explanation: "Чек ниже." },
      {
        key: "stock_pressure",
        evidence_level: "confirmed",
        label: "Остатки",
        explanation: "Дефицит.",
      },
      { key: "cart_add_decline", evidence_level: "likely", label: "Корзина", explanation: "Падение." },
    ],
  };

  const model = buildFunnelAnalysisDisplayModel(analysis);
  const byKey = Object.fromEntries(model.dropFactors.map((f) => [f.key, f]));

  assert(byKey.cart_add_decline?.group === "funnel_signal", "cart group in model");
  assert(byKey.cart_add_decline?.evidenceLevel === "confirmed", "cart evidence");
  assert(byKey.views_decline?.group === "funnel_signal", "views group");
  assert(byKey.views_decline?.evidenceLevel === "likely", "views downgraded from confirmed");
  assert(byKey.views_decline?.limitation?.includes("следующего этапа"), "views limitation");
  assert(byKey.aov_drop?.group === "business_signal", "aov group");
  assert(byKey.aov_drop?.evidenceLevel === "confirmed", "aov evidence");
  assert(
    byKey.aov_drop?.limitation?.includes("финансовый сигнал") &&
      byKey.aov_drop?.limitation?.includes("не доказанная причина"),
    "aov limitation wording",
  );
  assert(byKey.stock_pressure?.group === "possible_amplifier", "stock group");
  assert(byKey.stock_pressure?.evidenceLevel === "limited", "stock evidence");
  assert(byKey.stock_pressure?.limitation?.includes("срезом"), "stock limitation");
  assert(byKey.inefficient_ads?.group === "context_hypothesis", "ads group overrides AI");
  assert(byKey.inefficient_ads?.evidenceLevel === "hypothesis", "ads evidence");
  assert(byKey.inefficient_ads?.limitation?.includes("кампаний"), "ads limitation");

  const sections = buildDropFactorSections(model.dropFactors);
  assert(sections.length === 2, "two view-model sections");
  assert(sections[0]?.title === "Сигналы просадки", "signals section title");
  assert(sections[0]?.factors.length === 2, "two funnel signals");
  assert(sections[1]?.title === "Факторы, которые могли усилить эффект", "amplifiers title");
  assert(sections[1]?.factors.length === 3, "three amplifier factors");

  const emptyGroup: FunnelAIAnalysis = {
    status: "available",
    drop_factors: [{ key: "cart_add_decline", label: "Корзина", explanation: "Сигнал." }],
  };
  const legacy = buildFunnelAnalysisDisplayModel(emptyGroup);
  assert(legacy.dropFactors[0]?.group === "funnel_signal", "payload without group still works");

  const sectionHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={analysis} isExpanded={false} onToggle={() => undefined} />,
  );
  assertRemovedSectionsAbsent(sectionHtml);
  assert(!sectionHtml.includes('data-drop-factor-key="views_decline"'), "drop factor cards not rendered");
  assert(!sectionHtml.includes('data-drop-factor-section="drop_signals"'), "drop factor sections not rendered");

  const emptyHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent
      analysis={{ status: "available", drop_factors: [] }}
      isExpanded={false}
      onToggle={() => undefined}
    />,
  );
  assertRemovedSectionsAbsent(emptyHtml);

  const onlySignals: FunnelAIAnalysis = {
    status: "available",
    drop_factors: [{ key: "cart_add_decline", label: "Корзина", explanation: "Сигнал." }],
  };
  const onlySignalsHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={onlySignals} isExpanded={false} onToggle={() => undefined} />,
  );
  assertRemovedSectionsAbsent(onlySignalsHtml);

  const unknownOnly: FunnelAIAnalysis = {
    status: "available",
    drop_factors: [{ key: "mystery_factor", label: "Неизвестный фактор", explanation: "Контекст." }],
  };
  const unknownHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={unknownOnly} isExpanded={false} onToggle={() => undefined} />,
  );
  assertRemovedSectionsAbsent(unknownHtml);
}

run();
