"use client";

import { useEffect } from "react";

import { useT } from "@/i18n";

/*
Découvrir — la place gardée, la fonctionnalité retirée.

La recherche fédérée a été retirée du projet avant sa première version publique.
Ce fichier ne contient plus que la feuille et son raccourci.

# Pourquoi garder une porte qui n'ouvre sur rien

Parce qu'elle est déjà connue. Le raccourci figure dans la documentation et dans
les habitudes ; le supprimer ferait croire à une régression silencieuse — on
appuie, rien ne se passe, et on ne sait pas si c'est cassé ou volontaire.

Une feuille qui s'ouvre et dit « à venir » répond à la question posée. C'est
aussi la seule surface où l'annonce a une chance d'être lue par quelqu'un qui la
cherche.

Ce qui a disparu avec le reste : la recherche elle-même, les catalogues
distants, l'import, le moteur de gabarits, les bases de métadonnées et l'écran
de rapprochement. L'historique git les garde.
*/
export function DiscoverySheet({ onClose }: { onClose: () => void }) {
  const t = useT();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose]);

  return (
    /*
      `items-end` plutôt qu'un centrage : le panneau est ancré au bas de
      l'écran, d'où il vient. Le centrer annulerait le sens de son entrée.
    */
    <div className="fixed inset-0 z-[60] flex items-end bg-[var(--overlay)]">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("discovery.dialogLabel")}
        className="slide-in-bottom flex w-full flex-col overflow-hidden border-t border-border bg-surface shadow-2xl"
      >
        <header className="border-b border-border px-4 py-3">
          <div className="mx-auto flex w-full max-w-5xl items-center gap-3">
            <h2 className="text-title font-semibold text-fg">{t("discovery.title")}</h2>

            <span className="text-meta text-subtle">⌘⇧F</span>

            <button
              onClick={onClose}
              aria-label={t("action.close")}
              className="pressable ml-auto grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
            >
              <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
                <path
                  d="m4 4 8 8M12 4l-8 8"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                />
              </svg>
            </button>
          </div>
        </header>

        <div className="px-4 py-10">
          <div className="mx-auto flex w-full max-w-5xl flex-col items-center gap-2 text-center">
            <p className="text-ui font-medium text-fg">{t("discovery.soon")}</p>
            <p className="max-w-prose text-meta text-muted">{t("discovery.soonHint")}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
