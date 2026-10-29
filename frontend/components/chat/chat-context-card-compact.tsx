import { MessageSquare } from "lucide-react";
import { cn } from "@/components/ui/cn";

const GENERAL_CHIPS = ["Продажи", "Остатки", "Реклама", "Аналитика", "Экспорт"] as const;

function ContextChip({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex h-[30px] items-center rounded-full border border-[#E2EAF5] bg-white px-3 text-[12px] font-medium text-[#0F172A] shadow-aivion-sm">
      {children}
    </span>
  );
}

export function ChatCompactGeneralContextCard({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[16px] border border-[#D6E6FA] bg-gradient-to-br from-white via-[#F8FBFF] to-[#EFF6FF] px-5 py-4 shadow-aivion-sm",
        className,
      )}
    >
      <div className="flex min-h-[76px] flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-aivion-blue text-white shadow-aivion-sm">
              <MessageSquare className="h-4 w-4" aria-hidden="true" />
            </div>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-[#0F172A]">AI-чат по кабинету</p>
              <p className="text-xs text-[#64748B]">Ответы на основе данных вашего кабинета</p>
            </div>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {GENERAL_CHIPS.map((chip) => (
            <ContextChip key={chip}>{chip}</ContextChip>
          ))}
        </div>
      </div>
    </div>
  );
}
