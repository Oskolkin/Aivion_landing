import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChatAssistantMessage } from "@/components/chat/chat-assistant-message";
import { ChatContextCard, ChatCompactGeneralContextCard } from "@/components/chat/chat-context-card";
import { ChatUserMessage } from "@/components/chat/chat-user-message";
import {
  isControlledFallbackMessage,
  isCorruptedChatTitle,
  mapSessionToDisplay,
  resolveSessionTitle,
} from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";
import { resolveChatViewMode, shouldShowEmptyState } from "@/lib/chat-view";

const corrupted: ChatSession = {
  id: 3,
  title: "??????? orders ? SKU-TEST-002?",
  status: "active",
  created_at: "2026-07-01T10:00:00Z",
  updated_at: "2026-07-01T12:00:00Z",
};

assert.equal(isCorruptedChatTitle(corrupted.title), true);
assert.equal(resolveSessionTitle(corrupted), "Новый чат");
assert.equal(mapSessionToDisplay(corrupted).title, "Новый чат");

const russian: ChatSession = {
  id: 4,
  title: "Покажи товары без заказов за последние 7 дней",
  status: "active",
  created_at: "2026-07-04T10:00:00Z",
  updated_at: "2026-07-04T12:00:00Z",
};
assert.equal(isCorruptedChatTitle(russian.title), false);
assert.match(mapSessionToDisplay(russian).title, /Покажи товары/);

const actionSession: ChatSession = {
  id: 5,
  title: "Разберите 8 SKU без заказов",
  action_view_id: "view-stock-8sku",
  status: "active",
  created_at: "2026-06-30T10:00:00Z",
  updated_at: "2026-06-30T12:00:00Z",
};
assert.match(mapSessionToDisplay(actionSession).title, /Разберите 8 SKU/);

assert.equal(shouldShowEmptyState({ kind: "new_general" }, 0, false), true);
assert.equal(shouldShowEmptyState({ kind: "existing_session", sessionId: 1 }, 0, false), false);

const compactHtml = renderToStaticMarkup(<ChatCompactGeneralContextCard />);
assert.match(compactHtml, /AI-чат по кабинету/);
assert.doesNotMatch(compactHtml, /Анализ и отчёты по данным вашего кабинета/);

const existingGeneralHtml = renderToStaticMarkup(
  <ChatContextCard session={mapSessionToDisplay(russian)} variant="compact" />,
);
assert.match(existingGeneralHtml, /Ответы на основе данных вашего кабинета/);

const newGeneralHtml = renderToStaticMarkup(<ChatContextCard session={null} variant="hero" />);
assert.match(newGeneralHtml, /Анализ и отчёты по данным вашего кабинета/);

const fallbackHtml = renderToStaticMarkup(
  <ChatAssistantMessage
    message={{
      id: 1,
      session_id: 1,
      role: "assistant",
      message_type: "answer",
      content: "Ответ не прошёл проверку по фактам из кабинета. Задайте более конкретный вопрос.",
      created_at: "2026-07-04T12:00:00Z",
    }}
  />,
);
assert.match(fallbackHtml, /Не удалось подготовить подтверждённый ответ/);
assert.doesNotMatch(fallbackHtml, /Детали ответа|уверенност|related_alert|Trace ID|намерение/);

const cleanHtml = renderToStaticMarkup(
  <ChatAssistantMessage
    message={{
      id: 2,
      session_id: 1,
      role: "assistant",
      message_type: "answer",
      content: "Заказано на сумму по SKU417 за 7 дней — 74.",
      created_at: "2026-07-04T12:00:00Z",
    }}
    meta={{ limitations: ["Период ограничен 90 днями."] }}
  />,
);
assert.match(cleanHtml, /Aivion/);
assert.match(cleanHtml, /Период ограничен/);
assert.doesNotMatch(cleanHtml, /Детали ответа|уверенност|Кратко:|Оповещение/);
assert.equal(
  isControlledFallbackMessage(
    "Ответ не прошёл проверку по фактам из кабинета.",
    ["Ответ сформирован без вызова OpenAI (controlled fallback)."],
  ),
  true,
);

const userHtml = renderToStaticMarkup(
  <ChatUserMessage
    content="Проверь файл"
    createdAt="2026-07-04T12:00:00Z"
    attachments={[
      {
        id: "att-1",
        name: "very-long-export-filename-that-should-stay-contained.csv",
        kind: "csv",
        sizeLabel: "68 B",
      },
    ]}
  />,
);
assert.match(userHtml, /border-white\/20/);
assert.match(userHtml, /mt-3 whitespace-pre-wrap/);

assert.equal(
  resolveChatViewMode({ urlSessionId: null, draftActionKey: null, draftPrefill: "" }).kind,
  "new_general",
);

console.log("chat-stabilization.test.ts: ok");
