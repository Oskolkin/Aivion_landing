import { MessageSquare } from "lucide-react";
import { ChatPromptChips } from "@/components/chat/chat-prompt-chips";
import { EMPTY_STATE_PROMPT_CHIPS } from "@/lib/chat-display";

export function ChatEmptyState({
  onChipClick,
  disabled,
}: {
  onChipClick: (chip: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="mx-auto w-full max-w-[680px] rounded-[20px] border border-dashed border-[#C7D9F2] bg-white/80 px-8 py-10 text-center">
      <div className="mx-auto mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-[#EFF6FF] text-aivion-blue">
        <MessageSquare className="h-5 w-5" aria-hidden="true" />
      </div>
      <h3 className="text-lg font-bold text-[#0F172A]">AI-чат по кабинету</h3>
      <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-[#64748B]">
        Спросите Aivion о продажах, остатках, рекламе или попросите подготовить файл.
      </p>
      <div className="mx-auto mt-6 max-w-lg text-left">
        <ChatPromptChips
          title="Популярные запросы"
          chips={EMPTY_STATE_PROMPT_CHIPS}
          disabled={disabled}
          onChipClick={onChipClick}
        />
      </div>
    </div>
  );
}
