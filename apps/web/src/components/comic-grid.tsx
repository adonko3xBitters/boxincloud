"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { ComicCard, ComicCardSkeleton } from "./cover";
import { Spinner } from "./ui";
import type { Comic } from "@/lib/api/client";

/**
 * Grille virtualisée de couvertures.
 *
 * Au-delà de quelques centaines d'albums, monter un nœud DOM par couverture
 * fait chuter le défilement — c'est le point où une bibliothèque « qui marche »
 * devient une bibliothèque agréable. Seules les rangées visibles sont rendues.
 *
 * La virtualisation porte sur les rangées, pas sur chaque carte : une rangée
 * est l'unité naturelle de la mise en page, et cela divise par le nombre de
 * colonnes le travail de mesure.
 */

const GAP = 16; // doit correspondre à --space-4
const MIN_COLUMN = 140;
const MAX_COLUMN = 190;
const COVER_RATIO = 0.7;
const LABEL_HEIGHT = 44; // titre + sous-titre sous la couverture

export function ComicGrid({
  comics,
  hasMore,
  loadingMore,
  onLoadMore,
  progressByComic,
}: {
  comics: Comic[];
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  progressByComic?: Map<string, number>;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [columns, setColumns] = useState(6);
  const [columnWidth, setColumnWidth] = useState(MIN_COLUMN);

  // Le nombre de colonnes suit la largeur disponible, sans point de rupture
  // fixe : une grille qui s'adapte en continu évite les rangées à moitié vides
  // sur les largeurs intermédiaires.
  const measure = useCallback(() => {
    const width = containerRef.current?.clientWidth ?? 0;
    if (width === 0) return;

    const count = Math.max(2, Math.floor((width + GAP) / (MIN_COLUMN + GAP)));
    const available = (width - GAP * (count - 1)) / count;

    setColumns(count);
    setColumnWidth(Math.min(MAX_COLUMN, available));
  }, []);

  useEffect(() => {
    measure();
    const observer = new ResizeObserver(measure);
    if (containerRef.current) observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [measure]);

  const rowCount = Math.ceil(comics.length / columns);
  const rowHeight = columnWidth / COVER_RATIO + LABEL_HEIGHT + GAP;

  const virtualizer = useVirtualizer({
    count: rowCount,
    // La fenêtre entière défile : c'est ce qui garde l'en-tête collant et la
    // barre de défilement du navigateur, plutôt qu'une zone interne qui
    // casserait les deux.
    getScrollElement: () => (typeof window === "undefined" ? null : document.body),
    estimateSize: () => rowHeight,
    overscan: 3,
    scrollMargin: containerRef.current?.offsetTop ?? 0,
  });

  // Chargement de la page suivante à l'approche du bas. Le seuil est en
  // rangées plutôt qu'en pixels : la page arrive avant que l'utilisateur ne
  // voie le vide, quelle que soit la taille des couvertures.
  const virtualRows = virtualizer.getVirtualItems();
  const lastVisibleRow = virtualRows.at(-1)?.index ?? 0;

  useEffect(() => {
    if (!hasMore || loadingMore || !onLoadMore) return;
    if (lastVisibleRow >= rowCount - 3) onLoadMore();
  }, [lastVisibleRow, rowCount, hasMore, loadingMore, onLoadMore]);

  return (
    <div ref={containerRef}>
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualRows.map((virtualRow) => {
          const start = virtualRow.index * columns;
          const rowComics = comics.slice(start, start + columns);

          return (
            <div
              key={virtualRow.key}
              className="absolute left-0 top-0 grid w-full"
              style={{
                transform: `translateY(${virtualRow.start - virtualizer.options.scrollMargin}px)`,
                gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                gap: GAP,
              }}
            >
              {rowComics.map((comic) => (
                <ComicCard
                  key={comic.id}
                  comic={comic}
                  width={columnWidth > 170 ? 640 : 320}
                  // Les deux premières rangées se chargent sans attendre : ce
                  // sont celles que l'utilisateur voit à l'arrivée.
                  priority={virtualRow.index < 2}
                  progressPercent={progressByComic?.get(comic.id)}
                />
              ))}
            </div>
          );
        })}
      </div>

      {loadingMore && (
        <div className="flex justify-center py-8">
          <Spinner className="size-5 text-muted" />
        </div>
      )}
    </div>
  );
}

/** Grille de squelettes, aux mêmes dimensions que la grille réelle. */
export function ComicGridSkeleton({ count = 18 }: { count?: number }) {
  return (
    <div
      className="grid gap-4"
      style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${MIN_COLUMN}px, 1fr))` }}
    >
      {Array.from({ length: count }, (_, i) => (
        <ComicCardSkeleton key={i} />
      ))}
    </div>
  );
}
