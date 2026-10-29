import { Plus } from "lucide-react";
import { LoadingState } from "@/components/ui/loading-state";
import { ChatSessionItem } from "@/components/chat/chat-session-item";
import type { ChatSessionDisplay } from "@/lib/chat-types";
import type { SessionGroup } from "@/lib/chat-display";
import { isSessionListItemSelected } from "@/lib/chat-display";
import type { ChatViewMode } from "@/lib/chat-view";
import { cn } from "@/components/ui/cn";

function SessionGroupBlock({
  label,
  sessions,
  viewMode,
  selectedSessionId,
  onSelect,
  onArchive,
  onRename,
  onDelete,
  archivingSessionId,
  renamingSessionId,
  deletingSessionId,
}: {
  label: string;
  sessions: ChatSessionDisplay[];
  viewMode: ChatViewMode;
  selectedSessionId: number | null;
  onSelect: (id: number) => void;
  onArchive: (id: number) => void;
  onRename: (id: number, title: string) => void;
  onDelete: (id: number) => void;
  archivingSessionId: number | null;
  renamingSessionId: number | null;
  deletingSessionId: number | null;
}) {
  if (!sessions.length) return null;
  return (
    <div className="space-y-3">
      <p className="px-1 text-[11px] font-semibold uppercase tracking-wide text-[#64748B]">{label}</p>
      <ul className="space-y-2">
        {sessions.map((session) => (
          <li key={session.id}>
            <ChatSessionItem
              session={session}
              selected={isSessionListItemSelected(viewMode.kind, selectedSessionId, session.id)}
              onSelect={() => onSelect(session.id)}
              onArchive={() => onArchive(session.id)}
              onRename={(title) => onRename(session.id, title)}
              onDelete={() => onDelete(session.id)}
              archiving={archivingSessionId === session.id}
              renaming={renamingSessionId === session.id}
              deleting={deletingSessionId === session.id}
            />
          </li>
        ))}
      </ul>
    </div>
  );
}

export function ChatSessionList({
  groups,
  archivedCount,
  showArchived,
  onToggleArchived,
  viewMode,
  selectedSessionId,
  onNewChat,
  onSelect,
  onArchive,
  onRename,
  onDelete,
  archivingSessionId,
  renamingSessionId,
  deletingSessionId,
  loading,
  error,
}: {
  groups: SessionGroup;
  archivedCount: number;
  showArchived: boolean;
  onToggleArchived: () => void;
  viewMode: ChatViewMode;
  selectedSessionId: number | null;
  onNewChat: () => void;
  onSelect: (id: number) => void;
  onArchive: (id: number) => void;
  onRename: (id: number, title: string) => void;
  onDelete: (id: number) => void;
  archivingSessionId: number | null;
  renamingSessionId: number | null;
  deletingSessionId: number | null;
  loading: boolean;
  error: string | null;
}) {
  const hasSessions = groups.today.length > 0 || groups.earlier.length > 0;

  return (
    <aside className="flex h-full min-h-0 flex-col">
      <div className="mb-3 shrink-0 px-1">
        <h2 className="text-base font-semibold text-[#0F172A]">Чаты</h2>
      </div>

      <button
        type="button"
        onClick={onNewChat}
        className={cn(
          "mb-4 flex h-[40px] w-full shrink-0 items-center justify-center gap-2 rounded-[11px] border border-[#BFDBFE] bg-[#F8FBFF] text-sm font-medium text-aivion-blue transition-colors hover:bg-[#EFF6FF]",
        )}
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Новый чат
      </button>

      {loading ? <LoadingState variant="simple" message="Загрузка чатов…" /> : null}
      {error ? <p className="mb-2 text-sm text-aivion-danger">{error}</p> : null}

      {!loading && !hasSessions ? (
        <p className="rounded-[16px] border border-dashed border-[#E2EAF5] bg-white/60 px-4 py-6 text-center text-sm text-aivion-muted">
          Чатов пока нет. Задайте первый вопрос справа.
        </p>
      ) : (
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto pr-1">
          <SessionGroupBlock
            label="Сегодня"
            sessions={groups.today}
            viewMode={viewMode}
            selectedSessionId={selectedSessionId}
            onSelect={onSelect}
            onArchive={onArchive}
            onRename={onRename}
            onDelete={onDelete}
            archivingSessionId={archivingSessionId}
            renamingSessionId={renamingSessionId}
            deletingSessionId={deletingSessionId}
          />
          <SessionGroupBlock
            label="Ранее"
            sessions={groups.earlier}
            viewMode={viewMode}
            selectedSessionId={selectedSessionId}
            onSelect={onSelect}
            onArchive={onArchive}
            onRename={onRename}
            onDelete={onDelete}
            archivingSessionId={archivingSessionId}
            renamingSessionId={renamingSessionId}
            deletingSessionId={deletingSessionId}
          />
        </div>
      )}

      {archivedCount > 0 ? (
        <button
          type="button"
          onClick={onToggleArchived}
          className="mt-4 w-full rounded-full border border-transparent px-3 py-2 text-center text-xs text-aivion-muted transition-colors hover:border-[#E2EAF5] hover:bg-white hover:text-aivion-text"
        >
          {showArchived ? "Скрыть архив" : `Показать архив (${archivedCount})`}
        </button>
      ) : null}
    </aside>
  );
}
