"use client";

/**
 * Assistant de première installation.
 *
 * C'est le tout premier écran qu'un utilisateur voit après avoir lancé
 * `docker compose up`. Il détermine si l'impression laissée est « ça a l'air
 * sérieux » ou « encore un projet self-hosted austère » — et il mérite donc un
 * vrai soin.
 *
 * M3 en couvre la première étape : créer l'administrateur. Connecter un backend
 * de stockage et créer une bibliothèque arrivent avec la console
 * d'administration, en M7 ; en attendant, l'assistant renvoie explicitement
 * vers `boxincloudctl` plutôt que de laisser l'utilisateur sur une
 * bibliothèque vide sans explication.
 */

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { BrandLockup } from "@/components/brand";
import { Button, Input, Spinner } from "@/components/ui";
import { useT } from "@/i18n";
import { ApiError } from "@/lib/api/client";
import { setTokens } from "@/lib/api/tokens";
import * as api from "@/lib/api/endpoints";
import { useAuthStatus, useIsAuthenticated } from "@/lib/auth";

const MIN_PASSWORD = 10;

export default function SetupPage() {
  const t = useT();
  const router = useRouter();
  const status = useAuthStatus();
  const authenticated = useIsAuthenticated();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  // L'installation est déjà faite : la porte est fermée côté serveur, autant
  // ne pas afficher un formulaire qui échouerait.
  useEffect(() => {
    if (status.data && !status.data.needsSetup && !done) {
      router.replace(authenticated ? "/" : "/login");
    }
  }, [status.data, authenticated, done, router]);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setFieldErrors({});

    if (password !== confirm) {
      setFieldErrors({ confirm: t("setup.mismatch") });
      return;
    }

    setSubmitting(true);
    try {
      await api.setup({
        username: username.trim(),
        email: email.trim() || undefined,
        password,
      });

      // On enchaîne sur une connexion : demander à l'utilisateur de ressaisir
      // ce qu'il vient de taper serait absurde.
      const tokens = await api.login({ username: username.trim(), password });
      setTokens(tokens);
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError) {
        setFieldErrors(err.fieldErrors);
        if (Object.keys(err.fieldErrors).length === 0) {
          setError(err.message);
        }
      } else {
        setError(t("setup.unreachable"));
      }
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

  if (done) {
    return <SetupComplete onContinue={() => router.replace("/")} />;
  }

  const passwordTooShort = password.length > 0 && password.length < MIN_PASSWORD;

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-12">
      <div className="w-full max-w-md">
        <div className="mb-8 flex justify-center">
          <BrandLockup />
        </div>

        <div className="rounded-xl border border-border bg-surface p-6 shadow-[var(--shadow-md)]">
          <h1 className="mb-1 text-xl font-semibold">{t("setup.welcome")}</h1>
          <p className="mb-6 text-sm text-muted">
{t("setup.intro")}
          </p>

          <form onSubmit={onSubmit} className="flex flex-col gap-4">
            <Input
              name="username"
              label={t("auth.username")}
              autoComplete="username"
              autoFocus
              required
              minLength={3}
              maxLength={32}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              error={fieldErrors.username}
              hint={t("setup.usernameHint")}
            />
            <Input
              name="email"
              type="email"
              label={t("setup.emailOptional")}
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={fieldErrors.email}
            />
            <Input
              name="password"
              type="password"
              label={t("auth.password")}
              autoComplete="new-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={fieldErrors.password ?? (passwordTooShort ? `Au moins ${MIN_PASSWORD} caractères.` : undefined)}
              hint={`Au moins ${MIN_PASSWORD} caractères. La longueur compte plus que la complexité.`}
            />
            <Input
              name="confirm"
              type="password"
              label={t("setup.confirm")}
              autoComplete="new-password"
              required
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              error={fieldErrors.confirm}
            />

            {error && (
              <p role="alert" className="rounded-md bg-danger/10 px-3 py-2 text-sm text-danger">
                {error}
              </p>
            )}

            <Button
              type="submit"
              size="lg"
              className="mt-2"
              loading={submitting}
              disabled={password.length < MIN_PASSWORD}
            >
              Créer le compte
            </Button>
          </form>
        </div>

        <p className="mt-6 text-center text-xs text-subtle">
{t("setup.willBeAdmin")}
        </p>
      </div>
    </main>
  );
}

/**
 * Écran de fin.
 *
 * L'utilisateur vient de créer son compte et arriverait sur une bibliothèque
 * vide sans explication. On lui dit explicitement ce qui reste à faire, avec la
 * commande exacte — plutôt que de le laisser chercher.
 */
function SetupComplete({ onContinue }: { onContinue: () => void }) {
  const t = useT();
  return (
    <main className="grid min-h-dvh place-items-center px-4 py-12">
      <div className="w-full max-w-lg">
        <div className="mb-8 flex justify-center">
          <BrandLockup />
        </div>

        <div className="rounded-xl border border-border bg-surface p-6 shadow-[var(--shadow-md)]">
          <div className="mb-4 flex size-10 items-center justify-center rounded-full bg-success/15 text-success">
            <svg viewBox="0 0 20 20" fill="currentColor" className="size-5" aria-hidden="true">
              <path
                fillRule="evenodd"
                d="M16.7 5.3a1 1 0 0 1 0 1.4l-7.5 7.5a1 1 0 0 1-1.4 0l-3.5-3.5a1 1 0 1 1 1.4-1.4l2.8 2.8 6.8-6.8a1 1 0 0 1 1.4 0Z"
                clipRule="evenodd"
              />
            </svg>
          </div>

          <h1 className="mb-1 text-xl font-semibold">{t("setup.created")}</h1>
          <p className="mb-6 text-sm text-muted">
{t("setup.nextSteps")}
          </p>

          <div className="rounded-lg bg-surface-sunken p-4">
            <p className="mb-2 text-xs font-medium uppercase tracking-wide text-subtle">
              {t("setup.fromServer")}
            </p>
            <pre className="overflow-x-auto text-xs leading-relaxed text-fg">
              <code>{`boxincloudctl storage add monminio s3 \\
    endpoint=localhost:9000 bucket=comics \\
    access_key=… secret_key=… path_style=true

boxincloudctl library add BD monminio bd/
boxincloudctl scan-now BD`}</code>
            </pre>
          </div>

          <Button size="lg" className="mt-6 w-full" onClick={onContinue}>
            Continuer
          </Button>
        </div>
      </div>
    </main>
  );
}
