"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { AppShell } from "@/components/app-shell";
import { ComicGrid, ComicGridSkeleton } from "@/components/comic-grid";
import { EmptyState, ErrorState, cx } from "@/components/ui";
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

  const libraries = useQuery({
    queryKey: ["libraries"],
    queryFn: api.listLibraries,
  });

  const comics = useInfiniteQuery({
    queryKey: ["comics", { libraryId }],
    queryFn: ({ pageParam }) =>
      api.listComics({ libraryId, cursor: pageParam, limit: 60 }),
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

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Bibliothèque</h1>
          {!comics.isLoading && (
            <p className="text-sm text-muted">
              {all.length} album{all.length > 1 ? "s" : ""}
              {comics.hasNextPage ? " et plus" : ""}
            </p>
          )}
        </div>

        {availableLibraries.length > 1 && (
          <div className="flex flex-wrap gap-1" role="group" aria-label="Filtrer par bibliothèque">
            <FilterChip active={libraryId === undefined} onClick={() => setLibraryId(undefined)}>
              Toutes
            </FilterChip>
            {availableLibraries.map((library) => (
              <FilterChip
                key={library.id}
                active={libraryId === library.id}
                onClick={() => setLibraryId(library.id)}
              >
                {library.name}
              </FilterChip>
            ))}
          </div>
        )}
      </div>

      {comics.isLoading ? (
        <ComicGridSkeleton />
      ) : all.length === 0 ? (
        <EmptyState
          title="Aucun album"
          description="Cette bibliothèque ne contient rien pour le moment. Lancez un scan depuis le serveur."
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

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      aria-pressed={active}
      className={cx(
        "rounded-full px-3 py-1.5 text-sm font-medium transition-colors",
        active
          ? "bg-accent text-inverted"
          : "bg-surface-raised text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      {children}
    </button>
  );
}
