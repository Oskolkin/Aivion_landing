import { AlertTriangle } from "lucide-react";
import type { ChatMessage } from "@/lib/chat-api";
import { ChatAttachmentCard } from "@/components/chat/chat-attachment-card";
import { ChatCopyAction } from "@/components/chat/chat-copy-action";
import { ChatMarkdown } from "@/components/chat/chat-markdown";
import { formatMessageTime, isControlledFallbackMessage, mapAttachmentToDisplay } from "@/lib/chat-display";

export type MessageMeta = {
  limitations: string[];
};

export function ChatAssistantMessage({
  message,
  meta,
  onCopyError,
}: {
  message: ChatMessage;
  meta?: MessageMeta;
  onCopyError?: (message: string) => void;
}) {
  const attachments = message.attachments ?? [];
  const limitations = meta?.limitations ?? [];
  const isControlledFallback = isControlledFallbackMessage(message.content, limitations);

  return (
    <div className="flex w-full justify-start" data-testid="chat-assistant-message">
      <div className="w-full max-w-[780px]">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-aivion-blue text-xs font-bold text-white">
            A
          </div>
          <span className="text-sm font-semibold text-aivion-text">Aivion</span>
          <time className="text-xs text-aivion-muted" dateTime={message.created_at}>
            {formatMessageTime(message.created_at)}
          </time>
        </div>

        {isControlledFallback ? (
          <article className="rounded-[18px] border border-[#F5E0B3] bg-[#FFFBF0] px-5 py-[18px] shadow-aivion-sm">
            <div className="flex items-start gap-3">
              <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-[#B45309]" aria-hidden="true" />
              <div className="min-w-0 space-y-2">
                <p className="text-sm font-semibold text-[#92400E]">
                  Не удалось подготовить подтверждённый ответ
                </p>
                <p className="text-sm leading-relaxed text-[#78350F]">
                  AI не смог надёжно связать вывод с данными кабинета. Уточните вопрос, период или
                  конкретный показатель.
                </p>
              </div>
            </div>
          </article>
        ) : (
          <article className="rounded-[18px] border border-[#E6EEF8] bg-white px-5 py-[18px] shadow-aivion-sm">
            <ChatMarkdown content={message.content} />

            {limitations.length > 0 ? (
              <p className="mt-3 rounded-xl border border-[#F5E0B3] bg-[#FFFBF0] px-3 py-2 text-sm text-[#78350F]" role="note">
                {limitations[0]}
              </p>
            ) : null}

            {attachments.length > 0 ? (
              <div className="mt-3 space-y-3">
                {attachments.map((att) => (
                  <ChatAttachmentCard key={att.id} attachment={mapAttachmentToDisplay(att)} />
                ))}
              </div>
            ) : null}
          </article>
        )}

        {message.message_type === "answer" ? (
          <ChatCopyAction text={message.content} onError={onCopyError} />
        ) : null}
      </div>
    </div>
  );
}
