import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChatContextCard, ChatGeneralContextCard } from "@/components/chat/chat-context-card";
import { GENERAL_CHAT_TITLE } from "@/components/chat/general-chat-capabilities";
import { ChatComposer } from "@/components/chat/chat-composer";
import { ChatSessionItem } from "@/components/chat/chat-session-item";
import { ChatSessionList } from "@/components/chat/chat-session-list";
import { mapSessionToDisplay } from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";
import { resolveChatViewMode, shouldShowSuggestions } from "@/lib/chat-view";

const generalHtml = renderToStaticMarkup(<ChatGeneralContextCard />);
assert.match(generalHtml, /AI-чат по кабинету/);
assert.match(generalHtml, new RegExp(GENERAL_CHAT_TITLE.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));

const actionSession: ChatSession = {
  id: 10,
  title: "Разберите 8 SKU без заказов за 7 дней",
  action_view_id: "view-stock-8sku",
  status: "active",
  created_at: "2026-06-30T10:00:00Z",
  updated_at: "2026-06-30T12:00:00Z",
  last_message_at: "2026-06-30T12:00:00Z",
};
const actionDisplay = mapSessionToDisplay(actionSession);
const actionHtml = renderToStaticMarkup(<ChatContextCard session={actionDisplay} />);
assert.match(actionHtml, /Чат по действию/);
assert.doesNotMatch(actionHtml, />Действие</);
assert.doesNotMatch(actionHtml, />Открыто</);

const sessionItemHtml = renderToStaticMarkup(
  <ChatSessionItem
    session={actionDisplay}
    selected
    onSelect={() => undefined}
    onArchive={() => undefined}
  />,
);
assert.doesNotMatch(sessionItemHtml, /ДЕЙСТВИЕ|Действие|Аналитика|Выполнено/);
assert.match(sessionItemHtml, /Разберите 8 SKU/);

const listHtml = renderToStaticMarkup(
  <ChatSessionList
    groups={{ today: [actionDisplay], earlier: [] }}
    archivedCount={0}
    showArchived={false}
    onToggleArchived={() => undefined}
    viewMode={{ kind: "new_general" }}
    selectedSessionId={null}
    onNewChat={() => undefined}
    onSelect={() => undefined}
    onArchive={() => undefined}
    onRename={() => undefined}
    onDelete={() => undefined}
    archivingSessionId={null}
    renamingSessionId={null}
    deletingSessionId={null}
    loading={false}
    error={null}
  />,
);
assert.match(listHtml, /Новый чат/);
assert.doesNotMatch(listHtml, /Фильтр|SlidersHorizontal/);

const composerHtml = renderToStaticMarkup(
  <ChatComposer value="" onChange={() => undefined} onSubmit={() => undefined} isEmptyChat />,
);
assert.match(composerHtml, /Прикрепить файл/);
assert.match(composerHtml, /Добавить файл/);
assert.doesNotMatch(composerHtml, /Данные кабинета|aria-label="Данные кабинета"/);

const suggestions = shouldShowSuggestions({ kind: "new_general" }, 0, false);
assert.equal(suggestions, true);
assert.equal(shouldShowSuggestions({ kind: "draft_action", actionKey: "a", prefill: "x" }, 0, false), false);

const existingMode = resolveChatViewMode({
  urlSessionId: 5,
  draftActionKey: null,
  draftPrefill: "",
});
assert.equal(existingMode.kind, "existing_session");

console.log("chat-components.test.tsx passed");
