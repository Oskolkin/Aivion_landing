import { CheckCheck } from "lucide-react";
import { ChatAttachmentCard } from "@/components/chat/chat-attachment-card";
import { formatMessageTime } from "@/lib/chat-display";
import type { ChatAttachmentDisplay } from "@/lib/chat-types";

export function ChatUserMessage({
  content,
  createdAt,
  attachments,
}: {
  content: string;
  createdAt: string;
  attachments?: ChatAttachmentDisplay[];
}) {
  return (
    <div className="flex w-full justify-end">
      <div className="max-w-[min(65%,calc(100vw-2.5rem))] min-w-[120px]">
        <div className="mb-1 flex items-center justify-end gap-2 text-[11px] text-[#64748B]">
          <time dateTime={createdAt}>{formatMessageTime(createdAt)}</time>
        </div>
        <div className="rounded-[16px_16px_4px_16px] bg-aivion-blue px-4 py-3 text-sm leading-relaxed text-white shadow-aivion-sm">
          {attachments && attachments.length > 0 ? (
            <div className="space-y-2.5">
              {attachments.map((att) => (
                <ChatAttachmentCard key={att.id ?? att.name} attachment={att} variant="user" />
              ))}
            </div>
          ) : null}
          {content.trim() ? (
            <p className={attachments && attachments.length > 0 ? "mt-3 whitespace-pre-wrap" : "whitespace-pre-wrap"}>
              {content}
            </p>
          ) : null}
          <div className="mt-1.5 flex justify-end">
            <CheckCheck className="h-3.5 w-3.5 text-blue-100/90" aria-hidden="true" />
          </div>
        </div>
      </div>
    </div>
  );
}
