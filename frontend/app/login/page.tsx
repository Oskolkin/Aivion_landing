"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AuthLayout } from "@/components/auth-layout";
import { Button } from "@/components/ui/button";
import { FormField, Input } from "@/components/ui/input";
import { getPublicAuthConfig, login } from "@/lib/auth-api";
import { getPostLoginPath } from "@/lib/auth-redirect";
import { ui } from "@/lib/ui-copy";

export default function LoginPage() {
  const router = useRouter();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [registrationOpen, setRegistrationOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void getPublicAuthConfig()
      .then((cfg) => {
        if (!cancelled) setRegistrationOpen(Boolean(cfg.public_registration));
      })
      .catch(() => {
        if (!cancelled) setRegistrationOpen(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");

    try {
      setLoading(true);
      const auth = await login({ email, password });
      const path = getPostLoginPath(auth);
      if (!path) {
        setError("Для этого пользователя нужен аккаунт продавца");
        return;
      }
      router.push(path);
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось войти. Проверьте email и пароль.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout
      title={ui.login}
      subtitle="Войдите в Aivion, чтобы управлять продажами на Ozon"
      footer={
        registrationOpen ? (
          <>
            Нет аккаунта?{" "}
            <Link href="/register" className="font-medium text-aivion-blue hover:underline">
              {ui.register}
            </Link>
          </>
        ) : null
      }
    >
      <form onSubmit={handleSubmit} className="space-y-5">
        <FormField label={ui.email} htmlFor="email">
          <Input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="user@example.com"
            autoComplete="email"
            autoFocus
            required
          />
        </FormField>

        <FormField label={ui.password} htmlFor="password">
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </FormField>

        {error ? <p className="text-sm text-aivion-danger">{error}</p> : null}

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? ui.signingIn : ui.login}
        </Button>
      </form>
    </AuthLayout>
  );
}
