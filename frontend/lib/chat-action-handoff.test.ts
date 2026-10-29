import assert from "node:assert/strict";
import {
  ACTION_ASK_MESSAGE,
  buildActionChatHref,
  canStartActionHandoff,
  isActionHandoffBusy,
  nextHandoffPhaseAfterResolve,
  shouldAutoSendActionHandoff,
} from "@/lib/chat-action-handoff";
import { shouldShowEmptyState, shouldShowSuggestions } from "@/lib/chat-view";

assert.ok(ACTION_ASK_MESSAGE.includes("затронутые объекты"));

assert.equal(buildActionChatHref({ sessionId: 42 }), "/app/chat?session=42");
assert.equal(buildActionChatHref({ actionViewId: "view-1111" }), "/app/chat?action_view_id=view-1111");
assert.equal(buildActionChatHref({}), "/app/chat");

assert.equal(canStartActionHandoff("idle"), true);
assert.equal(canStartActionHandoff("failed"), true);
assert.equal(canStartActionHandoff("ready"), false);
assert.equal(canStartActionHandoff("sending_initial_message"), false);

assert.equal(isActionHandoffBusy("resolving_action"), true);
assert.equal(isActionHandoffBusy("ready"), false);
assert.equal(shouldAutoSendActionHandoff("sending_initial_message"), true);
assert.equal(nextHandoffPhaseAfterResolve(true), "ready");
assert.equal(nextHandoffPhaseAfterResolve(false), "creating_or_loading_conversation");

assert.equal(shouldShowEmptyState({ kind: "new_general" }, 0, false), true);
assert.equal(
  shouldShowEmptyState({ kind: "draft_action", actionKey: "a", prefill: "x" }, 0, false),
  false,
);
assert.equal(shouldShowSuggestions({ kind: "draft_action", actionKey: "a", prefill: "x" }, 0, false), false);

console.log("chat-action-handoff.test.ts passed");
