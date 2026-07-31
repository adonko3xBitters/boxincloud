"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { BrandLockup } from "@/components/brand";
import { Button, Input, Spinner } from "@/components/ui";
import { useT } from "@/i18n";
import { ApiError } from "@/lib/api/client";
import { useAuthStatus, useIsAuthenticated, useLogin } from "@/lib/auth";

export default function LoginPage() {
  const t = useT();
  const router = useRouter();
  const authenticated = useIsAuthenticated();
  const status = useAuthStatus();
  const login = useLogin();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Déjà connecté : on ne montre pas un formulaire de connexion inutile.
  useEffect(() => {
    if (authenticated) router.replace("/");
  }, [authenticated, router]);

  // Instance neuve : l'assistant d'installation prime sur la connexion.
  useEffect(() => {
    if (status.data?.needsSetup) router.replace("/setup");
  }, [status.data, router]);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      await login(username.trim(), password);
      router.replace("/");
    } catch (err) {
      // Le serveur répond volontairement la même chose pour un compte inconnu
      // et un mot de passe erroné : on ne cherche pas à en dire plus.
      setError(
        err instanceof ApiError && err.status === 401
          ? t("login.badCredentials")
          : t("login.failed"),
      );
      setSubmitting(false);
    }
  }

  if (status.isLoading) {
    return (
      <main className="grid min-h-dvh place-items-center">
        <Spinner className="size-6 text-muted" />
      </main>
    );
  }

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center">
          <BrandLockup />
        </div>

        <div className="rounded-xl border border-border bg-surface p-6 shadow-[var(--shadow-md)]">
          <h1 className="mb-1 text-xl font-semibold">{t("login.title")}</h1>
          <p className="mb-6 text-sm text-muted">{t("login.subtitle")}</p>

          <form onSubmit={onSubmit} className="flex flex-col gap-4">
            <Input
              name="username"
              label={t("auth.username")}
              autoComplete="username"
              autoFocus
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <Input
              name="password"
              type="password"
              label={t("auth.password")}
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />

            {error && (
              <p role="alert" className="rounded-md bg-danger/10 px-3 py-2 text-sm text-danger">
                {error}
              </p>
            )}

            <Button type="submit" size="lg" loading={submitting} className="mt-2">
              {t("auth.signIn")}
            </Button>
          </form>
        </div>
      </div>
    </main>
  );
}
