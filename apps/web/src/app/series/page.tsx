"use client";

import Link from "next/link";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { AppShell } from "@/components/app-shell";
import { EmptyState, ErrorState, Skeleton } from "@/components/ui";
import { imageURL } from "@/lib/api/client";
import type { Series } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";

/**
 * Liste des séries.
 *
 * Une collection de BD se pense en séries bien plus qu'en albums isolés :
 * c'est souvent l'entrée la plus naturelle dans la bibliothèque.
 */
export default function SeriesPage() {
  return (
    <AppShell>
      <SeriesList />
    </AppShell>
  );
}

function SeriesList() {
  const series = useInfiniteQuery({
    queryKey: ["series"],
    queryFn: ({ pageParam }) => api.listSeries({ cursor: pageParam, limit: 60 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor || undefined,
  });

  const all = useMemo(
    () => series.data?.pages.flatMap((page) => page.items) ?? [],
    [series.data],
  );

  if (series.isError) {
    return <ErrorState error={series.error} onRetry={() => void series.refetch()} />;
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Séries</h1>
        {!series.isLoading && (
          <p className="text-sm text-muted">{all.length} série{all.length > 1 ? "s" : ""}</p>
        )}
      </div>

      {series.isLoading ? (
        <SeriesGridSkeleton />
      ) : all.length === 0 ? (
        <EmptyState
          title="Aucune série"
          description="Les séries sont déduites des métadonnées ou du nom des fichiers lors du scan."
        />
      ) : (
        <>
          <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}>
            {all.map((item) => (
              <SeriesCard key={item.id} series={item} />
            ))}
          </div>

          {series.hasNextPage && (
            <button
              onClick={() => void series.fetchNextPage()}
              disabled={series.isFetchingNextPage}
              className="mx-auto rounded-md border border-border px-4 py-2 text-sm text-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-50"
            >
              {series.isFetchingNextPage ? "Chargement…" : "Charger la suite"}
            </button>
          )}
        </>
      )}
    </div>
  );
}

function SeriesCard({ series }: { series: Series }) {
  return (
    <Link href={`/serie?id=${series.id}`} className="group flex flex-col gap-2" title={series.name}>
      <div
        className="relative overflow-hidden rounded-cover bg-surface-sunken shadow-[var(--shadow-cover)] transition-[transform,box-shadow] duration-[--motion-duration-normal] group-hover:-translate-y-1 group-hover:shadow-[var(--shadow-cover-hover)]"
        style={{ aspectRatio: 0.7 }}
      >
        {series.coverPath ? (
          /* eslint-disable-next-line @next/next/no-img-element */
          <img
            src={imageURL(series.coverPath, { width: 320 })}
            alt=""
            loading="lazy"
            decoding="async"
            className="size-full object-cover"
          />
        ) : (
          <div className="grid size-full place-items-center text-xs text-subtle">
            {series.name}
          </div>
        )}

        {/* Le nombre de tomes est l'information qui distingue une série d'un
            album : elle est posée sur la couverture pour rester lisible même
            quand les titres sont tronqués. */}
        <span className="absolute bottom-1.5 right-1.5 rounded-full bg-black/65 px-2 py-0.5 text-[11px] font-medium text-white">
          {series.comicCount}
        </span>
      </div>

      <p className="truncate text-sm font-medium text-fg group-hover:text-accent-text">
        {series.name}
      </p>
    </Link>
  );
}

function SeriesGridSkeleton() {
  return (
    <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}>
      {Array.from({ length: 18 }, (_, i) => (
        <div key={i} className="flex flex-col gap-2">
          <div className="skeleton rounded-cover" style={{ aspectRatio: 0.7 }} />
          <Skeleton className="h-3.5 w-4/5" />
        </div>
      ))}
    </div>
  );
}
