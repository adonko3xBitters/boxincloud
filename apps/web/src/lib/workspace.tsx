"use client";

/**
 * État de l'espace de travail.
 *
 * Tout se gère sur une seule page : la sélection dans la barre latérale, les
 * filtres, le mode de vue et la sélection d'albums vivent ensemble. Un contexte
 * plutôt que des props traversant cinq niveaux — chaque composant lit
 * exactement ce dont il a besoin.
 */

import { createContext, useCallback, useContext, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import * as api from "./api/endpoints";
import type { ComicQuery } from "./api/endpoints";
import type { Comic } from "./api/client";

export type ViewMode = "grid" | "list" | "detail";
export type ReadStatus = "" | "unread" | "in_progress" | "read";
export type SortOrder = "recent" | "title" | "released";

/** Ce que la barre latérale a sélectionné. */
export type Scope =
  | { kind: "all" }
  | { kind: "library"; libraryId: string }
  | { kind: "folder"; libraryId?: string; path: string }
  | { kind: "series"; seriesId: string; name: string }
  | { kind: "favorites" }
  | { kind: "reading" }
  | { kind: "recent" };

type WorkspaceValue = {
  scope: Scope;
  setScope: (scope: Scope) => void;

  view: ViewMode;
  setView: (view: ViewMode) => void;

  readStatus: ReadStatus;
  setReadStatus: (status: ReadStatus) => void;

  sort: SortOrder;
  setSort: (sort: SortOrder) => void;

  /** Identifiants sélectionnés, dans l'ordre d'affichage. */
  selection: string[];
  isSelected: (id: string) => boolean;
  select: (id: string, mode: "replace" | "toggle" | "range", visible: string[]) => void;
  selectAll: (ids: string[]) => void;
  clearSelection: () => void;

  /** Album mis en avant dans le panneau de détail. */
  focused: string | null;
  setFocused: (id: string | null) => void;

  favorites: Set<string>;
  ratings: Map<string, number>;
  refreshMarks: () => void;
};

const WorkspaceContext = createContext<WorkspaceValue | null>(null);

export function WorkspaceProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();

  const [scope, setScopeRaw] = useState<Scope>({ kind: "all" });
  const [view, setView] = useState<ViewMode>("grid");
  const [readStatus, setReadStatus] = useState<ReadStatus>("");
  const [sort, setSort] = useState<SortOrder>("recent");

  const [selection, setSelection] = useState<string[]>([]);
  const [anchor, setAnchor] = useState<string | null>(null);
  const [focused, setFocused] = useState<string | null>(null);

  // Favoris et notes : une seule requête, partagée par toutes les vues.
  const marks = useQuery({
    queryKey: ["marks"],
    queryFn: api.getUserMarks,
    staleTime: 30_000,
  });

  const favorites = useMemo(
    () => new Set(marks.data?.favorites ?? []),
    [marks.data],
  );

  const ratings = useMemo(() => {
    const map = new Map<string, number>();
    for (const [id, value] of Object.entries(marks.data?.ratings ?? {})) {
      map.set(id, value);
    }
    return map;
  }, [marks.data]);

  // Changer de portée vide la sélection : garder des albums sélectionnés qui ne
  // sont plus affichés rendrait les actions en lot imprévisibles.
  const setScope = useCallback((next: Scope) => {
    setScopeRaw(next);
    setSelection([]);
    setAnchor(null);
    setFocused(null);
  }, []);

  /**
   * Sélection.
   *
   * Trois modes, ceux de tous les gestionnaires de fichiers : clic simple
   * remplace, Cmd-clic bascule, Maj-clic étend depuis l'ancre. Reproduire ces
   * conventions évite d'avoir à les apprendre.
   */
  const select = useCallback(
    (id: string, mode: "replace" | "toggle" | "range", visible: string[]) => {
      setFocused(id);

      setSelection((current) => {
        switch (mode) {
          case "toggle": {
            setAnchor(id);
            return current.includes(id)
              ? current.filter((existing) => existing !== id)
              : [...current, id];
          }
          case "range": {
            const from = visible.indexOf(anchor ?? id);
            const to = visible.indexOf(id);
            if (from < 0 || to < 0) return [id];

            const [start, end] = from <= to ? [from, to] : [to, from];
            return visible.slice(start, end + 1);
          }
          default:
            setAnchor(id);
            return [id];
        }
      });
    },
    [anchor],
  );

  const value = useMemo<WorkspaceValue>(
    () => ({
      scope,
      setScope,
      view,
      setView,
      readStatus,
      setReadStatus,
      sort,
      setSort,
      selection,
      isSelected: (id) => selection.includes(id),
      select,
      selectAll: (ids) => {
        setSelection(ids);
        setAnchor(ids[0] ?? null);
      },
      clearSelection: () => {
        setSelection([]);
        setAnchor(null);
      },
      focused,
      setFocused,
      favorites,
      ratings,
      refreshMarks: () => void queryClient.invalidateQueries({ queryKey: ["marks"] }),
    }),
    [scope, setScope, view, readStatus, sort, selection, select, focused, favorites, ratings, queryClient],
  );

  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspace(): WorkspaceValue {
  const value = useContext(WorkspaceContext);
  if (!value) throw new Error("useWorkspace hors de WorkspaceProvider");
  return value;
}

/** Traduit la portée en paramètres de requête. */
export function scopeToQuery(
  scope: Scope,
  readStatus: ReadStatus,
  sort: SortOrder,
): ComicQuery {
  const base: ComicQuery = { readStatus, sort, limit: 100 };

  switch (scope.kind) {
    case "library":
      return { ...base, libraryId: scope.libraryId };
    case "folder":
      return { ...base, libraryId: scope.libraryId, folder: scope.path };
    case "series":
      return { ...base, seriesId: scope.seriesId };
    case "favorites":
      return { ...base, favorites: true };
    case "reading":
      // Les portées de la barre latérale imposent leur filtre : cliquer sur
      // « En cours » doit montrer les albums en cours, quel que soit le filtre
      // de lecture actif par ailleurs.
      return { ...base, readStatus: "in_progress" };
    case "recent":
      return { ...base, sort: "recent" };
    default:
      return base;
  }
}

/** Libellé de la portée courante, affiché en tête de la zone principale. */
export function scopeLabel(scope: Scope): string {
  switch (scope.kind) {
    case "library":
      return "Bibliothèque";
    case "folder":
      return scope.path === "" ? "Racine" : (scope.path.split("/").pop() ?? scope.path);
    case "series":
      return scope.name;
    case "favorites":
      return "Favoris";
    case "reading":
      return "En cours de lecture";
    case "recent":
      return "Récemment ajouté";
    default:
      return "Tous les albums";
  }
}

export type { Comic };
