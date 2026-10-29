import assert from "node:assert/strict";
import { validateAttachmentBatchSelection } from "@/lib/chat-attachments";

let uploadCalls = 0;
let deleteCalls = 0;
let attachments: Array<{ localId: string; attachmentId?: string; status: string; removalError?: string }> = [];
const removingLocalIds = new Set<string>();

function uploadSelectedFiles(files: File[]) {
  uploadCalls += files.length;
}

function handleFileSelection(selectedFiles: File[], existingCount: number) {
  const validation = validateAttachmentBatchSelection({
    selectedCount: selectedFiles.length,
    existingCount,
  });
  if (!validation.ok) {
    return { ok: false as const, message: validation.message };
  }
  uploadSelectedFiles(selectedFiles);
  return { ok: true as const };
}

async function handleRemoveAttachment(localId: string) {
  if (removingLocalIds.has(localId)) return;
  const target = attachments.find((item) => item.localId === localId);
  if (!target) return;

  if (!target.attachmentId) {
    attachments = attachments.filter((item) => item.localId !== localId);
    return;
  }

  removingLocalIds.add(localId);
  target.status = "removing";
  try {
    deleteCalls += 1;
    throw new Error("delete failed");
  } catch {
    target.status = "uploaded";
    target.removalError = "Не удалось удалить";
  } finally {
    removingLocalIds.delete(localId);
  }
}

async function run() {
  // Limit: 0 existing + select 6
  uploadCalls = 0;
  const six = handleFileSelection(new Array(6).fill(new File(["a"], "a.txt")), 0);
  assert.equal(six.ok, false);
  assert.equal(uploadCalls, 0);

  // Limit: 3 existing + select 3
  uploadCalls = 0;
  const threeMore = handleFileSelection(new Array(3).fill(new File(["a"], "a.txt")), 3);
  assert.equal(threeMore.ok, false);
  assert.equal(uploadCalls, 0);

  // Limit: 0 existing + select 5
  uploadCalls = 0;
  const five = handleFileSelection(new Array(5).fill(new File(["a"], "a.txt")), 0);
  assert.equal(five.ok, true);
  assert.equal(uploadCalls, 5);

  // Limit: 3 existing + select 2
  uploadCalls = 0;
  const twoMore = handleFileSelection(new Array(2).fill(new File(["a"], "b.txt")), 3);
  assert.equal(twoMore.ok, true);
  assert.equal(uploadCalls, 2);

  // Removal: success clears card
  attachments = [{ localId: "a", attachmentId: "id-1", status: "uploaded" }];
  removingLocalIds.clear();
  deleteCalls = 0;
  if (!removingLocalIds.has("a")) {
    removingLocalIds.add("a");
    attachments = attachments.filter((item) => item.localId !== "a");
    removingLocalIds.delete("a");
  }
  assert.equal(attachments.length, 0);

  // Removal: error clears spinner state
  attachments = [{ localId: "b", attachmentId: "id-2", status: "uploaded" }];
  await handleRemoveAttachment("b");
  assert.equal(removingLocalIds.has("b"), false);
  assert.equal(attachments[0].status, "uploaded");
  assert.equal(attachments[0].removalError, "Не удалось удалить");

  // Double click guard
  deleteCalls = 0;
  attachments = [{ localId: "c", attachmentId: "id-3", status: "uploaded" }];
  removingLocalIds.add("c");
  await handleRemoveAttachment("c");
  assert.equal(deleteCalls, 0);
}

void run().then(() => {
  console.log("chat-attachments-behavior.test.ts passed");
});
