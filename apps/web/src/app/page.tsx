"use client";

import Link from "next/link";
import { useMemo } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { BrandLockup } from "@/components/brand";
import { ComicTable } from "@/components/comic-table";
import { DetailPanel } from "@/components/detail-panel";
import { SearchOverlay } from "@/components/search-overlay";
import { Sidebar } from "@/components/sidebar";
import { Toolbar } from "@/components/toolbar";
import { EmptyState, ErrorState, Spinner, cx } from "@/components/ui";
import { imageURL } from "@/lib/api/client";
import type { Comic } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useCurrentUser, useLogout, useRequireAuth } from "@/lib/auth";
import { WorkspaceProvider, scopeLabel, scopeToQuery, useWorkspace } from "@/lib/workspace";

/**
 * L'espace de travail — la page unique.
 *
 * Tout s'y gère : barre latérale à gauche, barre d'outils en haut, liste ou
 * grille au centre, détail à droite. Aucune navigation entre pages ; seul le
 * lecteur s'ouvre en plein écran.
 */
export default function Page() {
  const authenticated = useRequireAuth();

  if (!authenticated) {
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner className="size-6 text-muted" />
      </div>
    );
  }

  return (
    <WorkspaceProvider>
      <Workspace />
    </WorkspaceProvider>
  );
}

function Workspace() {
  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <TopBar />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <MainArea />
        <DetailPanel />
      </div>
    </div>
  );
}

function TopBar() {
  const { data: user } = useCurrentUser();
  const logout = useLogout();

  return (
    <header className="flex h-11 shrink-0 items-center gap-3 border-b border-border bg-surface px-3">
      <BrandLockup />

      <div className="ml-auto flex items-center gap-2">
        <SearchOverlay />
        <ThemeToggle />

        <div className="group relative">
          <button
            className="grid size-7 place-items-center rounded-full bg-accent-subtle text-[12px] font-semibold text-accent-text"
            aria-label="Compte"
          >
            {(user?.displayName || user?.username || "?").charAt(0).toUpperCase()}
          </button>
          <div className="invisible absolute right-0 top-full z-50 mt-1 w-44 rounded-lg border border-border bg-surface-raised p-1 opacity-0 shadow-lg transition-opacity group-hover:visible group-hover:opacity-100">
            <p className="truncate px-2 py-1 text-[12px] text-muted">
              {user?.username} · {user?.role === "admin" ? "admin" : "utilisateur"}
            </p>
            <button
              onClick={() => void logout()}
              className="w-full rounded px-2 py-1 text-left text-[12px] text-muted hover:bg-surface-hover hover:text-fg"
            >
              Se déconnecter
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}

function ThemeToggle() {
  return (
    <button
      onClick={() => {
        const root = document.documentElement;
        const current = root.dataset.theme;
        const next = current === "dark" ? "light" : "dark";
        root.dataset.theme = next;
        localStorage.setItem("boxincloud.theme", next);
      }}
      aria-label="Changer de thème"
      title="Changer de thème"
      className="grid size-7 place-items-center rounded text-subtle transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <svg viewBox="0 0 16 16" fill="currentColor" className="size-3.5" aria-hidden="true">
        <path d="M8 1.5a6.5 6.5 0 1 0 0 13v-13Z" />
        <circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" strokeWidth="1.3" />
      </svg>
    </button>
  );
}

// ─── Zone principale ─────────────────────────────────────────────────────────

function MainArea() {
  const { scope, readStatus, sort, view } = useWorkspace();

  const query = useMemo(() => scopeToQuery(scope, readStatus, sort), [scope, readStatus, sort]);

  const comics = useInfiniteQuery({
    queryKey: ["comics", query],
    queryFn: ({ pageParam }) => api.listComics({ ...query, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor || undefined,
  });

  const all = useMemo(
    () => comics.data?.pages.flatMap((page) => page.items) ?? [],
    [comics.data],
  );
  const ids = useMemo(() => all.map((c) => c.id), [all]);

  // Progression de tous les albums affichés, en une requête plutôt qu'une par
  // ligne — sans quoi une page de cent albums en déclencherait cent.
  const progress = useQuery({
    queryKey: ["continue-reading", "all"],
    queryFn: () => api.continueReading(500),
    staleTime: 30_000,
  });

  const progressByComic = useMemo(() => {
    const map = new Map<string, { page: number; status: string }>();
    for (const item of progress.data?.items ?? []) {
      map.set(item.comicId, { page: item.page, status: item.status });
    }
    return map;
  }, [progress.data]);

  return (
    <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
      <div className="flex shrink-0 items-baseline gap-2 border-b border-border bg-surface px-3 py-2">
        <h1 className="text-sm font-semibold text-fg">{scopeLabel(scope)}</h1>
        <span className="text-[12px] tabular-nums text-subtle">
          {comics.isLoading ? "…" : `${all.length}${comics.hasNextPage ? "+" : ""} albums`}
        </span>
      </div>

      <Toolbar visibleIds={ids} />

      <div className="min-h-0 flex-1 overflow-y-auto">
        {comics.isError ? (
          <ErrorState error={comics.error} onRetry={() => void comics.refetch()} />
        ) : comics.isLoading ? (
          <div className="grid place-items-center py-16">
            <Spinner className="size-6 text-muted" />
          </div>
        ) : all.length === 0 ? (
          <EmptyState
            title="Aucun album ici"
            description="Changez de dossier ou élargissez les filtres."
          />
        ) : view === "list" || view === "detail" ? (
          <ComicTable comics={all} progressByComic={progressByComic} />
        ) : (
          <CoverGrid comics={all} progressByComic={progressByComic} />
        )}

        {comics.hasNextPage && (
          <div className="p-4 text-center">
            <button
              onClick={() => void comics.fetchNextPage()}
              disabled={comics.isFetchingNextPage}
              className="rounded-md border border-border px-4 py-1.5 text-[13px] text-muted hover:bg-surface-hover hover:text-fg disabled:opacity-50"
            >
              {comics.isFetchingNextPage ? "Chargement…" : "Charger la suite"}
            </button>
          </div>
        )}
      </div>
    </main>
  );
}

/**
 * Grille dense.
 *
 * Colonnes serrées et informations empilées sous chaque couverture : une
 * bibliothèque doit se lire d'un coup d'œil, pas s'étaler.
 */
function CoverGrid({
  comics,
  progressByComic,
}: {
  comics: Comic[];
  progressByComic: Map<string, { page: number; status: string }>;
}) {
  const { isSelected, select, favorites, ratings } = useWorkspace();
  const ids = comics.map((c) => c.id);

  return (
    <div
      className="grid gap-3 p-3"
      style={{ gridTemplateColumns: "repeat(auto-fill, minmax(128px, 1fr))" }}
    >
      {comics.map((comic) => {
        const selected = isSelected(comic.id);
        const progress = progressByComic.get(comic.id);
        const rating = ratings.get(comic.id) ?? 0;

        return (
          <div
            key={comic.id}
            onClick={(event) => {
              const mode = event.shiftKey ? "range" : event.metaKey || event.ctrlKey ? "toggle" : "replace";
              select(comic.id, mode, ids);
            }}
            className={cx(
              "group cursor-default rounded-md p-1.5 transition-colors",
              selected ? "bg-accent/20 ring-1 ring-accent" : "hover:bg-surface-hover",
            )}
          >
            <div className="relative overflow-hidden rounded-[3px] bg-surface-sunken shadow-[var(--shadow-cover)]"
                 style={{ aspectRatio: 0.7 }}>
              {comic.coverPlaceholder && (
                <div
                  className="absolute inset-0 scale-110 blur-lg"
                  style={{
                    backgroundImage: `url("${comic.coverPlaceholder}")`,
                    backgroundSize: "cover",
                    backgroundPosition: "center",
                  }}
                  aria-hidden="true"
                />
              )}
              <img
                src={imageURL(comic.coverPath, { width: 320 })}
                alt=""
                loading="lazy"
                decoding="async"
                className="relative size-full object-cover"
              />

              {/* Marqueurs posés sur la couverture : ils informent sans coûter
                  de place sous la vignette. */}
              <div className="absolute left-1 top-1 flex gap-1">
                {favorites.has(comic.id) && (
                  <span className="grid size-4 place-items-center rounded-full bg-black/60">
                    <svg viewBox="0 0 16 16" fill="currentColor" className="size-2.5 text-danger" aria-hidden="true">
                      <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
                    </svg>
                  </span>
                )}
              </div>

              {progress?.status === "read" && (
                <span className="absolute right-1 top-1 grid size-4 place-items-center rounded-full bg-success text-white">
                  <svg viewBox="0 0 16 16" fill="currentColor" className="size-2.5" aria-hidden="true">
                    <path d="M13.5 4.5 6.5 11.5 2.5 7.5l1-1 3 3 6-6 1 1Z" />
                  </svg>
                </span>
              )}

              {progress && progress.status === "in_progress" && (
                <div className="absolute inset-x-0 bottom-0 h-[3px] bg-black/50">
                  <div
                    className="h-full bg-accent"
                    style={{ width: `${((progress.page + 1) / Math.max(comic.pageCount, 1)) * 100}%` }}
                  />
                </div>
              )}

              <Link
                href={`/read?id=${comic.id}`}
                onClick={(e) => e.stopPropagation()}
                className="absolute inset-0 grid place-items-center bg-black/45 opacity-0 transition-opacity group-hover:opacity-100"
                aria-label={`Lire ${comic.title}`}
              >
                <span className="grid size-9 place-items-center rounded-full bg-white/95 text-black">
                  <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
                    <path d="M5 3.5v9l7-4.5-7-4.5Z" />
                  </svg>
                </span>
              </Link>
            </div>

            <p className="mt-1.5 truncate text-[12px] font-medium leading-tight text-fg" title={comic.title}>
              {comic.title}
            </p>
            <p className="truncate text-[11px] text-subtle">
              {comic.seriesName ? `${comic.seriesName}${comic.number ? ` · ${comic.number}` : ""}` : `${comic.pageCount} p.`}
            </p>

            {rating > 0 && (
              <span className="mt-1 flex gap-0.5">
                {Array.from({ length: 5 }, (_, i) => (
                  <span key={i} className={cx("size-1.5 rounded-full", i < rating ? "bg-warning" : "bg-border")} />
                ))}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}
