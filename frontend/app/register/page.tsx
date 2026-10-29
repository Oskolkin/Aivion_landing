"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { AuthLayout } from "@/components/auth-layout";
import { Button } from "@/components/ui/button";
import { FormField, Input } from "@/components/ui/input";
import { getPublicAuthConfig, register } from "@/lib/auth-api";
import { ui } from "@/lib/ui-copy";

export default function RegisterPage() {
  const router = useRouter();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [validationError, setValidationError] = useState("");
  const [registrationOpen, setRegistrationOpen] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    void getPublicAuthConfig()
      .then((cfg) => {
        if (cancelled) return;
        if (!cfg.public_registration) {
          router.replace("/login");
          return;
        }
        setRegistrationOpen(true);
      })
      .catch(() => {
        if (!cancelled) router.replace("/login");
      });
    return () => {
      cancelled = true;
    };
  }, [router]);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setValidationError("");

    if (!email.trim()) {
      setValidationError("Укажите email");
      return;
    }
    if (password.length < 8) {
      setValidationError("Пароль должен быть не короче 8 символов");
      return;
    }
    if (password !== passwordConfirm) {
      setValidationError("Пароли не совпадают");
      return;
    }

    try {
      setLoading(true);
      await register({ email, password, password_confirm: passwordConfirm });
      router.push("/app");
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Не удалось зарегистрироваться");
    } finally {
      setLoading(false);
    }
  }

  if (registrationOpen !== true) {
    return (
      <AuthLayout title={ui.register} subtitle="Проверяем доступность регистрации…">
        <p className="text-sm text-aivion-muted">Перенаправление…</p>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      title={ui.register}
      subtitle="Создайте аккаунт Aivion для аналитики и AI-рекомендаций"
      footer={
        <>
          Уже есть аккаунт?{" "}
          <Link href="/login" className="font-medium text-aivion-blue hover:underline">
            {ui.login}
          </Link>
        </>
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
            required
          />
        </FormField>

        <FormField label={ui.password} htmlFor="password" hint="Минимум 8 символов">
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            required
          />
        </FormField>

        <FormField label="Подтверждение пароля" htmlFor="passwordConfirm">
          <Input
            id="passwordConfirm"
            type="password"
            value={passwordConfirm}
            onChange={(e) => setPasswordConfirm(e.target.value)}
            autoComplete="new-password"
            required
          />
        </FormField>

        {validationError ? <p className="text-sm text-aivion-danger">{validationError}</p> : null}
        {error ? <p className="text-sm text-aivion-danger">{error}</p> : null}

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? ui.creatingAccount : ui.register}
        </Button>
      </form>
    </AuthLayout>
  );
}
