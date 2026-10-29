import { MoreHorizontal } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { cn } from "@/components/ui/cn";
import type { ChatSessionDisplay } from "@/lib/chat-types";
import { formatSessionTime } from "@/lib/chat-display";

export function ChatSessionItem({
  session,
  selected,
  onSelect,
  onArchive,
  onRename,
  onDelete,
  archiving,
  renaming,
  deleting,
}: {
  session: ChatSessionDisplay;
  selected: boolean;
  onSelect: () => void;
  onArchive?: () => void;
  onRename?: (title: string) => void;
  onDelete?: () => void;
  archiving?: boolean;
  renaming?: boolean;
  deleting?: boolean;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState(session.title);
  const menuRef = useRef<HTMLDivElement>(null);
  const hasMenu = Boolean(onArchive || onRename || onDelete || session.actionViewId);

  useEffect(() => {
    if (!menuOpen) return;
    function handleClick(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [menuOpen]);

  function openRenameDialog() {
    setRenameValue(session.title);
    setRenameOpen(true);
    setMenuOpen(false);
  }

  function submitRename() {
    const trimmed = renameValue.trim();
    if (!trimmed || trimmed.length > 80) return;
    onRename?.(trimmed);
    setRenameOpen(false);
    setMenuOpen(false);
  }

  return (
    <div
      className={cn(
        "group relative rounded-[12px] border px-3 py-2.5 transition-all",
        selected
          ? "border-aivion-blue bg-[#EFF6FF] shadow-aivion-sm"
          : "border-[#E2EAF5] bg-white hover:border-aivion-blue/40",
      )}
    >
      <button type="button" className="w-full text-left" onClick={onSelect}>
        <div className="flex items-start justify-between gap-2 pr-5">
          <p className="line-clamp-2 flex-1 text-[13px] font-medium leading-snug text-[#0F172A]">
            {session.title}
          </p>
          <time
            className="shrink-0 pt-0.5 text-[11px] text-[#64748B]"
            dateTime={session.lastMessageAt ?? undefined}
          >
            {formatSessionTime(session.lastMessageAt)}
          </time>
        </div>
      </button>

      {hasMenu ? (
        <div ref={menuRef} className="absolute right-1.5 top-2">
          <button
            type="button"
            aria-label="Меню чата"
            className={cn(
              "rounded-md p-1 text-[#64748B] transition-colors hover:bg-white hover:text-[#0F172A]",
              selected || menuOpen ? "opacity-100" : "opacity-0 group-hover:opacity-100",
            )}
            onClick={(e) => {
              e.stopPropagation();
              setMenuOpen((open) => !open);
            }}
          >
            <MoreHorizontal className="h-4 w-4" />
          </button>
          {menuOpen ? (
            <div className="absolute right-0 z-10 mt-1 min-w-[168px] rounded-xl border border-[#E2EAF5] bg-white py-1 shadow-aivion-md">
              {session.actionViewId ? (
                <Link
                  href={`/app/actions?action=${encodeURIComponent(session.actionViewId)}`}
                  className="block w-full px-3 py-2 text-left text-xs text-[#0F172A] hover:bg-aivion-surface-soft"
                  onClick={() => setMenuOpen(false)}
                >
                  Открыть действие
                </Link>
              ) : null}
              {onRename ? (
                <button
                  type="button"
                  disabled={renaming}
                  className="w-full px-3 py-2 text-left text-xs text-[#0F172A] hover:bg-aivion-surface-soft disabled:opacity-50"
                  onClick={openRenameDialog}
                >
                  Переименовать
                </button>
              ) : null}
              {onArchive && session.raw.status !== "archived" ? (
                <button
                  type="button"
                  disabled={archiving}
                  className="w-full px-3 py-2 text-left text-xs text-[#0F172A] hover:bg-aivion-surface-soft disabled:opacity-50"
                  onClick={() => {
                    setMenuOpen(false);
                    onArchive();
                  }}
                >
                  {archiving ? "Архивация…" : "Архивировать"}
                </button>
              ) : null}
              {onDelete ? (
                <button
                  type="button"
                  disabled={deleting}
                  className="w-full px-3 py-2 text-left text-xs text-red-600 hover:bg-red-50 disabled:opacity-50"
                  onClick={() => {
                    setMenuOpen(false);
                    if (window.confirm("Удалить чат и всю историю сообщений?")) {
                      onDelete();
                    }
                  }}
                >
                  {deleting ? "Удаление…" : "Удалить"}
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}

      {renameOpen ? (
        <div
          className="absolute inset-x-2 bottom-2 z-20 rounded-lg border border-[#E2EAF5] bg-white p-2 shadow-aivion-md"
          onClick={(e) => e.stopPropagation()}
        >
          <input
            type="text"
            maxLength={80}
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            className="mb-2 w-full rounded-md border border-[#E2EAF5] px-2 py-1 text-xs"
            autoFocus
          />
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="rounded-md px-2 py-1 text-xs text-[#64748B]"
              onClick={() => setRenameOpen(false)}
            >
              Отмена
            </button>
            <button
              type="button"
              disabled={renaming || !renameValue.trim()}
              className="rounded-md bg-aivion-blue px-2 py-1 text-xs text-white disabled:opacity-50"
              onClick={submitRename}
            >
              {renaming ? "Сохранение…" : "Сохранить"}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
