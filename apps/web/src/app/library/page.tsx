"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { AppShell } from "@/components/app-shell";
import { ComicGrid, ComicGridSkeleton } from "@/components/comic-grid";
import { LibraryFilters, type ReadStatus, type SortOrder } from "@/components/filters";
import { EmptyState, ErrorState } from "@/components/ui";
import * as api from "@/lib/api/endpoints";

/**
 * Bibliothèque complète.
 *
 * Pagination par curseur chargée à l'approche du bas de page : un utilisateur
 * qui parcourt une collection ne doit jamais rencontrer de bouton « page
 * suivante », ni attendre.
 */
export default function LibraryPage() {
  return (
    <AppShell>
      <LibraryContent />
    </AppShell>
  );
}

function LibraryContent() {
  const [libraryId, setLibraryId] = useState<string | undefined>(undefined);
  const [readStatus, setReadStatus] = useState<ReadStatus>("");
  const [sort, setSort] = useState<SortOrder>("recent");

  const libraries = useQuery({
    queryKey: ["libraries"],
    queryFn: api.listLibraries,
  });

  const comics = useInfiniteQuery({
    // Les filtres font partie de la clé : changer de tri repart d'une page
    // vierge plutôt que de recoller des résultats d'ordres différents.
    queryKey: ["comics", { libraryId, readStatus, sort }],
    queryFn: ({ pageParam }) =>
      api.listComics({ libraryId, readStatus, sort, cursor: pageParam, limit: 60 }),
    initialPageParam: undefined as string | undefined,
    // nextCursor absent signifie « dernière page » : le contrat le garantit,
    // le client n'a pas à compter les éléments pour le déduire.
    getNextPageParam: (last) => last.nextCursor || undefined,
  });

  const all = useMemo(
    () => comics.data?.pages.flatMap((page) => page.items) ?? [],
    [comics.data],
  );

  if (comics.isError) {
    return <ErrorState error={comics.error} onRetry={() => void comics.refetch()} />;
  }

  const availableLibraries = libraries.data?.libraries ?? [];
  const filtered = readStatus !== "" || libraryId !== undefined;

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Bibliothèque</h1>
        {!comics.isLoading && (
          <p className="text-sm text-muted">
            {all.length} album{all.length > 1 ? "s" : ""}
            {comics.hasNextPage ? " et plus" : ""}
          </p>
        )}
      </div>

      <LibraryFilters
        libraries={availableLibraries}
        libraryId={libraryId}
        onLibraryChange={setLibraryId}
        readStatus={readStatus}
        onReadStatusChange={setReadStatus}
        sort={sort}
        onSortChange={setSort}
      />

      {comics.isLoading ? (
        <ComicGridSkeleton />
      ) : all.length === 0 ? (
        <EmptyState
          title={filtered ? "Aucun album ne correspond" : "Aucun album"}
          description={
            filtered
              ? "Élargissez les filtres pour voir plus de résultats."
              : "Cette bibliothèque ne contient rien pour le moment. Lancez un scan depuis le serveur."
          }
        />
      ) : (
        <ComicGrid
          comics={all}
          hasMore={comics.hasNextPage}
          loadingMore={comics.isFetchingNextPage}
          onLoadMore={() => void comics.fetchNextPage()}
        />
      )}
    </div>
  );
}
