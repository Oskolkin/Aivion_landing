import { cn } from "@/components/ui/cn";

export function ChatPromptChips({
  title = "Попробуйте спросить",
  chips,
  disabled,
  onChipClick,
  className,
}: {
  title?: string;
  chips: readonly string[];
  disabled?: boolean;
  onChipClick: (chip: string) => void;
  className?: string;
}) {
  return (
    <div className={cn("space-y-2", className)}>
      <p className="text-xs font-medium text-aivion-muted">{title}</p>
      <div className="flex flex-wrap gap-2">
        {chips.map((chip) => (
          <button
            key={chip}
            type="button"
            disabled={disabled}
            onClick={() => onChipClick(chip)}
            className="inline-flex h-[36px] items-center rounded-full border border-[#E2EAF5] bg-white px-3.5 text-[12px] text-[#0F172A] transition-colors hover:border-aivion-blue hover:bg-[#F8FBFF] hover:text-aivion-blue disabled:opacity-50"
          >
            {chip}
          </button>
        ))}
      </div>
    </div>
  );
}
