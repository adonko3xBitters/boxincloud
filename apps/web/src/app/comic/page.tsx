"use client";

import { Suspense } from "react";

import { AppShell } from "@/components/app-shell";
import { Spinner } from "@/components/ui";
import { ComicDetailView } from "./detail";

/**
 * Détail d'un album : `/comic?id=…`
 *
 * Identifiant en paramètre de requête plutôt qu'en segment de chemin.
 *
 * L'export statique de Next exige de connaître toutes les routes au build, et
 * les identifiants d'albums n'existent qu'à l'exécution — un segment
 * `[comicId]` casserait donc la compilation. Le paramètre de requête donne des
 * URL tout aussi partageables et met en favori sans faire de compromis sur
 * l'artefact unique (ADR-003).
 */
export default function Page() {
  return (
    <AppShell>
      <Suspense fallback={<Spinner className="mx-auto mt-12 size-6 text-muted" />}>
        <ComicDetailView />
      </Suspense>
    </AppShell>
  );
}
