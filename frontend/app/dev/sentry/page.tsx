"use client";

export default function DevSentryPage() {
  return (
    <main className="min-h-screen p-8">
      <div className="mx-auto max-w-4xl">
        <h1 className="text-3xl font-bold">Тест Sentry</h1>
        <button
          className="mt-6 rounded bg-black px-4 py-2 text-white"
          onClick={() => {
            throw new Error("Тестовая ошибка фронтенда для Sentry");
          }}
        >
          Вызвать ошибку на фронтенде
        </button>
      </div>
    </main>
  );
}