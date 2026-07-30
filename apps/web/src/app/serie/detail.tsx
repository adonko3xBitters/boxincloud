"use client";

import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { ComicCard, ComicCardSkeleton } from "@/components/cover";
import { Badge, ErrorState, Skeleton } from "@/components/ui";
import * as api from "@/lib/api/endpoints";

/**
 * Détail d'une série : ses albums dans l'ordre de lecture.
 *
 * L'ordre vient du serveur (number_sort, puis titre) : le client ne retrie
 * rien, sans quoi les deux divergeraient sur les numéros exotiques — « HS »,
 * « 3.5 », « Tome 2 ».
 */
export function SeriesDetailView() {
  const seriesId = useSearchParams().get("id") ?? "";

  const query = useQuery({
    queryKey: ["series", seriesId],
    queryFn: () => api.getSeries(seriesId),
    enabled: Boolean(seriesId),
  });

  if (query.isError) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  }

  if (query.isLoading || !query.data) {
    return (
      <div className="flex flex-col gap-5">
        <Skeleton className="h-7 w-64" />
        <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}>
          {Array.from({ length: 12 }, (_, i) => <ComicCardSkeleton key={i} />)}
        </div>
      </div>
    );
  }

  const { series, comics } = query.data;

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight">{series.name}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Badge>{series.comicCount} album{series.comicCount > 1 ? "s" : ""}</Badge>
          {series.publisher && <Badge>{series.publisher}</Badge>}
        </div>
        {series.description && (
          <p className="mt-4 max-w-prose text-sm leading-relaxed text-muted">
            {series.description}
          </p>
        )}
      </header>

      <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}>
        {comics.map((comic, index) => (
          <ComicCard key={comic.id} comic={comic} width={320} priority={index < 12} />
        ))}
      </div>
    </div>
  );
}
