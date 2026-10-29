import assert from "node:assert/strict";
import type { ChatSession } from "@/lib/chat-api";
import {
  completenessWarningCode,
  formatFileSize,
  getAttachmentWarningLabel,
  getInitialSelectedSessionId,
  getAttachmentDisplayName,
  groupSessionsByDate,
  isActionSession,
  isCorruptedChatTitle,
  isSessionListItemSelected,
  mapAttachmentToDisplay,
  mapSessionToDisplay,
  resolveSessionTitle,
  splitActiveAndArchived,
} from "@/lib/chat-display";

assert.equal(getInitialSelectedSessionId(undefined), null);
assert.equal(getInitialSelectedSessionId(42), 42);

const actionSession: ChatSession = {
  id: 1,
  title: "Разберите 8 SKU без заказов",
  action_view_id: "view-stock-8sku",
  status: "active",
  created_at: "2026-06-30T10:00:00Z",
  updated_at: "2026-06-30T12:00:00Z",
  last_message_at: "2026-06-30T12:00:00Z",
};

assert.equal(isActionSession(actionSession), true);
const display = mapSessionToDisplay(actionSession);
assert.match(display.title, /Разберите 8 SKU/);
assert.equal(display.subtitle, "");

const grouped = groupSessionsByDate([actionSession]);
assert.equal(grouped.today.length + grouped.earlier.length, 1);

const split = splitActiveAndArchived([
  actionSession,
  { ...actionSession, id: 4, status: "archived" },
]);
assert.equal(split.active.length, 1);
assert.equal(split.archived.length, 1);

assert.equal(isSessionListItemSelected("existing_session", 1, 1), true);
assert.equal(isSessionListItemSelected("new_general", null, 1), false);

const attachment = mapAttachmentToDisplay({
  id: "att-1",
  file_name: "ABC_анализ_товарной_базы_30дней.xlsx",
  mime_type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  file_size: 25190,
  export_type: "abc_analysis",
  format: "xlsx",
  download_url: "/api/v1/chat/attachments/att-1/download",
  created_at: "2026-07-03T15:28:00Z",
});
assert.equal(attachment.kind, "xlsx");
assert.match(formatFileSize(25190), /24[,.]6 КБ/);
assert.equal(
  getAttachmentDisplayName({ name: "attachment_deadbeef.pdf", mimeType: "application/pdf" }),
  "Документ PDF",
);

assert.equal(isCorruptedChatTitle("??????? orders ? SKU-TEST-002?"), true);
assert.equal(resolveSessionTitle({ ...actionSession, title: "?????? SKU ? txt?" }), "Чат по действию");

assert.equal(getAttachmentWarningLabel("PARTIAL"), "Файл обработан частично");
assert.equal(getAttachmentWarningLabel("UNREADABLE"), "Файл не удалось прочитать");
assert.equal(getAttachmentWarningLabel("VISION_REQUIRED"), "Нужен просмотр изображения");
assert.equal(completenessWarningCode("COMPLETE"), null);
assert.equal(completenessWarningCode("PARTIAL"), "PARTIAL");

console.log("chat-display.test.ts: ok");
