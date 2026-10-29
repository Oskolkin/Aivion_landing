import { MessageCircle } from "lucide-react";
import { GeneralChatAnalyticsIllustration } from "@/components/chat/general-chat-analytics-illustration";
import {
  GENERAL_CHAT_DESCRIPTION,
  GENERAL_CHAT_EYEBROW,
  GENERAL_CHAT_TITLE,
  generalChatCapabilities,
} from "@/components/chat/general-chat-capabilities";
import { cn } from "@/components/ui/cn";

function GeneralChatHeaderIcon() {
  return (
    <div
      className="flex size-[72px] shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-blue-700 shadow-lg shadow-blue-200/50 xl:size-24"
      aria-hidden="true"
    >
      <div className="relative flex items-center justify-center">
        <MessageCircle className="size-9 text-white xl:size-11" strokeWidth={1.75} />
        <span className="absolute flex gap-[3px]">
          <span className="h-1 w-1 rounded-full bg-white/95" />
          <span className="h-1 w-1 rounded-full bg-white/95" />
          <span className="h-1 w-1 rounded-full bg-white/95" />
        </span>
      </div>
    </div>
  );
}

function GeneralChatCapabilityItem({
  label,
  icon: Icon,
}: {
  label: string;
  icon: (typeof generalChatCapabilities)[number]["icon"];
}) {
  return (
    <span className="inline-flex h-[40px] items-center gap-2 rounded-full border border-slate-200 bg-white/90 px-4 text-sm font-medium text-slate-600 shadow-sm">
      <Icon className="size-4 shrink-0 text-slate-500" aria-hidden="true" />
      {label}
    </span>
  );
}

function GeneralChatCapabilityList() {
  return (
    <div className="mt-5 flex flex-wrap gap-2.5">
      {generalChatCapabilities.map((item) => (
        <GeneralChatCapabilityItem key={item.label} label={item.label} icon={item.icon} />
      ))}
    </div>
  );
}

export function GeneralChatHeader({ className }: { className?: string }) {
  return (
    <section
      className={cn(
        "relative min-h-[180px] overflow-hidden rounded-[28px] border border-blue-200 bg-gradient-to-r from-white via-blue-50/40 to-blue-50/70 px-5 py-6 shadow-[0_12px_40px_rgba(37,99,235,0.06)] sm:px-7 sm:py-8 xl:min-h-[210px] xl:px-8",
        className,
      )}
    >
      <div className="grid grid-cols-1 items-center gap-6 xl:grid-cols-[auto_minmax(0,1fr)_280px] xl:gap-8">
        <GeneralChatHeaderIcon />

        <div className="min-w-0">
          <p className="text-[15px] font-semibold text-aivion-blue sm:text-base">{GENERAL_CHAT_EYEBROW}</p>
          <h2 className="mt-2 max-w-[900px] text-[22px] font-bold leading-[1.18] tracking-tight text-[#0F172A] sm:text-[28px] xl:text-[34px]">
            {GENERAL_CHAT_TITLE}
          </h2>
          <GeneralChatCapabilityList />
          <p className="mt-5 max-w-[760px] text-sm leading-relaxed text-[#64748B] sm:text-[15px] xl:mt-6">
            {GENERAL_CHAT_DESCRIPTION}
          </p>
        </div>

        <GeneralChatAnalyticsIllustration />
      </div>
    </section>
  );
}

/** @deprecated Use GeneralChatHeader */
export const ChatGeneralContextCard = GeneralChatHeader;
