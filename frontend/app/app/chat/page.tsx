import { Suspense } from "react";
import ChatScreen from "@/components/chat-screen";
import { getCurrentUserServer } from "@/lib/server-auth";

export default async function ChatPage({
  searchParams,
}: {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = (await searchParams) ?? {};
  const rawSession = Array.isArray(params.session) ? params.session[0] : params.session;
  const parsed = Number.parseInt((rawSession ?? "").trim(), 10);
  const initialSessionId = Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;

  const rawView = Array.isArray(params.action_view_id) ? params.action_view_id[0] : params.action_view_id;
  const initialActionViewId = (rawView ?? "").trim() || undefined;
  const initialDraftMode = Boolean(initialActionViewId);

  const auth = await getCurrentUserServer();
  const isLiveReview = Boolean(auth?.is_live_review);

  return (
    <Suspense fallback={null}>
      <ChatScreen
        initialSessionId={initialSessionId}
        initialActionViewId={initialActionViewId}
        initialDraftMode={initialDraftMode}
        isLiveReview={isLiveReview}
      />
    </Suspense>
  );
}
