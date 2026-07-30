"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { cx } from "./ui";
import { FolderDialogs, type FolderDialog } from "./folder-dialogs";
import * as api from "@/lib/api/endpoints";
import type { Folder } from "@/lib/api/endpoints";
import { useWorkspace, type Scope } from "@/lib/workspace";

/**
 * Barre latérale.
 *
 * Trois blocs, dans l'ordre où on les consulte : les bibliothèques, l'arbre des
 * dossiers, puis les listes. Elle est permanente — c'est ce qui permet à tout
 * de se gérer sur une seule page, sans navigation.
 */
export function Sidebar() {
  const { scope, setScope } = useWorkspace();
  const [dialog, setDialog] = useState<FolderDialog | null>(null);

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const series = useQuery({ queryKey: ["series", "all"], queryFn: () => api.listSeries({ limit: 200 }) });

  const activeLibrary =
    scope.kind === "library" || scope.kind === "folder" ? scope.libraryId : undefined;

  const folders = useQuery({
    queryKey: ["folders", activeLibrary],
    queryFn: () => api.listFolders(activeLibrary),
  });

  // Les actions sur les dossiers visent une bibliothèque : à défaut de portée
  // active, la première fait office de défaut, ce qui couvre le cas courant
  // d'une installation à bibliothèque unique.
  const firstLibrary = activeLibrary ?? libraries.data?.libraries[0]?.id;

  return (
    <aside className="flex h-full w-[var(--layout-sidebar-width)] shrink-0 flex-col overflow-y-auto border-r border-border bg-surface-sunken">
      <Section title="Bibliothèques" count={libraries.data?.libraries.length}>
        <Row
          active={scope.kind === "all"}
          onClick={() => setScope({ kind: "all" })}
          icon={<StackIcon />}
          label="Tous les albums"
        />
        {libraries.data?.libraries.map((library) => (
          <Row
            key={library.id}
            active={scope.kind === "library" && scope.libraryId === library.id}
            onClick={() => setScope({ kind: "library", libraryId: library.id })}
            icon={<BoxIcon />}
            label={library.name}
            badge={library.comicCount}
          />
        ))}
      </Section>

      <Section
        title="Dossiers"
        action={
          firstLibrary && (
            <button
              onClick={() => setDialog({ kind: "create", libraryId: firstLibrary, parent: "" })}
              aria-label="Nouveau dossier à la racine"
              title="Nouveau dossier"
              className="pressable grid size-5 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
            >
              <svg viewBox="0 0 16 16" fill="none" className="size-3.5" aria-hidden="true">
                <path d="M8 4v8M4 8h8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
              </svg>
            </button>
          )
        }
      >
        <FolderTree
          folders={folders.data?.folders ?? []}
          scope={scope}
          onSelect={setScope}
          libraryId={activeLibrary}
          onAction={setDialog}
        />
      </Section>

      <Section title="Séries" count={series.data?.items.length}>
        <div className="max-h-72 overflow-y-auto">
          {series.data?.items.map((item) => (
            <Row
              key={item.id}
              active={scope.kind === "series" && scope.seriesId === item.id}
              onClick={() => setScope({ kind: "series", seriesId: item.id, name: item.name })}
              icon={<SeriesIcon />}
              label={item.name}
              badge={item.comicCount}
            />
          ))}
        </div>
      </Section>

      <Section title="Listes de lecture">
        <Row
          active={scope.kind === "favorites"}
          onClick={() => setScope({ kind: "favorites" })}
          icon={<HeartIcon className="text-danger" />}
          label="Favoris"
        />
        <Row
          active={scope.kind === "reading"}
          onClick={() => setScope({ kind: "reading" })}
          icon={<BookmarkIcon className="text-accent" />}
          label="En cours"
        />
        <Row
          active={scope.kind === "recent"}
          onClick={() => setScope({ kind: "recent" })}
          icon={<DotIcon className="text-warning" />}
          label="Récents"
        />
      </Section>
      <FolderDialogs dialog={dialog} onClose={() => setDialog(null)} />
    </aside>
  );
}

// ─── Arborescence ────────────────────────────────────────────────────────────

/**
 * Arbre des dossiers.
 *
 * Le serveur renvoie une liste à plat, triée de sorte qu'un parent précède ses
 * enfants. On la parcourt en une passe en respectant les nœuds repliés — pas de
 * récursion, pas de construction d'arbre intermédiaire.
 */
function FolderTree({
  folders,
  scope,
  onSelect,
  libraryId,
  onAction,
}: {
  folders: Folder[];
  scope: Scope;
  onSelect: (scope: Scope) => void;
  libraryId?: string;
  onAction: (dialog: FolderDialog) => void;
}) {
  // Les dossiers de premier niveau sont dépliés d'emblée : replier tout
  // obligerait à cliquer avant de voir quoi que ce soit.
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const visible = useMemo(() => {
    const out: Folder[] = [];
    let skipPrefix: string | null = null;

    for (const folder of folders) {
      if (skipPrefix !== null) {
        if (folder.path.startsWith(skipPrefix + "/")) continue;
        skipPrefix = null;
      }
      out.push(folder);
      if (collapsed.has(folder.path)) skipPrefix = folder.path;
    }
    return out;
  }, [folders, collapsed]);

  const hasChildren = (folder: Folder) =>
    folders.some((other) => other.path.startsWith(folder.path === "" ? "" : folder.path + "/") && other.path !== folder.path);

  if (folders.length === 0) {
    return <p className="px-3 py-2 text-meta text-subtle">Aucun dossier</p>;
  }

  return (
    <>
      {visible.map((folder) => {
        const active = scope.kind === "folder" && scope.path === folder.path;
        const expandable = hasChildren(folder);
        const isCollapsed = collapsed.has(folder.path);

        return (
          <div key={folder.path || "__root__"} className="group/tree flex items-center">
            {expandable ? (
              <button
                onClick={() =>
                  setCollapsed((current) => {
                    const next = new Set(current);
                    if (next.has(folder.path)) next.delete(folder.path);
                    else next.add(folder.path);
                    return next;
                  })
                }
                aria-label={isCollapsed ? "Déplier" : "Replier"}
                className="grid size-6 shrink-0 place-items-center rounded text-subtle transition-colors hover:bg-surface-hover hover:text-fg"
                style={{ marginLeft: folder.depth * 12 }}
              >
                <ChevronIcon
                  className={cx(
                    "size-3.5 transition-transform duration-(--motion-duration-normal) ease-spring",
                    !isCollapsed && "rotate-90",
                  )}
                />
              </button>
            ) : (
              <span className="size-6 shrink-0" style={{ marginLeft: folder.depth * 12 }} />
            )}

            <button
              onClick={() => onSelect({ kind: "folder", path: folder.path, libraryId })}
              className={cx(
                "pressable fade-in flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left text-ui",
                active
                  ? "bg-accent text-inverted shadow-sm"
                  : "text-muted hover:bg-surface-hover hover:text-fg",
              )}
            >
              <FolderIcon className="size-4 shrink-0 opacity-70" />
              <span className="truncate">{folder.path === "" ? "Racine" : folder.name}</span>
              <span className={cx("ml-auto shrink-0 text-meta tabular-nums", active ? "opacity-80" : "text-subtle")}>
                {folder.comicCount}
              </span>
            </button>

            <FolderMenu folder={folder} onAction={onAction} />
          </div>
        );
      })}
    </>
  );
}

/**
 * Actions d'un dossier.
 *
 * Révélées au survol plutôt qu'affichées en permanence : une arborescence
 * constellée de boutons devient illisible, et ces gestes sont rares comparés au
 * simple fait de cliquer pour parcourir.
 *
 * La racine n'en a pas : elle EST la bibliothèque, la renommer ou la supprimer
 * n'aurait aucun sens représentable.
 */
function FolderMenu({
  folder,
  onAction,
}: {
  folder: Folder;
  onAction: (dialog: FolderDialog) => void;
}) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return undefined;
    function close() {
      setOpen(false);
    }
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open]);

  const isRoot = folder.path === "";

  return (
    <div className="relative shrink-0">
      <button
        onPointerDown={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
        aria-label={`Actions sur ${folder.name || "la racine"}`}
        className={cx(
          "pressable grid size-6 place-items-center rounded text-subtle",
          "opacity-0 transition-opacity focus-visible:opacity-100 group-hover/tree:opacity-100",
          open && "opacity-100 bg-surface-hover text-fg",
          "hover:bg-surface-hover hover:text-fg",
        )}
      >
        <svg viewBox="0 0 16 16" fill="currentColor" className="size-3.5" aria-hidden="true">
          <circle cx="8" cy="3.5" r="1.3" />
          <circle cx="8" cy="8" r="1.3" />
          <circle cx="8" cy="12.5" r="1.3" />
        </svg>
      </button>

      {open && (
        <div className="fade-in absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border border-border bg-surface-raised p-1 shadow-lg">
          <MenuItem
            onClick={() =>
              onAction({ kind: "create", libraryId: folder.libraryId, parent: folder.path })
            }
          >
            Nouveau sous-dossier
          </MenuItem>

          {!isRoot && (
            <>
              <MenuItem
                onClick={() =>
                  onAction({
                    kind: "rename",
                    libraryId: folder.libraryId,
                    path: folder.path,
                    name: folder.name,
                  })
                }
              >
                Renommer
              </MenuItem>
              <MenuItem
                destructive
                onClick={() =>
                  onAction({
                    kind: "delete",
                    libraryId: folder.libraryId,
                    path: folder.path,
                    comicCount: folder.comicCount,
                  })
                }
              >
                Supprimer
              </MenuItem>
            </>
          )}
        </div>
      )}
    </div>
  );
}

function MenuItem({
  onClick,
  destructive,
  children,
}: {
  onClick: () => void;
  destructive?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      onPointerDown={(e) => e.stopPropagation()}
      onClick={onClick}
      className={cx(
        "pressable w-full rounded px-2 py-1.5 text-left text-ui",
        destructive
          ? "text-danger hover:bg-danger/10"
          : "text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      {children}
    </button>
  );
}

// ─── Éléments ────────────────────────────────────────────────────────────────

function Section({
  title,
  count,
  action,
  children,
}: {
  title: string;
  count?: number;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="border-b border-border py-2.5 last:border-b-0">
      <div className="flex items-center justify-between px-3 pb-1.5">
        <h2 className="text-micro font-semibold uppercase tracking-wider text-subtle">{title}</h2>
        {action}
        {count !== undefined && (
          <span className="text-micro tabular-nums text-subtle">{count}</span>
        )}
      </div>
      <div className="px-1.5">{children}</div>
    </div>
  );
}

function Row({
  active,
  onClick,
  icon,
  label,
  badge,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  badge?: number;
}) {
  return (
    <button
      onClick={onClick}
      className={cx(
        "pressable flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-ui",
        active
          ? "bg-accent text-inverted shadow-sm"
          : "text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      <span className="grid size-5 shrink-0 place-items-center">{icon}</span>
      <span className="truncate">{label}</span>
      {badge !== undefined && (
        <span className={cx("ml-auto shrink-0 text-meta tabular-nums", active ? "opacity-80" : "text-subtle")}>
          {badge}
        </span>
      )}
    </button>
  );
}

// ─── Icônes ──────────────────────────────────────────────────────────────────

function StackIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="M2 5.5 8 3l6 2.5L8 8 2 5.5Z" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
      <path d="m2 8.5 6 2.5 6-2.5M2 11.5 8 14l6-2.5" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
    </svg>
  );
}

function BoxIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="M2 5 8 2.5 14 5v6L8 13.5 2 11V5Z" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
      <path d="m2 5 6 2.5L14 5M8 7.5v6" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
    </svg>
  );
}

function FolderIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" className={className} aria-hidden="true">
      <path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.2l1.3 1.5h5.5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5v-7Z" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
    </svg>
  );
}

function SeriesIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <rect x="2.5" y="2.5" width="4" height="11" rx="0.8" stroke="currentColor" strokeWidth="1.3" />
      <rect x="8" y="2.5" width="4" height="11" rx="0.8" stroke="currentColor" strokeWidth="1.3" />
    </svg>
  );
}

function HeartIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-4", className)} aria-hidden="true">
      <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
    </svg>
  );
}

function BookmarkIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-4", className)} aria-hidden="true">
      <path d="M4 2.5h8v11l-4-2.8-4 2.8v-11Z" />
    </svg>
  );
}

function DotIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-3", className)} aria-hidden="true">
      <circle cx="8" cy="8" r="4" />
    </svg>
  );
}

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 12 12" fill="none" className={className} aria-hidden="true">
      <path d="m4.5 2.5 3.5 3.5-3.5 3.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
