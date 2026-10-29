import { clientEnv } from "@/lib/env/client";

export default function DevSystemPage() {
  return (
    <main className="min-h-screen p-8">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-3xl font-bold">Система (dev)</h1>

        <div className="mt-6 space-y-2">
          <p>
            <span className="font-semibold">Фронтенд:</span> работает
          </p>
          <p>
            <span className="font-semibold">Базовый URL API:</span>{" "}
            {clientEnv.apiBaseUrl}
          </p>
        </div>
      </div>
    </main>
  );
}