import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";

function renderInline(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  const regex = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let key = 0;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index));
    }
    const token = match[0];
    if (token.startsWith("**")) {
      parts.push(
        <strong key={key++} className="font-bold text-aivion-text">
          {token.slice(2, -2)}
        </strong>,
      );
    } else if (token.startsWith("`")) {
      parts.push(
        <code
          key={key++}
          className="rounded-md bg-[#F1F5F9] px-1.5 py-0.5 text-[0.85em] text-aivion-text"
        >
          {token.slice(1, -1)}
        </code>,
      );
    }
    lastIndex = match.index + token.length;
  }

  if (lastIndex < text.length) parts.push(text.slice(lastIndex));
  return parts;
}

function isBulletLine(line: string): boolean {
  const trimmed = line.trim();
  return /^[-*•✓✔]\s/.test(trimmed) || /^✓/.test(trimmed);
}

function bulletContent(line: string): string {
  return line.trim().replace(/^[-*•✓✔]\s*/, "");
}

export function ChatMarkdown({ content, className }: { content: string; className?: string }) {
  const lines = content.split("\n");
  const blocks: ReactNode[] = [];
  let listItems: string[] = [];
  let blockKey = 0;

  function flushList() {
    if (!listItems.length) return;
    blocks.push(
      <ul key={`list-${blockKey++}`} className="my-2 space-y-1.5 pl-1">
        {listItems.map((item, index) => (
          <li key={index} className="flex gap-2 text-sm leading-relaxed text-aivion-text">
            <span className="mt-0.5 text-[#12b76a]" aria-hidden="true">
              ✓
            </span>
            <span>{renderInline(item)}</span>
          </li>
        ))}
      </ul>,
    );
    listItems = [];
  }

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) {
      flushList();
      continue;
    }

    if (isBulletLine(trimmed)) {
      listItems.push(bulletContent(trimmed));
      continue;
    }

    flushList();

    if (trimmed.startsWith("### ")) {
      blocks.push(
        <h4 key={`h-${blockKey++}`} className="mb-2 mt-4 text-sm font-bold text-aivion-text">
          {renderInline(trimmed.slice(4))}
        </h4>,
      );
      continue;
    }

    if (trimmed.startsWith("## ")) {
      blocks.push(
        <h3 key={`h-${blockKey++}`} className="mb-2 mt-4 text-base font-bold text-aivion-text">
          {renderInline(trimmed.slice(3))}
        </h3>,
      );
      continue;
    }

    blocks.push(
      <p key={`p-${blockKey++}`} className="mb-2 text-sm leading-relaxed text-aivion-text last:mb-0">
        {renderInline(trimmed)}
      </p>,
    );
  }

  flushList();

  return <div className={cn("chat-markdown", className)}>{blocks}</div>;
}
