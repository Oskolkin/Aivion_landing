import { renderToStaticMarkup } from "react-dom/server";
import { WhatChangedCard } from "@/components/dashboard/what-changed-card";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  const html = renderToStaticMarkup(
    <WhatChangedCard
      whatChanged={{
        status: "available",
        summary: "Продажи растут.\n\nРиски по остаткам усилились.\n\nПриоритет — пополнить ключевые SKU.",
        main_conclusion: "Продажи растут.\n\nРиски по остаткам усилились.\n\nПриоритет — пополнить ключевые SKU.",
        groups: [
          {
            key: "worsened",
            label: "Ухудшилось",
            tone: "negative",
            summary: "Главное ухудшение.",
            items: [
              {
                label: "Без остатка осталось 4 SKU",
                text: "Количество выросло с 2 до 4.",
                current_value_text: "4 SKU",
                previous_value_text: "2 SKU",
                highlights: ["4 SKU"],
              },
            ],
          },
          { key: "new_risks", label: "Новые риски", tone: "warning", summary: "Новые сигналы.", items: [] },
          {
            key: "unchanged_risks",
            label: "Активные проблемы",
            tone: "neutral",
            summary: "Недостаточно истории публикаций.",
            items: [],
          },
          {
            key: "improved",
            label: "Улучшилось",
            tone: "positive",
            summary: "Заметных улучшений не выявлено.",
            empty_text: "Заметных улучшений за неделю не выявлено.",
            items: [],
          },
        ],
      }}
    />,
  );

  assert(html.includes("Что изменилось за неделю"), "card title rendered");
  assert(html.includes("Главный вывод"), "main conclusion label rendered");
  assert(html.includes("data-testid=\"what-changed-groups-grid\""), "groups grid test id");
  assert(html.includes("sm:grid-cols-2"), "2-column desktop grid");
  assert(!html.includes("xl:grid-cols-4"), "no 4-column desktop grid");
  assert(!html.includes("grid-cols-4"), "no 4-column grid");
  assert(html.includes("h-full"), "equal-height group stretch");
  assert(html.includes("Ухудшилось"), "group title rendered");
  assert(html.includes("Было: 2 SKU → Стало: 4 SKU") || html.includes("2 SKU"), "comparison values rendered");
  assert(html.includes("Активные проблемы"), "empty active-problems group shown");
  assert(!html.includes("max-h-[100px]"), "no clipped empty-state max-height");
  assert(!html.includes("overflow-hidden"), "no overflow-hidden clipping on empty state");
  assert(!/\b882\d{9}\b/.test(html), "no numeric campaign ids");

  console.log("what-changed-card.test.tsx: ok");
}

run();
