import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ChatComposer, type ChatDraftAttachment } from "@/components/chat/chat-composer";
import {
  formatAttachedFilesCount,
  formatFileSize,
  getAttachmentDisplayName,
  getAttachmentVisualKind,
  isInternalAttachmentName,
} from "@/lib/chat-display";

// Helpers
assert.equal(formatAttachedFilesCount(1), "Прикреплён 1 файл");
assert.equal(formatAttachedFilesCount(2), "Прикреплено 2 файла");
assert.equal(formatAttachedFilesCount(5), "Прикреплено 5 файлов");
assert.equal(formatAttachedFilesCount(21), "Прикреплён 21 файл");

assert.equal(formatFileSize(68), "68 Б");
assert.match(formatFileSize(4300), /4[,.]2 КБ/);
assert.match(formatFileSize(287654), /280[,.]9 КБ/);
assert.match(formatFileSize(2516582), /2[,.]4 МБ/);

assert.equal(isInternalAttachmentName("attachment_5d465ce4efc1d4da.pdf"), true);
assert.equal(isInternalAttachmentName("Опрос.xlsx"), false);
assert.equal(
  getAttachmentDisplayName({ name: "attachment_5d465ce4efc1d4da.pdf", mimeType: "application/pdf" }),
  "Документ PDF",
);
assert.equal(getAttachmentDisplayName({ name: "Опрос.xlsx" }), "Опрос.xlsx");

assert.equal(getAttachmentVisualKind({ mimeType: "application/pdf" }).label, "PDF");
assert.equal(getAttachmentVisualKind({ filename: "data.csv" }).label, "CSV");
assert.equal(
  getAttachmentVisualKind({
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  }).label,
  "XLSX",
);
assert.equal(getAttachmentVisualKind({ filename: "note.txt" }).label, "TXT");
assert.equal(getAttachmentVisualKind({ filename: "doc.docx" }).label, "DOCX");

function makeAttachment(overrides: Partial<ChatDraftAttachment>): ChatDraftAttachment {
  return {
    localId: overrides.localId ?? "local-1",
    name: overrides.name ?? "Опрос.xlsx",
    sizeBytes: overrides.sizeBytes ?? 287654,
    mimeType: overrides.mimeType ?? "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    status: overrides.status ?? "uploaded",
    ...overrides,
  };
}

// Layout: 0 attachments
const emptyHtml = renderToStaticMarkup(
  <ChatComposer value="" onChange={() => undefined} onSubmit={() => undefined} />,
);
assert.doesNotMatch(emptyHtml, /Прикреплён|Прикреплено/);
assert.doesNotMatch(emptyHtml, /Удалить все/);
assert.match(emptyHtml, /Задайте вопрос о продажах/);
assert.match(emptyHtml, /Добавить файл/);
assert.doesNotMatch(emptyHtml, /из 5/);

// Layout: 1 attachment
const oneHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[makeAttachment({ localId: "a1" })]}
  />,
);
assert.match(oneHtml, /Прикреплён 1 файл/);
assert.doesNotMatch(oneHtml, /Удалить все/);
assert.match(oneHtml, /Опрос\.xlsx/);
assert.match(oneHtml, /1 из 5/);
assert.match(oneHtml, /Опишите, что нужно сделать с приложенными файлами/);

// Layout: 2 attachments
const twoHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[
      makeAttachment({ localId: "a1" }),
      makeAttachment({ localId: "a2", name: "data.csv", mimeType: "text/csv" }),
    ]}
    onRemoveAllAttachments={() => undefined}
  />,
);
assert.match(twoHtml, /Прикреплено 2 файла/);
assert.match(twoHtml, /Удалить все/);
assert.match(twoHtml, /md:grid-cols-2/);

// Layout: 5 attachments at limit
const fiveAttachments = Array.from({ length: 5 }, (_, i) =>
  makeAttachment({ localId: `a${i}`, name: `file-${i}.txt`, mimeType: "text/plain" }),
);
const fiveHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={fiveAttachments}
    onRemoveAllAttachments={() => undefined}
  />,
);
assert.match(fiveHtml, /Прикреплено 5 файлов/);
assert.match(fiveHtml, /5 из 5/);
assert.match(fiveHtml, /disabled/);

// File display: internal name hidden
const internalHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[
      makeAttachment({
        name: "attachment_5d465ce4efc1d4da.pdf",
        mimeType: "application/pdf",
      }),
    ]}
  />,
);
assert.match(internalHtml, /Документ PDF/);
assert.doesNotMatch(internalHtml, /attachment_5d465ce4efc1d4da/);

// Statuses: uploading
const uploadingHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[makeAttachment({ status: "uploading" })]}
  />,
);
assert.match(uploadingHtml, /Загрузка…/);

// Statuses: error
const errorHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[makeAttachment({ status: "error" })]}
    onRetryAttachment={() => undefined}
  />,
);
assert.match(errorHtml, /Не удалось загрузить/);
assert.match(errorHtml, /Повторить/);

// Statuses: warning
const warningHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[
      makeAttachment({
        warningCode: "spreadsheet_truncated_for_direct_ai_analysis",
      }),
    ]}
  />,
);
assert.match(warningHtml, /AI может использовать не все строки/);

// Composer: send disabled with empty text
const emptySendHtml = renderToStaticMarkup(
  <ChatComposer value="" onChange={() => undefined} onSubmit={() => undefined} />,
);
assert.match(emptySendHtml, /disabled/);

// Composer: send disabled while uploading
const sendUploadingHtml = renderToStaticMarkup(
  <ChatComposer
    value="Вопрос"
    onChange={() => undefined}
    onSubmit={() => undefined}
    attachments={[makeAttachment({ status: "uploading" })]}
  />,
);
assert.match(sendUploadingHtml, /disabled/);

// Accessibility
assert.match(emptyHtml, /aria-label="Прикрепить файл"/);
assert.match(emptyHtml, /aria-label="Отправить сообщение"/);
assert.match(oneHtml, /aria-label="Удалить файл Опрос\.xlsx"/);

// Unified card focus-within
assert.match(emptyHtml, /focus-within:border-blue-300/);

console.log("chat-composer.test.tsx passed");
