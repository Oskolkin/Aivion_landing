"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AivionMark } from "@/components/brand/aivion-logo";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AlertBanner, ErrorState } from "@/components/ui/error-state";
import { LoadingState } from "@/components/ui/loading-state";
import { PageHeader } from "@/components/ui/page-header";
import { PUBLIC_API_BASE_URL } from "@/lib/env/api-base-url";
import { disconnectOzonCabinet, getOzonUserConnection, type OzonUserConnectionDto } from "@/lib/ozon-api";

const DISCONNECT_PHRASE = "ОТКЛЮЧИТЬ";

export default function OzonOnboarding() {
  const router = useRouter();
  const [connection, setConnection] = useState<OzonUserConnectionDto | null>(null);
  const [loading, setLoading] = useState(true);
  const [disconnectOpen, setDisconnectOpen] = useState(false);
  const [disconnectPhrase, setDisconnectPhrase] = useState("");
  const [disconnecting, setDisconnecting] = useState(false);
  const [error, setError] = useState("");
  const isConnected = connection?.connection_ready === true;

  useEffect(() => {
    getOzonUserConnection()
      .then(setConnection)
      .catch((err) => setError(err instanceof Error ? err.message : "Не удалось загрузить состояние подключения"))
      .finally(() => setLoading(false));
  }, []);

  function startOAuth() {
    window.location.assign(`${PUBLIC_API_BASE_URL}/oauth/start`);
  }

  async function handleDisconnect() {
    setError("");
    if (disconnectPhrase.trim() !== DISCONNECT_PHRASE) {
      setError(`Введите слово «${DISCONNECT_PHRASE}» для подтверждения отключения`);
      return;
    }
    try {
      setDisconnecting(true);
      const result = await disconnectOzonCabinet(disconnectPhrase.trim());
      setConnection(result.connection);
      setDisconnectOpen(false);
      setDisconnectPhrase("");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось отключить кабинет");
    } finally {
      setDisconnecting(false);
    }
  }

  if (loading) return <div className="py-4"><LoadingState message="Загрузка…" /></div>;

  return (
    <div className="mx-auto max-w-2xl space-y-6 p-6">
      {!isConnected ? <div className="flex justify-center pb-2"><AivionMark size="large" /></div> : null}
      <PageHeader
        title={isConnected ? "Настройки подключения Ozon" : "Подключение кабинета Ozon"}
        subtitle="Безопасная авторизация выполняется на стороне Ozon. Aivion не запрашивает и не показывает API-ключи кабинета."
      />
      {connection?.status === "disconnected_retention" && connection.retention_delete_after ? (
        <AlertBanner tone="warning" message={`Кабинет отключён. Данные будут храниться до ${new Date(connection.retention_delete_after).toLocaleString("ru-RU")}.`} />
      ) : null}
      {error ? <ErrorState title="Не удалось выполнить действие" message={error} /> : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Private OAuth Ozon</CardTitle>
          <CardDescription>Один seller-bound доступ используется сервером Aivion для разрешённых Seller API и Performance API запросов.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-aivion-muted">
            {isConnected && connection?.authentication_mode === "PRIVATE_OAUTH"
              ? "Кабинет подключён через Private OAuth. Токены хранятся в зашифрованном виде."
              : "Нажмите кнопку и подтвердите доступ в Ozon."}
          </p>
          <Button type="button" variant="primary" onClick={startOAuth}>
            {isConnected ? "Переподключить через Ozon" : "Подключить через Ozon"}
          </Button>
        </CardContent>
      </Card>

      {isConnected ? (
        <Card className="border-aivion-border">
          <CardHeader>
            <CardTitle className="text-lg text-red-900">Отключить кабинет Ozon</CardTitle>
            <CardDescription>Авторизация будет отозвана в Aivion, но seller account и канонические данные не удаляются. Применяется действующая retention policy.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {!disconnectOpen ? (
              <Button type="button" variant="secondary" onClick={() => setDisconnectOpen(true)}>Отключить кабинет Ozon</Button>
            ) : (
              <>
                <p className="text-sm text-aivion-text">Введите слово <strong>{DISCONNECT_PHRASE}</strong> для подтверждения.</p>
                <input className="w-full max-w-xs rounded-md border border-aivion-border px-3 py-2 text-sm" value={disconnectPhrase} onChange={(event) => setDisconnectPhrase(event.target.value)} placeholder={DISCONNECT_PHRASE} />
                <div className="flex flex-wrap gap-3">
                  <Button type="button" variant="secondary" disabled={disconnecting} onClick={() => void handleDisconnect()}>{disconnecting ? "Отключение…" : "Подтвердить отключение"}</Button>
                  <Button type="button" variant="secondary" onClick={() => { setDisconnectOpen(false); setDisconnectPhrase(""); }}>Отмена</Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
