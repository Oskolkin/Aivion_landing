import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ActionChatHeader } from "@/components/chat/action-chat-header";
import {
  ACTION_CHAT_DESCRIPTION,
  ACTION_CHAT_EYEBROW,
  ACTION_CHAT_OPEN_LABEL,
} from "@/components/chat/action-chat-copy";
import { ChatContextCard } from "@/components/chat/chat-context-card";
import { mapSessionToDisplay } from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";

const actionTitle = "Остановите кампанию «Холодный трафик — без заказов»";

const heroHtml = renderToStaticMarkup(
  <ActionChatHeader title={actionTitle} actionViewId="view-ad-cold" />,
);

assert.match(heroHtml, new RegExp(ACTION_CHAT_EYEBROW));
assert.match(heroHtml, /Остановите кампанию/);
assert.match(heroHtml, new RegExp(ACTION_CHAT_OPEN_LABEL));
assert.match(heroHtml, new RegExp(ACTION_CHAT_DESCRIPTION.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
assert.match(heroHtml, /\/app\/actions\?action=view-ad-cold/);
assert.match(heroHtml, /line-clamp-2/);
assert.match(heroHtml, /aria-hidden="true"/);

assert.doesNotMatch(heroHtml, />Действие</);
assert.doesNotMatch(heroHtml, />Открыто</);
assert.doesNotMatch(heroHtml, /Расход без заказов/);
assert.doesNotMatch(heroHtml, /<button[^>]*>Открыть действие<\/button>/);

const skeletonHtml = renderToStaticMarkup(<ActionChatHeader title="" loading />);
assert.match(skeletonHtml, /aria-busy="true"/);

const errorHtml = renderToStaticMarkup(
  <ActionChatHeader title="" error="Действие не найдено" />,
);
assert.match(errorHtml, /role="alert"/);
assert.match(errorHtml, /Вернуться к действиям/);

const actionSession: ChatSession = {
  id: 10,
  title: actionTitle,
  action_view_id: "view-ad-cold",
  status: "active",
  created_at: "2026-06-30T10:00:00Z",
  updated_at: "2026-06-30T12:00:00Z",
  last_message_at: "2026-06-30T12:00:00Z",
};

const routedHtml = renderToStaticMarkup(
  <ChatContextCard session={mapSessionToDisplay(actionSession)} />,
);
assert.match(routedHtml, /Остановите кампанию/);
assert.doesNotMatch(routedHtml, />Открыто</);

const generalHtml = renderToStaticMarkup(<ChatContextCard session={null} />);
assert.match(generalHtml, /Анализ и отчёты по данным вашего кабинета/);
assert.doesNotMatch(generalHtml, /Остановите кампанию/);

console.log("action-chat-header.test.tsx passed");
