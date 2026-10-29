import { BarChart3, LineChart, Sparkles } from "lucide-react";

export function GeneralChatAnalyticsIllustration() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none relative hidden h-[170px] w-[260px] shrink-0 xl:block"
    >
      <div className="absolute inset-0 rounded-[28px] bg-gradient-to-br from-blue-100/50 via-blue-50/30 to-transparent" />

      <div className="absolute left-2 top-2 grid grid-cols-4 grid-rows-4 gap-1.5 opacity-35">
        {Array.from({ length: 16 }).map((_, index) => (
          <span key={index} className="h-1 w-1 rounded-full bg-blue-300/80" />
        ))}
      </div>

      <div className="absolute left-6 top-8 w-[118px] rounded-2xl border border-white/90 bg-white/95 p-3 shadow-[0_10px_30px_rgba(37,99,235,0.12)]">
        <div className="mb-2 flex items-center justify-between">
          <span className="h-2 w-10 rounded-full bg-slate-200" />
          <LineChart className="h-4 w-4 text-blue-500/70" />
        </div>
        <svg viewBox="0 0 88 40" className="h-10 w-full" role="presentation">
          <path
            d="M4 32 L20 24 L36 28 L52 14 L68 18 L84 8"
            fill="none"
            stroke="#3B82F6"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <circle cx="52" cy="14" r="3.5" fill="#2563EB" />
        </svg>
      </div>

      <div className="absolute right-3 top-3 flex h-[74px] w-[92px] flex-col justify-end rounded-2xl border border-white/90 bg-white/95 p-2.5 shadow-[0_10px_28px_rgba(37,99,235,0.1)]">
        <BarChart3 className="absolute right-2.5 top-2.5 h-4 w-4 text-blue-500/65" />
        <div className="flex h-11 items-end gap-1.5">
          <span className="h-3 w-2.5 rounded-sm bg-blue-200" />
          <span className="h-5 w-2.5 rounded-sm bg-blue-400" />
          <span className="h-8 w-2.5 rounded-sm bg-blue-600" />
          <span className="h-4 w-2.5 rounded-sm bg-blue-300" />
        </div>
      </div>

      <div className="absolute bottom-5 left-10 flex h-[62px] w-[62px] items-center justify-center rounded-full border border-white/90 bg-white/95 shadow-[0_8px_24px_rgba(37,99,235,0.1)]">
        <svg viewBox="0 0 40 40" className="h-10 w-10" role="presentation">
          <circle cx="20" cy="20" r="14" fill="none" stroke="#BFDBFE" strokeWidth="6" />
          <circle
            cx="20"
            cy="20"
            r="14"
            fill="none"
            stroke="#2563EB"
            strokeWidth="6"
            strokeDasharray="44 44"
            strokeDashoffset="12"
            transform="rotate(-90 20 20)"
          />
        </svg>
      </div>

      <div className="absolute bottom-3 right-8 w-[72px] rounded-xl border border-white/90 bg-white/95 p-2.5 shadow-[0_8px_22px_rgba(37,99,235,0.08)]">
        <div className="space-y-1.5">
          <span className="block h-1.5 w-full rounded-full bg-slate-200" />
          <span className="block h-1.5 w-[85%] rounded-full bg-slate-200" />
          <span className="block h-1.5 w-[70%] rounded-full bg-blue-100" />
        </div>
      </div>

      <Sparkles className="absolute right-1 top-8 h-4 w-4 text-blue-400/80" />
      <Sparkles className="absolute bottom-10 right-1 h-3.5 w-3.5 text-blue-300/90" />
    </div>
  );
}
