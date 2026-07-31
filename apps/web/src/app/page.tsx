"use client";

import Link from "next/link";
import { useCallback, useMemo, useRef, useState } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { BrandLockup } from "@/components/brand";
import { ComicTable } from "@/components/comic-table";
import { Coverflow } from "@/components/coverflow";
import { AccountsPanel } from "@/components/accounts-panel";
import { useComicMenu } from "@/components/comic-menu";
import { MobileAppDialog } from "@/components/mobile-app-dialog";
import { SessionsPanel } from "@/components/sessions-panel";
import { StoragePanel } from "@/components/storage-panel";
import { AddContentButton, GlobalDropZone, IngestProvider } from "@/components/ingest";
import { DetailPanel } from "@/components/detail-panel";
import { SearchOverlay } from "@/components/search-overlay";
import { Sidebar } from "@/components/sidebar";
import { Toolbar } from "@/components/toolbar";
import { EmptyState, ErrorState, Spinner, cx } from "@/components/ui";
import { imageURL } from "@/lib/api/client";
import type { Comic } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useCurrentUser, useLogout, useRequireAuth } from "@/lib/auth";
import { useDismissOnOutside } from "@/lib/dismiss";
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
      <IngestProvider>
        <GlobalDropZone>
          <Workspace />
        </GlobalDropZone>
      </IngestProvider>
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
  const [accountsOpen, setAccountsOpen] = useState(false);
  const [storageOpen, setStorageOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sessionsOpen, setSessionsOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  // Un clic ailleurs referme le menu, comme partout ailleurs dans le système.
  // La référence englobe le bouton ET le menu : rouvrir ou choisir un élément
  // ne doit pas être vu comme un clic « ailleurs ».
  const menuRef = useRef<HTMLDivElement>(null);
  const closeMenu = useCallback(() => setMenuOpen(false), []);
  useDismissOnOutside(menuOpen, menuRef, closeMenu);

  return (
    <header className="flex h-13 shrink-0 items-center gap-3 border-b border-border bg-surface px-4">
      <BrandLockup />

      <div className="ml-auto flex items-center gap-2">
        <AddContentButton />
        <SearchOverlay />
        <ThemeToggle />

        {/*
          Menu au CLIC, pas au survol.

          Un menu ouvert au survol se referme dès que le curseur traverse
          l'espace qui le sépare de son bouton — c'est-à-dire à chaque tentative
          de l'atteindre. Le clic supprime le problème plutôt que de le
          contourner en collant les deux éléments.
        */}
        <div className="relative" ref={menuRef}>
          <button
            onClick={() => setMenuOpen((v) => !v)}
            aria-expanded={menuOpen}
            aria-haspopup="menu"
            className="pressable grid size-8 place-items-center rounded-full bg-accent-subtle text-ui font-semibold text-accent-text"
            aria-label="Compte"
          >
            {(user?.displayName || user?.username || "?").charAt(0).toUpperCase()}
          </button>

          {menuOpen && (
          <div
            role="menu"
            className="fade-in absolute right-0 top-full z-50 mt-1.5 w-52 rounded-lg border border-border bg-surface-raised p-1.5 shadow-lg"
          >
            <p className="truncate px-2 py-1 text-meta text-muted">
              {user?.username} · {user?.role === "admin" ? "admin" : "utilisateur"}
            </p>

            {/*
              Accessible à tout le monde, pas seulement aux administrateurs :
              c'est l'entrée par laquelle un membre de la famille installe
              l'application sur son propre téléphone.
            */}
            <button
              onClick={() => {
                setMenuOpen(false);
                setMobileOpen(true);
              }}
              className="pressable w-full rounded px-2 py-1.5 text-left text-ui text-muted hover:bg-surface-hover hover:text-fg"
            >
              Application mobile
            </button>

            {/*
              Les appareils connectés relèvent du compte, pas de
              l'administration : chacun révoque les siens, y compris un
              utilisateur sans droits particuliers.
            */}
            <button
              onClick={() => {
                setMenuOpen(false);
                setSessionsOpen(true);
              }}
              className="pressable w-full rounded px-2 py-1.5 text-left text-ui text-muted hover:bg-surface-hover hover:text-fg"
            >
              Appareils connectés
            </button>

            {user?.role === "admin" && (
              <>
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    setStorageOpen(true);
                  }}
                  className="pressable w-full rounded px-2 py-1.5 text-left text-ui text-muted hover:bg-surface-hover hover:text-fg"
                >
                  Stockage
                </button>
                <button
                  onClick={() => {
                    setMenuOpen(false);
                    setAccountsOpen(true);
                  }}
                  className="pressable w-full rounded px-2 py-1.5 text-left text-ui text-muted hover:bg-surface-hover hover:text-fg"
                >
                  Comptes
                </button>
              </>
            )}

            <button
              onClick={() => void logout()}
              className="pressable w-full rounded px-2 py-1.5 text-left text-ui text-muted hover:bg-surface-hover hover:text-fg"
            >
              Se déconnecter
            </button>
          </div>
          )}
        </div>
      </div>

      {mobileOpen && <MobileAppDialog onClose={() => setMobileOpen(false)} />}
      {sessionsOpen && <SessionsPanel onClose={() => setSessionsOpen(false)} />}
      {storageOpen && <StoragePanel onClose={() => setStorageOpen(false)} />}
      {accountsOpen && <AccountsPanel onClose={() => setAccountsOpen(false)} />}
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
      className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
    >
      <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
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

  // Les boîtes de suppression et de rangement rappellent sur quoi elles portent :
  // agir sur « 12 albums » sans savoir lesquels invite à l'erreur.
  const titleOf = useMemo(() => {
    const byId = new Map(all.map((comic) => [comic.id, comic.title]));
    return (id: string) => byId.get(id) ?? id;
  }, [all]);

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
        <h1 key={scopeLabel(scope)} className="fade-in text-title font-semibold text-fg">
          {scopeLabel(scope)}
        </h1>
        <span className="text-meta tabular-nums text-subtle">
          {comics.isLoading ? "…" : `${all.length}${comics.hasNextPage ? "+" : ""} albums`}
        </span>
      </div>

      <Toolbar visibleIds={ids} titleOf={titleOf} />

      {/* Le carrousel est solidaire de la liste : il occupe le haut de la même
          zone, et la sélection circule entre les deux. Deux vues d'une même
          chose, pas deux écrans. */}
      {view === "coverflow" && !comics.isLoading && all.length > 0 && (
        <Coverflow comics={all} />
      )}

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
        ) : view === "list" || view === "coverflow" ? (
          <ComicTable comics={all} progressByComic={progressByComic} />
        ) : (
          <CoverGrid comics={all} progressByComic={progressByComic} />
        )}

        {comics.hasNextPage && (
          <div className="p-4 text-center">
            <button
              onClick={() => void comics.fetchNextPage()}
              disabled={comics.isFetchingNextPage}
              className="pressable rounded-md border border-border px-4 py-2 text-ui text-muted hover:bg-surface-hover hover:text-fg disabled:opacity-50"
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
 * Grille de couvertures.
 *
 * Large : une couverture de BD est une illustration, souvent la seule chose par
 * laquelle on reconnaît un album. La réduire à une vignette de gestionnaire de
 * fichiers économise de la place au prix de ce qu'on est venu regarder.
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

  const titleOf = useMemo(() => {
    const byId = new Map(comics.map((comic) => [comic.id, comic.title]));
    return (id: string) => byId.get(id) ?? id;
  }, [comics]);

  const menu = useComicMenu(titleOf);

  return (
    <div
      className="grid gap-x-5 gap-y-6 p-5"
      style={{ gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))" }}
    >
      {comics.map((comic, index) => {
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
            onContextMenu={menu.bind(comic.id, ids)}
            style={{
              // Cascade plafonnée : au-delà d'une vingtaine de vignettes,
              // l'effet est acquis et attendre davantage devient une latence.
              animationDelay: `${Math.min(index, 20) * 22}ms`,
            }}
            className={cx(
              "rise-in group cursor-default rounded-lg p-2",
              "transition-[background-color,box-shadow,transform] duration-(--motion-duration-fast)",
              "ease-emphasized hover:-translate-y-0.5",
              selected ? "bg-accent/20 ring-1 ring-accent" : "hover:bg-surface-hover",
            )}
          >
            <div
              className={cx(
                "relative overflow-hidden rounded-[5px] bg-surface-sunken",
                "shadow-[var(--shadow-cover)] transition-shadow duration-(--motion-duration-normal)",
                "group-hover:shadow-[var(--shadow-cover-hover)]",
              )}
              style={{ aspectRatio: 0.7 }}
            >
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
                src={imageURL(comic.coverPath, { width: 640 })}
                alt=""
                loading="lazy"
                decoding="async"
                className="relative size-full object-cover"
              />

              {/* Marqueurs posés sur la couverture : ils informent sans coûter
                  de place sous la vignette. */}
              <div className="absolute left-2 top-2 flex gap-1">
                {favorites.has(comic.id) && (
                  <span className="grid size-6 place-items-center rounded-full bg-black/60 backdrop-blur-sm">
                    <svg viewBox="0 0 16 16" fill="currentColor" className="size-3.5 text-danger" aria-hidden="true">
                      <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
                    </svg>
                  </span>
                )}
              </div>

              {progress?.status === "read" && (
                <span className="absolute right-2 top-2 grid size-6 place-items-center rounded-full bg-success text-white shadow-sm">
                  <svg viewBox="0 0 16 16" fill="currentColor" className="size-3.5" aria-hidden="true">
                    <path d="M13.5 4.5 6.5 11.5 2.5 7.5l1-1 3 3 6-6 1 1Z" />
                  </svg>
                </span>
              )}

              {progress && progress.status === "in_progress" && (
                <div className="absolute inset-x-0 bottom-0 h-1 bg-black/50">
                  <div
                    className="h-full bg-accent transition-[width] duration-(--motion-duration-slow) ease-standard"
                    style={{ width: `${((progress.page + 1) / Math.max(comic.pageCount, 1)) * 100}%` }}
                  />
                </div>
              )}

              <Link
                href={`/read?id=${comic.id}`}
                onClick={(e) => e.stopPropagation()}
                className={cx(
                  "absolute inset-0 grid place-items-center bg-black/45 opacity-0",
                  "transition-opacity duration-(--motion-duration-normal) group-hover:opacity-100",
                )}
                aria-label={`Lire ${comic.title}`}
              >
                <span
                  className={cx(
                    "grid size-14 scale-75 place-items-center rounded-full bg-white/95 text-black shadow-lg",
                    "transition-transform duration-(--motion-duration-normal) ease-spring",
                    "group-hover:scale-100",
                  )}
                >
                  <svg viewBox="0 0 16 16" fill="currentColor" className="size-6" aria-hidden="true">
                    <path d="M5 3.5v9l7-4.5-7-4.5Z" />
                  </svg>
                </span>
              </Link>
            </div>

            <p className="mt-2.5 truncate text-ui font-medium leading-snug text-fg" title={comic.title}>
              {comic.title}
            </p>
            <p className="mt-0.5 truncate text-meta text-subtle">
              {comic.seriesName ? `${comic.seriesName}${comic.number ? ` · ${comic.number}` : ""}` : `${comic.pageCount} p.`}
            </p>

            {rating > 0 && (
              <span className="mt-1.5 flex gap-1">
                {Array.from({ length: 5 }, (_, i) => (
                  <span key={i} className={cx("size-2 rounded-full", i < rating ? "bg-warning" : "bg-border")} />
                ))}
              </span>
            )}
          </div>
        );
      })}

      {menu.node}
    </div>
  );
}
