import { GeneralChatHeader } from "@/components/chat/general-chat-header";
import { ActionChatHeader } from "@/components/chat/action-chat-header";
import { ChatCompactGeneralContextCard } from "@/components/chat/chat-context-card-compact";
import type { ChatSessionDisplay } from "@/lib/chat-types";
import { isActionSession, mapSessionToDisplay } from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";

export { GeneralChatHeader, ChatGeneralContextCard } from "@/components/chat/general-chat-header";
export { ActionChatHeader, ChatActionContextCard } from "@/components/chat/action-chat-header";
export { ChatCompactGeneralContextCard } from "@/components/chat/chat-context-card-compact";

export function ChatContextCard({
  session,
  variant = "hero",
  className,
  actionHeaderLoading,
  actionHeaderError,
}: {
  session: ChatSession | ChatSessionDisplay | null;
  variant?: "hero" | "compact";
  className?: string;
  actionHeaderLoading?: boolean;
  actionHeaderError?: string | null;
}) {
  if (!session) return <GeneralChatHeader className={className} />;
  const raw = "raw" in session ? session.raw : session;
  if (isActionSession(raw)) {
    const display = "raw" in session ? session : mapSessionToDisplay(raw);
    const actionViewId = display.actionViewId ?? raw.action_view_id ?? undefined;
    return (
      <ActionChatHeader
        className={className}
        title={display.title}
        actionViewId={actionViewId ?? undefined}
        loading={actionHeaderLoading}
        error={actionHeaderError}
      />
    );
  }
  if (variant === "compact") {
    return <ChatCompactGeneralContextCard className={className} />;
  }
  return <GeneralChatHeader className={className} />;
}
