export function ActionsLoadingState() {
  return (
    <div className="space-y-4" aria-busy="true" aria-live="polite">
      <div className="h-[88px] animate-pulse rounded-[16px] bg-slate-100" />

      <div className="flex flex-wrap gap-2">
        {Array.from({ length: 5 }).map((_, index) => (
          <div key={index} className="h-9 w-28 animate-pulse rounded-full bg-slate-100" />
        ))}
        <div className="ml-auto h-10 w-56 animate-pulse rounded-full bg-slate-100" />
      </div>

      <div className="flex flex-wrap justify-end gap-2">
        <div className="h-10 w-72 animate-pulse rounded-xl bg-slate-100" />
        <div className="h-10 w-48 animate-pulse rounded-xl bg-slate-100" />
      </div>

      <div className="grid min-h-[620px] gap-4 xl:grid-cols-[minmax(340px,380px)_minmax(0,1fr)]">
        <div className="flex min-h-0 flex-col gap-2">
          <div className="h-5 w-28 animate-pulse rounded bg-slate-200" />
          {Array.from({ length: 4 }).map((_, index) => (
            <div key={index} className="h-28 animate-pulse rounded-[14px] bg-slate-100" />
          ))}
        </div>
        <div className="flex min-h-0 flex-col overflow-hidden rounded-[20px] border border-slate-200 bg-white">
          <div className="space-y-3 border-b border-slate-200 p-5">
            <div className="flex gap-2">
              <div className="h-5 w-24 animate-pulse rounded-full bg-slate-100" />
              <div className="h-5 w-28 animate-pulse rounded-full bg-slate-100" />
              <div className="h-5 w-20 animate-pulse rounded-full bg-slate-100" />
            </div>
            <div className="h-8 w-3/4 animate-pulse rounded bg-slate-200" />
            <div className="h-4 w-1/2 animate-pulse rounded bg-slate-100" />
          </div>
          <div className="flex-1 space-y-4 p-5">
            <div className="grid gap-2.5 sm:grid-cols-3 xl:grid-cols-5">
              {Array.from({ length: 5 }).map((_, index) => (
                <div key={index} className="h-20 animate-pulse rounded-[12px] bg-slate-100" />
              ))}
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              <div className="h-40 animate-pulse rounded-[14px] bg-slate-100" />
              <div className="h-40 animate-pulse rounded-[14px] bg-slate-100" />
            </div>
          </div>
          <div className="border-t border-slate-200 p-4">
            <div className="flex gap-2">
              <div className="h-11 w-48 animate-pulse rounded-aivion-md bg-slate-100" />
              <div className="h-11 w-36 animate-pulse rounded-aivion-md bg-slate-100" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
