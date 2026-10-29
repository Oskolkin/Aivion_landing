import assert from "node:assert/strict";
import { applyPromptChip, buildChatTitle, shouldShowSuggestions } from "@/lib/chat-view";

assert.equal(buildChatTitle({ actionTitle: "Разберите 8 SKU без заказов" }), "Разберите 8 SKU без заказов");
assert.equal(
  buildChatTitle({ firstUserMessage: "Сделай ABC-анализ товарной базы" }),
  "Сделай ABC-анализ товарной базы",
);
assert.equal(buildChatTitle({ fallback: "Новый чат" }), "Новый чат");

assert.equal(applyPromptChip("", "Сделай ABC"), "Сделай ABC");
assert.equal(applyPromptChip("Уже есть текст", "Новый prompt"), "Уже есть текст\nНовый prompt");

assert.equal(shouldShowSuggestions({ kind: "new_general" }, 0, false), true);
assert.equal(shouldShowSuggestions({ kind: "existing_session", sessionId: 1 }, 0, false), false);
assert.equal(
  shouldShowSuggestions({ kind: "draft_action", actionKey: "a", prefill: "x" }, 0, false),
  false,
);

console.log("chat-view.test.ts passed");
