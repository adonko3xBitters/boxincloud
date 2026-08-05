"use client";

/**
 * Ossature commune aux panneaux du module.
 *
 * Les huit panneaux répondent tous à la même question — « que dit le démon
 * sur ce point » — et rencontrent donc tous les mêmes trois échecs : la
 * réponse n'est pas encore là, elle est vide, elle n'est pas venue. Écrire la
 * réponse une fois évite que le panneau Serveurs affiche une erreur en carte
 * rouge pendant que le panneau Envois n'affiche rien du tout.
 */

import type { ReactNode } from "react";
import type { UseQueryResult } from "@tanstack/react-query";

import { EmptyState, ErrorState, Spinner } from "@/components/ui";
import { useT } from "@/i18n";
import { useEd2kFormat } from "./format";

/**
 * En-tête d'un panneau.
 *
 * `takenAt` est affiché systématiquement quand il existe. Le serveur rend le
 * dernier instantané scruté, pas une mesure prise à la demande : sans cette
 * heure, des chiffres figés depuis dix minutes parce que le démon ne répond
 * plus ont exactement la même tête que des chiffres frais.
 */
export function PanelHeader({
  title,
  hint,
  takenAt,
  children,
}: {
  title: string;
  hint?: string;
  takenAt?: string;
  children?: ReactNode;
}) {
  const t = useT();
  const format = useEd2kFormat();

  return (
    <header className="mb-3 flex flex-wrap items-baseline gap-x-3 gap-y-1">
      <h2 className="text-title font-semibold text-fg">{title}</h2>
      {takenAt && (
        <span className="text-micro tabular-nums text-subtle">
          {t("ed2k.takenAt", { time: format.time(takenAt) })}
        </span>
      )}
      {children}
      {hint && <p className="w-full max-w-prose text-meta text-muted">{hint}</p>}
    </header>
  );
}

/**
 * Les trois états d'une requête, rendus au même endroit.
 *
 * `isEmpty` est passé par l'appelant plutôt que déduit : « vide » ne veut pas
 * dire la même chose pour une file de téléchargement (aucune ligne) et pour un
 * panneau d'envois (ni transfert ni file d'attente), et deviner produirait un
 * état vide affiché par-dessus des données bien présentes.
 */
export function Async<T>({
  query,
  isEmpty,
  empty,
  children,
}: {
  query: UseQueryResult<T>;
  isEmpty: (data: T) => boolean;
  empty: { title: string; description: string; action?: ReactNode };
  children: (data: T) => ReactNode;
}) {
  if (query.isPending) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner className="size-5 text-muted" />
      </div>
    );
  }

  if (query.isError || query.data === undefined) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  }

  if (isEmpty(query.data)) {
    return (
      <EmptyState
        title={empty.title}
        description={empty.description}
        action={empty.action}
      />
    );
  }

  return <>{children(query.data)}</>;
}

/**
 * Cadence de rafraîchissement des panneaux.
 *
 * Le flux SSE ne porte aujourd'hui que l'état du module, pas le contenu des
 * tableaux : ces derniers se rafraîchissent donc en redemandant l'instantané.
 * Deux secondes, parce que c'est l'ordre de grandeur de la scrutation côté
 * serveur — interroger plus vite rendrait deux fois le même instantané, et
 * plus lentement donnerait des débits qui traînent derrière la réalité.
 *
 * Le jour où le flux portera un événement par domaine, cette constante
 * disparaîtra au profit d'une écriture directe dans le cache : c'est le même
 * mécanisme, la même clé de requête, et aucun panneau n'aura à changer.
 */
export const REFRESH_MS = 2000;
