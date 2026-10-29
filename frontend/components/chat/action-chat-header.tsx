import Link from "next/link";
import { ArrowRight, Sparkles } from "lucide-react";
import {
  ACTION_CHAT_DESCRIPTION,
  ACTION_CHAT_EYEBROW,
  ACTION_CHAT_OPEN_LABEL,
} from "@/components/chat/action-chat-copy";
import {
  UniversalActionIllustration,
  UniversalActionIllustrationSkeleton,
} from "@/components/chat/universal-action-illustration";
import { cn } from "@/components/ui/cn";
import type { ChatSessionDisplay } from "@/lib/chat-types";
import { mapSessionToDisplay } from "@/lib/chat-display";
import type { ChatSession } from "@/lib/chat-api";

function ActionChatUniversalIcon({ loading }: { loading?: boolean }) {
  return (
    <div
      className={cn(
        "flex size-[72px] shrink-0 items-center justify-center rounded-full border border-blue-200 bg-blue-50/70 xl:size-24",
        loading && "animate-pulse bg-blue-100/70",
      )}
      aria-hidden="true"
    >
      {!loading ? <Sparkles className="size-9 text-blue-600 xl:size-11" strokeWidth={1.75} /> : null}
    </div>
  );
}

function OpenActionButton({ actionViewId }: { actionViewId: string }) {
  return (
    <Link
      href={`/app/actions?action=${encodeURIComponent(actionViewId)}`}
      aria-label={ACTION_CHAT_OPEN_LABEL}
      className="mt-5 inline-flex w-full items-center justify-center gap-3 rounded-xl border border-blue-500 bg-white/70 px-5 py-3 text-sm font-semibold text-blue-600 transition hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-blue-100 sm:w-auto"
    >
      {ACTION_CHAT_OPEN_LABEL}
      <ArrowRight className="size-4 shrink-0" aria-hidden="true" />
    </Link>
  );
}

function ActionChatHeaderSkeleton({ className }: { className?: string }) {
  return (
    <section
      className={cn(
        "relative min-h-[190px] overflow-hidden rounded-[28px] border border-blue-200 bg-gradient-to-r from-white via-blue-50/30 to-blue-50/60 px-5 py-6 shadow-[0_12px_40px_rgba(37,99,235,0.05)] sm:px-7 sm:py-8 xl:min-h-[210px] xl:px-8",
        className,
      )}
      aria-busy="true"
      aria-label="Загрузка контекста действия"
    >
      <div className="grid grid-cols-1 items-center gap-6 xl:grid-cols-[auto_minmax(0,1fr)_300px] xl:gap-8">
        <ActionChatUniversalIcon loading />
        <div className="min-w-0 space-y-3">
          <div className="h-4 w-36 animate-pulse rounded-full bg-blue-100" />
          <div className="space-y-2">
            <div className="h-8 w-full max-w-[720px] animate-pulse rounded-lg bg-slate-100" />
            <div className="h-8 w-[82%] max-w-[620px] animate-pulse rounded-lg bg-slate-100" />
          </div>
          <div className="h-11 w-full max-w-[220px] animate-pulse rounded-xl bg-blue-100/80 sm:w-[220px]" />
          <div className="h-4 w-full max-w-[520px] animate-pulse rounded-full bg-slate-100" />
        </div>
        <UniversalActionIllustrationSkeleton />
      </div>
    </section>
  );
}

export function ActionChatHeader({
  title,
  actionViewId,
  loading,
  error,
  className,
}: {
  title: string;
  actionViewId?: string;
  loading?: boolean;
  error?: string | null;
  className?: string;
}) {
  if (loading) {
    return <ActionChatHeaderSkeleton className={className} />;
  }

  if (error) {
    return (
      <section
        className={cn(
          "rounded-[20px] border border-red-200 bg-red-50/80 px-5 py-4 text-sm text-red-700",
          className,
        )}
        role="alert"
      >
        <p>{error}</p>
        <Link href="/app/actions" className="mt-3 inline-flex font-medium text-red-800 underline">
          Вернуться к действиям
        </Link>
      </section>
    );
  }

  const resolvedTitle = title.trim() || ACTION_CHAT_EYEBROW;

  return (
    <section
      className={cn(
        "relative min-h-[190px] overflow-hidden rounded-[28px] border border-blue-200 bg-gradient-to-r from-white via-blue-50/30 to-blue-50/60 px-5 py-6 shadow-[0_12px_40px_rgba(37,99,235,0.05)] sm:px-7 sm:py-8 xl:min-h-[210px] xl:px-8",
        className,
      )}
    >
      <div className="grid grid-cols-1 items-center gap-6 xl:grid-cols-[auto_minmax(0,1fr)_300px] xl:gap-8">
        <ActionChatUniversalIcon />

        <div className="min-w-0">
          <p className="text-[15px] font-semibold text-aivion-blue sm:text-base">{ACTION_CHAT_EYEBROW}</p>
          <h2
            className="mt-2 line-clamp-2 max-w-[900px] text-[22px] font-bold leading-[1.18] tracking-tight text-[#0F172A] sm:text-[28px] xl:text-[34px]"
            title={resolvedTitle}
            aria-label={resolvedTitle}
          >
            {resolvedTitle}
          </h2>

          {actionViewId ? <OpenActionButton actionViewId={actionViewId} /> : null}

          <p className="mt-5 max-w-[850px] text-sm leading-relaxed text-[#64748B] sm:text-[15px] xl:mt-6">
            {ACTION_CHAT_DESCRIPTION}
          </p>
        </div>

        <UniversalActionIllustration />
      </div>
    </section>
  );
}

export function ChatActionContextCard({
  session,
  className,
  loading,
  error,
}: {
  session: ChatSession | ChatSessionDisplay;
  className?: string;
  loading?: boolean;
  error?: string | null;
}) {
  const rawSession = "raw" in session ? session.raw : session;
  const display = "raw" in session ? session : mapSessionToDisplay(rawSession);
  const actionViewId = display.actionViewId ?? rawSession.action_view_id ?? undefined;

  return (
    <ActionChatHeader
      className={className}
      title={display.title}
      actionViewId={actionViewId ?? undefined}
      loading={loading}
      error={error}
    />
  );
}
