import assert from "node:assert/strict";
import {
  buildAttachmentLimitError,
  mapAttachmentRemovalError,
  validateAttachmentBatchSelection,
} from "@/lib/chat-attachments";
import { CHAT_ATTACHMENT_MAX_FILES } from "@/lib/chat-display";

assert.equal(
  buildAttachmentLimitError({ selectedCount: 6, existingCount: 0 }),
  "Можно прикрепить не более 5 файлов. Вы выбрали 6 — файлы не были добавлены.",
);

assert.equal(
  buildAttachmentLimitError({ selectedCount: 3, existingCount: 3 }),
  "Можно добавить ещё 2 файла. Вы выбрали 3 — файлы не были добавлены.",
);

assert.equal(
  buildAttachmentLimitError({ selectedCount: 1, existingCount: 5 }),
  "Достигнут лимит: можно прикрепить не более 5 файлов.",
);

const rejectSix = validateAttachmentBatchSelection({ selectedCount: 6, existingCount: 0 });
assert.equal(rejectSix.ok, false);
if (!rejectSix.ok) {
  assert.match(rejectSix.message, /Вы выбрали 6/);
}

const rejectThreeMore = validateAttachmentBatchSelection({ selectedCount: 3, existingCount: 3 });
assert.equal(rejectThreeMore.ok, false);
if (!rejectThreeMore.ok) {
  assert.match(rejectThreeMore.message, /ещё 2 файла/);
}

const acceptTwo = validateAttachmentBatchSelection({ selectedCount: 2, existingCount: 3 });
assert.equal(acceptTwo.ok, true);

const acceptFive = validateAttachmentBatchSelection({
  selectedCount: CHAT_ATTACHMENT_MAX_FILES,
  existingCount: 0,
});
assert.equal(acceptFive.ok, true);

assert.match(
  mapAttachmentRemovalError(new DOMException("Aborted", "AbortError")),
  /Не удалось удалить файл/,
);

console.log("chat-attachments.test.ts passed");
