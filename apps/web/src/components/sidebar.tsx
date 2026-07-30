"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { cx } from "./ui";
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

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const series = useQuery({ queryKey: ["series", "all"], queryFn: () => api.listSeries({ limit: 200 }) });

  const activeLibrary =
    scope.kind === "library" || scope.kind === "folder" ? scope.libraryId : undefined;

  const folders = useQuery({
    queryKey: ["folders", activeLibrary],
    queryFn: () => api.listFolders(activeLibrary),
  });

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

      <Section title="Dossiers">
        <FolderTree folders={folders.data?.folders ?? []} scope={scope} onSelect={setScope} libraryId={activeLibrary} />
      </Section>

      <Section title="Séries" count={series.data?.items.length}>
        <div className="max-h-64 overflow-y-auto">
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
}: {
  folders: Folder[];
  scope: Scope;
  onSelect: (scope: Scope) => void;
  libraryId?: string;
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
    return <p className="px-3 py-1.5 text-xs text-subtle">Aucun dossier</p>;
  }

  return (
    <>
      {visible.map((folder) => {
        const active = scope.kind === "folder" && scope.path === folder.path;
        const expandable = hasChildren(folder);
        const isCollapsed = collapsed.has(folder.path);

        return (
          <div key={folder.path || "__root__"} className="flex items-center">
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
                className="grid size-5 shrink-0 place-items-center text-subtle hover:text-fg"
                style={{ marginLeft: folder.depth * 10 }}
              >
                <ChevronIcon className={cx("size-3 transition-transform", !isCollapsed && "rotate-90")} />
              </button>
            ) : (
              <span className="size-5 shrink-0" style={{ marginLeft: folder.depth * 10 }} />
            )}

            <button
              onClick={() => onSelect({ kind: "folder", path: folder.path, libraryId })}
              className={cx(
                "flex min-w-0 flex-1 items-center gap-1.5 rounded px-1.5 py-1 text-left text-[13px]",
                "transition-colors",
                active ? "bg-accent text-inverted" : "text-muted hover:bg-surface-hover hover:text-fg",
              )}
            >
              <FolderIcon className="size-3.5 shrink-0 opacity-70" />
              <span className="truncate">{folder.path === "" ? "Racine" : folder.name}</span>
              <span className={cx("ml-auto shrink-0 text-[11px] tabular-nums", active ? "opacity-80" : "text-subtle")}>
                {folder.comicCount}
              </span>
            </button>
          </div>
        );
      })}
    </>
  );
}

// ─── Éléments ────────────────────────────────────────────────────────────────

function Section({
  title,
  count,
  children,
}: {
  title: string;
  count?: number;
  children: React.ReactNode;
}) {
  return (
    <div className="border-b border-border py-2 last:border-b-0">
      <div className="flex items-center justify-between px-3 pb-1">
        <h2 className="text-[10px] font-semibold uppercase tracking-wider text-subtle">{title}</h2>
        {count !== undefined && (
          <span className="text-[10px] tabular-nums text-subtle">{count}</span>
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
        "flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-[13px]",
        "transition-colors",
        active ? "bg-accent text-inverted" : "text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      <span className="grid size-4 shrink-0 place-items-center">{icon}</span>
      <span className="truncate">{label}</span>
      {badge !== undefined && (
        <span className={cx("ml-auto shrink-0 text-[11px] tabular-nums", active ? "opacity-80" : "text-subtle")}>
          {badge}
        </span>
      )}
    </button>
  );
}

// ─── Icônes ──────────────────────────────────────────────────────────────────

function StackIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-3.5" aria-hidden="true">
      <path d="M2 5.5 8 3l6 2.5L8 8 2 5.5Z" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
      <path d="m2 8.5 6 2.5 6-2.5M2 11.5 8 14l6-2.5" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
    </svg>
  );
}

function BoxIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-3.5" aria-hidden="true">
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
    <svg viewBox="0 0 16 16" fill="none" className="size-3.5" aria-hidden="true">
      <rect x="2.5" y="2.5" width="4" height="11" rx="0.8" stroke="currentColor" strokeWidth="1.3" />
      <rect x="8" y="2.5" width="4" height="11" rx="0.8" stroke="currentColor" strokeWidth="1.3" />
    </svg>
  );
}

function HeartIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-3.5", className)} aria-hidden="true">
      <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
    </svg>
  );
}

function BookmarkIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-3.5", className)} aria-hidden="true">
      <path d="M4 2.5h8v11l-4-2.8-4 2.8v-11Z" />
    </svg>
  );
}

function DotIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className={cx("size-2.5", className)} aria-hidden="true">
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
