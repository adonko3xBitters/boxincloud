"use client";

import Link from "next/link";
import { useState } from "react";

import { imageURL } from "@/lib/api/client";
import type { Comic } from "@/lib/api/client";
import { cx } from "./ui";

/**
 * Couverture d'album.
 *
 * L'élément le plus répété de l'application : une bibliothèque en affiche des
 * centaines. Deux conséquences dictent sa conception.
 *
 * D'abord le ratio est réservé avant le chargement, par `aspect-ratio`. Sans
 * cela la grille sautille au fur et à mesure que les images arrivent — le
 * défaut le plus visible d'une bibliothèque en ligne.
 *
 * Ensuite le survol reste sobre : une élévation et une ombre légèrement plus
 * marquée. Un effet appuyé, répété trois cents fois, fatigue.
 */

const COVER_RATIO = "0.7"; // largeur / hauteur, format album courant

export function Cover({
  comic,
  width = 320,
  className,
  priority = false,
}: {
  comic: Comic;
  /** Largeur demandée au serveur. Ramenée à 160, 320 ou 640 côté API. */
  width?: number;
  className?: string;
  /** Charge sans attendre le défilement — pour la première rangée seulement. */
  priority?: boolean;
}) {
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);

  return (
    <div
      className={cx(
        "relative overflow-hidden rounded-cover bg-surface-sunken",
        "shadow-[var(--shadow-cover)]",
        className,
      )}
      style={{ aspectRatio: COVER_RATIO }}
    >
      {!loaded && !failed && <div className="absolute inset-0 skeleton" aria-hidden="true" />}

      {failed ? (
        <FallbackCover comic={comic} />
      ) : (
        /* eslint-disable-next-line @next/next/no-img-element */
        <img
          src={imageURL(comic.coverPath, { width })}
          alt=""
          loading={priority ? "eager" : "lazy"}
          decoding="async"
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
          className={cx(
            "size-full object-cover",
            "transition-opacity duration-[--motion-duration-normal]",
            loaded ? "opacity-100" : "opacity-0",
          )}
        />
      )}
    </div>
  );
}

/**
 * Repli quand la couverture ne charge pas.
 *
 * Un rectangle vide laisserait croire à un bug. Afficher le titre garde la
 * grille lisible même si le backend de stockage est momentanément injoignable.
 */
function FallbackCover({ comic }: { comic: Comic }) {
  return (
    <div className="flex size-full flex-col items-center justify-center gap-2 bg-surface-raised p-3 text-center">
      <svg viewBox="0 0 24 24" fill="none" className="size-6 text-subtle" aria-hidden="true">
        <path
          d="M4 5.5A1.5 1.5 0 0 1 5.5 4H11v16H5.5A1.5 1.5 0 0 1 4 18.5v-13ZM13 4h5.5A1.5 1.5 0 0 1 20 5.5v13a1.5 1.5 0 0 1-1.5 1.5H13V4Z"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinejoin="round"
        />
      </svg>
      <span className="line-clamp-3 text-xs font-medium text-muted">{comic.title}</span>
    </div>
  );
}

/**
 * Carte d'album : couverture, titre, numéro, et progression le cas échéant.
 */
export function ComicCard({
  comic,
  width = 320,
  priority = false,
  progressPercent,
}: {
  comic: Comic;
  width?: number;
  priority?: boolean;
  /** Avancement de lecture, 0–100. Absent si l'album n'a pas été ouvert. */
  progressPercent?: number;
}) {
  return (
    <Link
      href={`/comic?id=${comic.id}`}
      className="group flex flex-col gap-2 focus-visible:outline-none"
      title={comic.title}
    >
      <div className="relative">
        <Cover
          comic={comic}
          width={width}
          priority={priority}
          className={cx(
            "transition-[transform,box-shadow] duration-[--motion-duration-normal]",
            "group-hover:-translate-y-1 group-hover:shadow-[var(--shadow-cover-hover)]",
            "group-focus-visible:-translate-y-1 group-focus-visible:ring-2 group-focus-visible:ring-accent",
          )}
        />

        {progressPercent !== undefined && progressPercent > 0 && (
          <ProgressBar percent={progressPercent} />
        )}

        {comic.state === "error" && (
          <span
            className="absolute right-1.5 top-1.5 rounded-full bg-danger px-1.5 py-0.5 text-[10px] font-medium text-white"
            title="Cet album n'a pas pu être indexé"
          >
            erreur
          </span>
        )}
      </div>

      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-fg group-hover:text-accent-text">
          {comic.title}
        </p>
        <p className="truncate text-xs text-muted">
          {comic.seriesName
            ? comic.number
              ? `${comic.seriesName} · ${comic.number}`
              : comic.seriesName
            : `${comic.pageCount} pages`}
        </p>
      </div>
    </Link>
  );
}

/** Barre de progression posée en pied de couverture. */
function ProgressBar({ percent }: { percent: number }) {
  const clamped = Math.min(100, Math.max(0, percent));

  return (
    <div
      className="absolute inset-x-0 bottom-0 h-1 bg-black/40"
      role="progressbar"
      aria-valuenow={Math.round(clamped)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label="Progression de lecture"
    >
      <div className="h-full bg-accent" style={{ width: `${clamped}%` }} />
    </div>
  );
}

/** Réserve la place d'une carte pendant le chargement. */
export function ComicCardSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      <div className="skeleton rounded-cover" style={{ aspectRatio: COVER_RATIO }} />
      <div className="flex flex-col gap-1.5">
        <div className="skeleton h-3.5 w-4/5 rounded" />
        <div className="skeleton h-3 w-3/5 rounded" />
      </div>
    </div>
  );
}
