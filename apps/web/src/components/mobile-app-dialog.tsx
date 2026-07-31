"use client";

import { useEffect, useMemo, useState } from "react";

import { encodeQR } from "@/lib/qr";
import { ANDROID_APK_URL } from "@/lib/mobile-app";

/**
 * Le code QR qui mène à l'application mobile.
 *
 * Il pointe vers une page de l'instance, pas directement vers l'APK. Deux
 * raisons : le téléphone qui scanne doit apprendre l'adresse du serveur, et
 * cette adresse c'est précisément celle qu'il vient de scanner — la page la
 * connaît sans qu'on ait à l'y écrire. Et un lien direct vers un fichier
 * téléchargerait sans rien expliquer, ce qui est le comportement d'une
 * publicité, pas d'une application.
 */
export function MobileAppDialog({ onClose }: { onClose: () => void }) {
  // L'origine n'existe pas au rendu statique : la page est exportée à la
  // construction, bien avant de savoir sous quelle adresse elle sera servie.
  const [origin, setOrigin] = useState("");

  useEffect(() => setOrigin(window.location.origin), []);

  const target = origin ? `${origin}/telecharger` : "";
  const qr = useMemo(() => (target ? encodeQR(target) : null), [target]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-[65] grid place-items-center bg-[var(--overlay)] p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Application mobile"
        onClick={(e) => e.stopPropagation()}
        className="rise-in flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-title font-semibold text-fg">Application mobile</h2>
            <p className="mt-0.5 text-meta text-muted">
              Scannez avec l&apos;appareil photo du téléphone.
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Fermer"
            className="pressable grid size-8 shrink-0 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        </div>

        {/*
          Fond blanc et marge, quel que soit le thème : un lecteur de code QR
          attend des modules sombres sur clair, et la marge fait partie de la
          spécification — sans elle, beaucoup d'appareils ne décodent pas.
        */}
        <div className="grid place-items-center rounded-lg bg-white p-4">
          {qr ? (
            <svg
              // Quatre modules de marge, comme l'exige la spécification : sous
              // cette valeur, une partie des lecteurs ne trouve plus les
              // repères d'alignement.
              viewBox={`-4 -4 ${qr.size + 8} ${qr.size + 8}`}
              className="size-56"
              role="img"
              aria-label={`Code QR vers ${target}`}
              shapeRendering="crispEdges"
            >
              <path d={qr.path} fill="#000" />
            </svg>
          ) : (
            <div className="size-56" />
          )}
        </div>

        <div className="flex flex-col gap-1">
          <p className="text-meta text-subtle">La page ouverte par le code</p>
          <code className="truncate rounded-md border border-border bg-surface-sunken px-2 py-1.5 text-meta text-muted">
            {target || "…"}
          </code>
        </div>

        <p className="text-meta leading-relaxed text-subtle">
          Elle propose le téléchargement pour Android et rappelle l&apos;adresse de
          ce serveur, à saisir à la première connexion.{" "}
          <a
            href={ANDROID_APK_URL}
            className="text-accent-text underline underline-offset-2 hover:no-underline"
          >
            Télécharger l&apos;APK directement
          </a>
          .
        </p>
      </div>
    </div>
  );
}
