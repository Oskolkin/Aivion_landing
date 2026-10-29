export function ChatMessagesSkeleton() {
  return (
    <div className="space-y-5" aria-busy="true" aria-label="Загрузка сообщений">
      <div className="flex justify-end">
        <div className="h-16 w-[45%] max-w-sm animate-pulse rounded-[16px_16px_4px_16px] bg-[#D6E6FA]/70" />
      </div>
      <div className="flex justify-start">
        <div className="h-24 w-[70%] max-w-xl animate-pulse rounded-[18px] bg-white shadow-aivion-sm" />
      </div>
      <div className="flex justify-end">
        <div className="h-12 w-[38%] max-w-xs animate-pulse rounded-[16px_16px_4px_16px] bg-[#D6E6FA]/70" />
      </div>
    </div>
  );
}
