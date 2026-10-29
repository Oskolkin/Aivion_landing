"use client";

import { useState } from "react";
import { Sparkles } from "lucide-react";
import { useRouter } from "next/navigation";
import { buttonClassNames } from "@/components/ui/button";
import { buildActionChatHref } from "@/lib/chat-action-handoff";
import { ACTION_ASK_PREFILL } from "@/lib/chat-view";

export function ActionAskAIButton({
  actionViewId,
  disabled,
  variant = "primary",
  label = "Спросить AI",
  hint,
}: {
  actionViewId: string;
  disabled?: boolean;
  variant?: "primary" | "secondary";
  label?: string;
  hint?: string;
}) {
  const router = useRouter();
  const [asking, setAsking] = useState(false);
  const [askError, setAskError] = useState<string | null>(null);

  async function handleAskAI() {
    if (asking || disabled) return;
    const id = actionViewId.trim();
    if (!id) {
      setAskError("Не указано действие");
      return;
    }
    setAsking(true);
    setAskError(null);
    try {
      router.push(buildActionChatHref({ actionViewId: id }));
    } catch (e: unknown) {
      setAskError(e instanceof Error ? e.message : "Не удалось открыть чат по действию");
    } finally {
      setAsking(false);
    }
  }

  return (
    <div className="space-y-1">
      <button
        type="button"
        data-testid="action-ask-ai"
        aria-label={label}
        className={buttonClassNames(
          variant,
          "inline-flex h-11 flex-col items-start justify-center gap-0 px-4",
        )}
        onClick={() => void handleAskAI()}
        disabled={asking || disabled}
      >
        <span className="inline-flex items-center gap-2 text-sm font-semibold leading-none">
          <Sparkles className="h-4 w-4" aria-hidden="true" />
          {asking ? "Открываем чат…" : label}
        </span>
        {hint && !asking ? (
          <span className="mt-1 hidden text-[11px] font-normal opacity-80 sm:inline">{hint}</span>
        ) : null}
      </button>
      {askError ? <p className="text-sm text-red-600">{askError}</p> : null}
    </div>
  );
}

export { ACTION_ASK_PREFILL };
