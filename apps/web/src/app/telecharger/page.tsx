"use client";

import { useEffect, useState } from "react";

import { BrandLockup } from "@/components/brand";
import { cx } from "@/components/ui";
import { useT } from "@/i18n";
import {
  ANDROID_APK_URL,
  ANDROID_TEST_APK_URL,
  RELEASES_URL,
  detectPlatform,
  type Platform,
} from "@/lib/mobile-app";

/**
 * Page d'installation de l'application mobile.
 *
 * Elle ne demande pas de compte : on y arrive en scannant un code QR depuis un
 * téléphone qui, par construction, n'a encore rien installé ni ouvert de
 * session.
 *
 * Elle fait deux choses, et l'ordre compte. D'abord donner l'application.
 * Ensuite rappeler l'adresse du serveur — celle-là même par laquelle on est
 * arrivé, qu'il faudra saisir à la première connexion. Dicter une adresse au
 * téléphone est le seul moment pénible de l'installation ; l'écrire ici l'évite.
 */
export default function Page() {
  const t = useT();
  const [platform, setPlatform] = useState<Platform>("other");
  const [origin, setOrigin] = useState("");

  // Rien de tout cela n'existe au rendu statique : la page est exportée à la
  // construction, sans savoir sous quelle adresse ni sur quel appareil.
  useEffect(() => {
    setPlatform(detectPlatform(navigator.userAgent));
    setOrigin(window.location.origin);
  }, []);

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-md flex-col gap-8 px-5 py-12">
      <BrandLockup />

      <div>
        <h1 className="text-2xl font-semibold leading-tight text-fg">
          {t("download.title")}
        </h1>
        <p className="mt-2 leading-relaxed text-muted">
{t("download.subtitle")}
        </p>
      </div>

      {platform !== "ios" && (
        <section className="flex flex-col gap-3">
          <a
            href={ANDROID_APK_URL}
            className={cx(
              "pressable grid h-11 place-items-center rounded-lg bg-accent px-4",
              "font-semibold text-accent-fg",
            )}
          >
            {t("mobile.download")}
          </a>
          <p className="text-meta leading-relaxed text-subtle">
{t("download.androidWarning")}{" "}
            <a
              href={RELEASES_URL}
              className="text-accent-text underline underline-offset-2 hover:no-underline"
            >
              {t("download.allVersions")}
            </a>
            .
          </p>

          {/*
            Le lien de test est là parce qu'il doit y être tant qu'aucune clé
            de signature n'existe — mais en second, et en le disant. Un APK
            signé avec la clé de debug s'installe très bien ; il ne se met
            simplement jamais à jour vers la version signée, ce qui se découvre
            trop tard si personne ne l'a écrit.
          */}
          <details className="text-meta text-subtle">
            <summary className="cursor-pointer select-none hover:text-muted">
              {t("download.noFile")}
            </summary>
            <p className="mt-2 leading-relaxed">{t("download.noFileDetail")}</p>
            <a
              href={ANDROID_TEST_APK_URL}
              className="mt-1 inline-block text-accent-text underline underline-offset-2 hover:no-underline"
            >
              {t("download.testVersion")}
            </a>
          </details>
        </section>
      )}

      {platform !== "android" && (
        <section className="rounded-lg border border-border bg-surface-sunken p-4">
          <h2 className="font-semibold text-fg">{t("download.iosTitle")}</h2>
          <p className="mt-1 text-meta leading-relaxed text-muted">
{t("download.iosDetail")}
          </p>
        </section>
      )}

      <section className="flex flex-col gap-2">
        <h2 className="font-semibold text-fg">{t("mobile.serverAddress")}</h2>
        <p className="text-meta leading-relaxed text-muted">
{t("mobile.serverAddressHint")}
        </p>
        <ServerAddress origin={origin} />
      </section>
    </main>
  );
}

/**
 * L'adresse, avec un bouton pour la copier.
 *
 * La copie est le geste attendu : la recopier à la main sur un clavier tactile
 * est exactement ce qu'on cherche à éviter en scannant un code.
 */
function ServerAddress({ origin }: { origin: string }) {
  const t = useT();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return undefined;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <div className="flex items-center gap-2">
      <code className="flex-1 truncate rounded-md border border-border bg-surface-sunken px-3 py-2 text-ui text-fg">
        {origin || "…"}
      </code>
      <button
        disabled={!origin}
        onClick={() => {
          navigator.clipboard.writeText(origin).then(
            () => setCopied(true),
            // Le presse-papiers exige un contexte sécurisé : sur une instance en
            // HTTP simple, la copie échoue. L'adresse reste lisible et
            // sélectionnable, ce qui suffit.
            () => setCopied(false),
          );
        }}
        className={cx(
          "pressable h-10 shrink-0 rounded-md border border-border px-3 text-ui",
          "text-muted hover:bg-surface-hover hover:text-fg disabled:opacity-40",
        )}
      >
        {copied ? t("action.copied") : t("action.copy")}
      </button>
    </div>
  );
}
