import { Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { cn } from "@/components/ui/cn";

export function ChatCopyAction({ text, onError }: { text: string; onError?: (message: string) => void }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1800);
    return () => window.clearTimeout(timer);
  }, [copied]);

  async function handleCopy() {
    try {
      const plain = text.replace(/\*\*(.*?)\*\*/g, "$1").replace(/`([^`]+)`/g, "$1");
      await navigator.clipboard.writeText(plain);
      setCopied(true);
    } catch {
      onError?.("Не удалось скопировать текст.");
    }
  }

  return (
    <div className="mt-2 flex justify-end px-1">
      <button
        type="button"
        onClick={() => void handleCopy()}
        className={cn(
          "inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] text-[#64748B] transition-colors hover:text-aivion-blue",
        )}
      >
        <Copy className="h-3.5 w-3.5" aria-hidden="true" />
        {copied ? "Скопировано" : "Скопировать"}
      </button>
    </div>
  );
}
