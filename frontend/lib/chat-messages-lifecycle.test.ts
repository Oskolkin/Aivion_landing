import assert from "node:assert/strict";
import {
  isStaleMessagesResponse,
  shouldClearVisibleMessages,
  shouldFetchMessagesForSession,
} from "@/lib/chat-messages-lifecycle";
import { parseSessionId, resolveChatViewMode } from "@/lib/chat-view";

assert.equal(parseSessionId("16"), 16);
assert.equal(parseSessionId("0"), null);
assert.equal(parseSessionId("abc"), null);

assert.equal(
  resolveChatViewMode({ urlSessionId: 16, draftActionKey: "stock:1", draftPrefill: "x" }).kind,
  "existing_session",
);

assert.equal(shouldFetchMessagesForSession("existing_session", 16, null), true);
assert.equal(shouldFetchMessagesForSession("existing_session", 16, 16), false);
assert.equal(shouldFetchMessagesForSession("existing_session", 17, 16), true);
assert.equal(shouldFetchMessagesForSession("new_general", null, null), false);

assert.equal(shouldClearVisibleMessages(null, 16, "existing_session"), true);
assert.equal(shouldClearVisibleMessages(16, 16, "existing_session"), false);
assert.equal(shouldClearVisibleMessages(16, 17, "existing_session"), true);
assert.equal(shouldClearVisibleMessages(16, null, "new_general"), true);

assert.equal(isStaleMessagesResponse(1, 2, 16, 16), true);
assert.equal(isStaleMessagesResponse(2, 2, 16, 17), true);
assert.equal(isStaleMessagesResponse(2, 2, 16, 16), false);

console.log("chat-messages-lifecycle.test.ts: ok");
