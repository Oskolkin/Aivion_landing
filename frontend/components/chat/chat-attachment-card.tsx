import { Download } from "lucide-react";
import { AttachmentTypeIcon } from "@/components/chat/attachment-type-icon";
import { cn } from "@/components/ui/cn";
import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";
import { getAttachmentVisualKind, getAttachmentWarningLabel } from "@/lib/chat-display";
import type { ChatAttachmentDisplay } from "@/lib/chat-types";

function resolveDownloadUrl(url?: string): string | undefined {
  if (!url) return undefined;
  if (url.startsWith("http")) return url;
  return `${PUBLIC_API_BASE_URL}${url}`;
}

export function ChatAttachmentCard({
  attachment,
  variant = "assistant",
}: {
  attachment: ChatAttachmentDisplay;
  variant?: "assistant" | "user" | "composer";
}) {
  const href = resolveDownloadUrl(attachment.downloadUrl);
  const visual = getAttachmentVisualKind({
    filename: attachment.name,
    format: attachment.visualLabel?.toLowerCase() ?? attachment.kind,
  });

  if (variant === "user") {
    return (
      <div className="flex min-h-[52px] items-center gap-2 rounded-[11px] border border-white/20 bg-white/12 px-2 py-1.5">
        <AttachmentTypeIcon visual={visual} size="sm" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-[13px] font-medium text-white" title={attachment.name}>
            {attachment.name}
          </p>
          {attachment.sizeLabel ? (
            <p className="text-[11px] text-white/75">{attachment.sizeLabel}</p>
          ) : null}
        </div>
      </div>
    );
  }

  const meta = [visual.label, attachment.sizeLabel].filter(Boolean).join(" · ");

  return (
    <div className="mt-3 flex flex-wrap items-center gap-2 rounded-[14px] border border-[#E2EAF5] bg-[#F8FAFC] px-3.5 py-3">
      <AttachmentTypeIcon visual={visual} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-aivion-text">{attachment.name}</p>
        {meta ? <p className="text-xs text-aivion-muted">{meta}</p> : null}
        {attachment.warningCode ? (
          <p className="mt-1 text-[11px] text-[#B45309]" role="status">
            {getAttachmentWarningLabel(attachment.warningCode) ?? attachment.warningCode}
          </p>
        ) : null}
      </div>
      {href ? (
        <a
          href={href}
          download={attachment.name}
          className={cn(
            "inline-flex shrink-0 items-center gap-1.5 rounded-full border border-[#E2EAF5] bg-white px-3 py-1.5 text-xs font-medium text-aivion-text transition-colors hover:border-aivion-blue hover:text-aivion-blue",
          )}
        >
          <Download className="h-3.5 w-3.5" aria-hidden="true" />
          Скачать файл
        </a>
      ) : (
        <span className="shrink-0 rounded-full border border-[#E2EAF5] bg-white px-3 py-1.5 text-xs text-aivion-muted">
          Файл готовится
        </span>
      )}
    </div>
  );
}
