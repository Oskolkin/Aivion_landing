import { BarChart3, FileText, PieChart, Sparkles } from "lucide-react";

export function GeneralChatHeroIllustration() {
  return (
    <div className="relative hidden h-[130px] w-[148px] shrink-0 md:block" aria-hidden="true">
      <div className="absolute inset-0 rounded-[22px] bg-gradient-to-br from-[#DBEAFE]/50 to-[#EFF6FF]/30" />
      <div className="absolute right-2 top-3 flex h-[72px] w-[88px] flex-col justify-end rounded-xl border border-white/80 bg-white/90 p-2 shadow-aivion-md">
        <div className="flex h-10 items-end gap-1">
          <span className="h-3 w-2 rounded-sm bg-[#93C5FD]" />
          <span className="h-5 w-2 rounded-sm bg-[#60A5FA]" />
          <span className="h-7 w-2 rounded-sm bg-aivion-blue" />
          <span className="h-4 w-2 rounded-sm bg-[#BFDBFE]" />
        </div>
        <BarChart3 className="absolute right-2 top-2 h-4 w-4 text-aivion-blue/70" />
      </div>
      <div className="absolute bottom-4 left-1 flex h-14 w-14 items-center justify-center rounded-full border border-white/90 bg-white/85 shadow-aivion-sm">
        <PieChart className="h-6 w-6 text-aivion-blue/75" />
      </div>
      <div className="absolute bottom-2 right-12 flex h-10 w-10 items-center justify-center rounded-lg border border-white/80 bg-white/75 shadow-aivion-sm">
        <FileText className="h-4 w-4 text-aivion-blue/65" />
      </div>
      <Sparkles className="absolute right-0 top-1 h-4 w-4 text-[#60A5FA]/80" />
      <Sparkles className="absolute bottom-10 right-1 h-3 w-3 text-[#93C5FD]/90" />
    </div>
  );
}
