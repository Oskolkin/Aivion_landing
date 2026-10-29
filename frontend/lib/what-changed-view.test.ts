import {
  buildWhatChangedItemDisplay,
  buildWhatChangedView,
  labelHasCount,
  splitWhatChangedItemText,
} from "@/lib/what-changed-view";
import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const GROUP_ORDER_KEYS = "worsened,new_risks,unchanged_risks,improved";

function makeWhatChanged(
  overrides: Partial<DashboardHomeResponse["ai_diagnostics"]["what_changed"]> = {},
): DashboardHomeResponse["ai_diagnostics"]["what_changed"] {
  return {
    status: "available",
    summary: "Относительно прошлой сводки сильнее проявились негативные изменения.",
    groups: [
      {
        key: "worsened",
        label: "Ухудшилось",
        tone: "negative",
        summary: "Главное ухудшение — продажи стали менее денежными.",
        items: [
          {
            label: "Выручка",
            text: "Снизилась до 101 721 ₽, на 20 845 ₽ ниже прошлой сводки.",
            highlights: ["101 721 ₽", "20 845 ₽"],
          },
        ],
      },
      {
        key: "new_risks",
        label: "Новые риски",
        tone: "warning",
        summary: "Появились новые сигналы.",
        items: [],
        empty_text: "",
      },
      {
        key: "unchanged_risks",
        label: "Активные проблемы",
        tone: "neutral",
        summary: "Часть проблем не новая.",
        items: [],
      },
      {
        key: "improved",
        label: "Улучшилось",
        tone: "positive",
        summary: "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
        empty_text: "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе.",
        items: [],
      },
    ],
    ...overrides,
  };
}

function run() {
  const view = buildWhatChangedView(makeWhatChanged());
  assert(view.groups.length === 4, "always renders all 4 canonical groups");
  assert(
    view.groups.map((group) => group.key).join(",") === GROUP_ORDER_KEYS,
    "canonical order: worsened, new_risks, unchanged_risks, improved",
  );
  assert(view.groups.every((group) => !labelHasCount(group.label)), "labels without counts");
  assert(view.groups[0].label === "Ухудшилось", "worsened label preserved");

  const segments = splitWhatChangedItemText("Снизилась до 101 721 ₽", ["101 721 ₽"]);
  assert(segments.some((segment) => segment.bold && segment.text === "101 721 ₽"), "highlights become bold");

  const emptyOnly = buildWhatChangedView(
    makeWhatChanged({
      groups: [
        { key: "worsened", label: "Ухудшилось", tone: "negative", summary: "", items: [] },
        { key: "new_risks", label: "Новые риски", tone: "warning", summary: "", items: [] },
        { key: "unchanged_risks", label: "Активные проблемы", tone: "neutral", summary: "", items: [] },
        {
          key: "improved",
          label: "Улучшилось",
          tone: "positive",
          summary: "",
          empty_text: "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе.",
          items: [],
        },
      ],
    }),
  );
  assert(emptyOnly.groups.length === 4, "always renders all 4 groups, even when all empty");
  assert(emptyOnly.groups.every((group) => group.items.length === 0), "all groups empty");
  const improved = emptyOnly.groups.find((group) => group.key === "improved");
  assert(
    improved?.emptyText === "Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе." ||
      improved?.emptyText === "Заметных улучшений за неделю не выявлено.",
    "empty improved keeps API empty_text or default",
  );
  const active = emptyOnly.groups.find((group) => group.key === "unchanged_risks");
  assert(
    Boolean(active?.emptyText?.includes("Недостаточно истории") || active?.emptyText?.includes("не выявлено")),
    "active problems empty text is history-safe",
  );
  const worsenedEmpty = emptyOnly.groups.find((group) => group.key === "worsened");
  assert(worsenedEmpty?.emptyText === "Заметных ухудшений не выявлено.", "empty worsened uses DEFAULT_EMPTY_TEXT");
  assert(view.summary.endsWith("."), "summary normalized with trailing period");

  const mainConclusionView = buildWhatChangedView(
    makeWhatChanged({
      main_conclusion: "Выручка снизилась на 5,03%, основной риск — дефицит ходовых SKU.",
      summary: "Ключевые изменения и активные сигналы по продажам.",
    }),
  );
  assert(
    mainConclusionView.summary.includes("Выручка снизилась на 5,03%"),
    "main_conclusion is rendered instead of generic summary",
  );
  assert(!mainConclusionView.summary.includes("активные сигналы"), "generic summary is not shown");

  const labeledItem = buildWhatChangedItemDisplay({
    label: "Выручка",
    text: "Снизилась до 101 721 ₽, на 20 845 ₽ ниже прошлой сводки.",
    highlights: [],
  });
  assert(labeledItem.title === "Выручка", "item label becomes title");
  assert(Boolean(labeledItem.subtitle?.includes("101")), "item text becomes subtitle");

  // Regression: Unified Analysis may emit thematic groups (e.g. "sales",
  // "ads") instead of the legacy worsened/new_risks/unchanged_risks/improved
  // keys. Those must render in API order, not be silently dropped.
  const thematicView = buildWhatChangedView(
    makeWhatChanged({
      groups: [
        {
          key: "sales",
          label: "Продажи",
          tone: "negative",
          summary: "Падение заказов 7д к предыдущим 7д",
          items: [{ label: "Заказы", text: "Заказы снизились на 12%", highlights: ["-12%"] }],
        },
        {
          key: "ads",
          label: "Реклама",
          tone: "warning",
          items: [{ label: "ДРР", text: "ДРР вырос до 28%" }],
        },
      ],
    }),
  );
  assert(thematicView.groups.length === 2, "thematic groups render all API groups, not padded to 4");
  assert(thematicView.groups[0].key === "sales", "thematic groups preserve API order (sales first)");
  assert(thematicView.groups[1].key === "ads", "thematic groups preserve API order (ads second)");
  assert(thematicView.groups[0].label === "Продажи", "thematic group label passed through");
  assert(thematicView.groups[0].items.length === 1, "thematic group items rendered");
  assert(thematicView.groups[0].tone === "negative", "thematic group tone normalized");

  // A mix of legacy + thematic groups still renders all 4 canonical groups
  // (thematic keys outside GROUP_ORDER are not part of the legacy layout).
  const mixedView = buildWhatChangedView(
    makeWhatChanged({
      groups: [
        {
          key: "worsened",
          label: "Ухудшилось",
          tone: "negative",
          items: [{ label: "Выручка", text: "Снизилась", highlights: [] }],
        },
        { key: "sales", label: "Продажи", tone: "negative", items: [] },
      ],
    }),
  );
  assert(mixedView.groups.length === 4, "legacy path always renders all 4 canonical groups");
  assert(mixedView.groups[0].key === "worsened", "worsened retained");
  assert(mixedView.groups[0].items.length === 1, "worsened keeps its item");

  // Thematic group with no items falls back to its empty_text/summary, and
  // ultimately a generic placeholder rather than crashing or rendering blank.
  const thematicEmptyView = buildWhatChangedView(
    makeWhatChanged({
      groups: [{ key: "stock", label: "Остатки", tone: "neutral", items: [] }],
    }),
  );
  assert(thematicEmptyView.groups.length === 1, "single thematic group renders");
  assert(thematicEmptyView.groups[0].summary.length > 0, "empty thematic group has a fallback summary");

  // SUMMARY_EMPTY placeholder must not appear when summary/main_conclusion is present.
  const withSummaryOnly = buildWhatChangedView(
    makeWhatChanged({ main_conclusion: undefined, summary: "Ключевые изменения по продажам." }),
  );
  assert(
    withSummaryOnly.summary === "Ключевые изменения по продажам.",
    "summary is used when main_conclusion is absent",
  );
  assert(!withSummaryOnly.summary.includes("появится после обновления"), "SUMMARY_EMPTY not shown when summary present");

  const noSummaryView = buildWhatChangedView(makeWhatChanged({ main_conclusion: undefined, summary: undefined }));
  assert(
    noSummaryView.summary === "AI-вывод появится после обновления сводки.",
    "SUMMARY_EMPTY placeholder shown only when both summary and main_conclusion are absent",
  );

  console.log("what-changed-view.test.ts: ok");
}

run();
