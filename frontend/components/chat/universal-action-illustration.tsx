import { CircleCheckBig, Sparkles } from "lucide-react";

function ChecklistRow({ checked }: { checked: boolean }) {
  return (
    <div className="flex items-center gap-3 border-b border-slate-100/80 px-5 py-3 last:border-b-0">
      <span
        className={
          checked
            ? "flex size-7 shrink-0 items-center justify-center rounded-full bg-blue-500 text-white"
            : "size-7 shrink-0 rounded-full border-2 border-slate-200 bg-white"
        }
      >
        {checked ? <CircleCheckBig className="size-4" strokeWidth={2.5} /> : null}
      </span>
      <div className="min-w-0 flex-1 space-y-1.5">
        <span
          className={`block h-2 rounded-full ${checked ? "w-[78%] bg-slate-300" : "w-[62%] bg-slate-200"}`}
        />
        {!checked ? <span className="block h-1.5 w-[42%] rounded-full bg-slate-100" /> : null}
      </div>
    </div>
  );
}

export function UniversalActionIllustration() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none relative hidden h-[170px] w-[280px] shrink-0 xl:block"
    >
      <div className="absolute inset-y-3 right-2 w-[220px] rounded-full bg-blue-100/55 blur-[0.5px]" />

      <div className="absolute left-7 top-3 grid grid-cols-4 grid-rows-4 gap-1.5 opacity-30">
        {Array.from({ length: 16 }).map((_, index) => (
          <span key={index} className="h-1 w-1 rounded-full bg-blue-300/80" />
        ))}
      </div>

      <div className="absolute left-8 top-4 overflow-hidden rounded-[24px] border border-blue-100 bg-white shadow-xl shadow-blue-100/50">
        <ChecklistRow checked />
        <ChecklistRow checked={false} />
        <ChecklistRow checked={false} />
      </div>

      <div className="absolute bottom-2 left-0 size-16 rounded-2xl bg-blue-200/70 shadow-lg shadow-blue-100/40" />

      <div className="absolute bottom-0 right-0 size-20 rounded-full border-[14px] border-blue-300/70 bg-transparent" />

      <Sparkles className="absolute right-2 top-6 h-4 w-4 text-blue-400/75" />
      <Sparkles className="absolute bottom-8 right-6 h-3.5 w-3.5 text-blue-300/85" />
    </div>
  );
}

export function UniversalActionIllustrationSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none relative hidden h-[170px] w-[280px] shrink-0 xl:block"
    >
      <div className="absolute left-8 top-4 h-[145px] w-[190px] animate-pulse rounded-[24px] bg-blue-100/60" />
      <div className="absolute bottom-2 left-0 size-16 animate-pulse rounded-2xl bg-blue-100/50" />
      <div className="absolute bottom-0 right-0 size-20 animate-pulse rounded-full border-[14px] border-blue-100/80" />
    </div>
  );
}
