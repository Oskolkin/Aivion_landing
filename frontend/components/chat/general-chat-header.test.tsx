import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChatContextCard, ChatCompactGeneralContextCard } from "@/components/chat/chat-context-card";
import { GeneralChatHeader } from "@/components/chat/general-chat-header";
import {
  GENERAL_CHAT_DESCRIPTION,
  GENERAL_CHAT_TITLE,
  generalChatCapabilities,
} from "@/components/chat/general-chat-capabilities";
import { mapSessionToDisplay } from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";

const heroHtml = renderToStaticMarkup(<GeneralChatHeader />);

assert.match(heroHtml, /AI-чат по кабинету/);
assert.match(heroHtml, new RegExp(GENERAL_CHAT_TITLE.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
assert.match(heroHtml, new RegExp(GENERAL_CHAT_DESCRIPTION.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
assert.doesNotMatch(heroHtml, /Анализ, отчеты и ответы на любые вопросы/);
assert.doesNotMatch(heroHtml, /AI отвечает на основе актуальных данных/);

for (const item of generalChatCapabilities) {
  assert.match(heroHtml, new RegExp(item.label));
}

assert.doesNotMatch(heroHtml, /<button[^>]*>Продажи<\/button>/);
assert.match(heroHtml, /aria-hidden="true"/);

const actionSession: ChatSession = {
  id: 10,
  title: "Разберите 8 SKU без заказов за 7 дней",
  action_view_id: "view-stock-8sku",
  status: "active",
  created_at: "2026-06-30T10:00:00Z",
  updated_at: "2026-06-30T12:00:00Z",
  last_message_at: "2026-06-30T12:00:00Z",
};
const actionHtml = renderToStaticMarkup(
  <ChatContextCard session={mapSessionToDisplay(actionSession)} />,
);
assert.match(actionHtml, /Чат по действию/);
assert.doesNotMatch(actionHtml, />Действие</);
assert.doesNotMatch(actionHtml, />Открыто</);
assert.doesNotMatch(actionHtml, /Анализ и отчёты по данным вашего кабинета/);

const compactHtml = renderToStaticMarkup(<ChatCompactGeneralContextCard />);
assert.match(compactHtml, /AI-чат по кабинету/);
assert.match(compactHtml, /Ответы на основе данных вашего кабинета/);
assert.doesNotMatch(compactHtml, /Анализ и отчёты по данным вашего кабинета/);

const newGeneralHtml = renderToStaticMarkup(<ChatContextCard session={null} variant="hero" />);
assert.match(newGeneralHtml, /Анализ и отчёты по данным вашего кабинета/);

console.log("general-chat-header.test.tsx passed");
