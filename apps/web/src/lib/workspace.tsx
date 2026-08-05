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
import type { MessageKey } from "@/i18n";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import * as api from "./api/endpoints";
import type { ComicQuery } from "./api/endpoints";
import type { Comic } from "./api/client";

/**
 * Modes d'affichage.
 *
 * `coverflow` n'est pas une troisième liste : c'est le carrousel posé au-dessus
 * du tableau, les deux partageant la même sélection.
 */
export type ViewMode = "grid" | "list" | "coverflow";
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
  /*
    Le pêle-mêle est la vue d'ouverture.

    C'est celle qui montre le plus : une couverture en grand, ce qui l'entoure,
    et la liste complète en dessous. La grille seule dit moins — elle donne des
    vignettes, mais ni les pages, ni la taille, ni la série, qu'il faut alors
    aller chercher album par album.
  */
  const [view, setView] = useState<ViewMode>("coverflow");
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

        // Le panneau de détail suit `focused`, pas la sélection. L'oublier ici
        // laissait la fiche d'un album supprimé affichée à droite d'une grille
        // vide — un album qu'on ne pouvait plus ni ouvrir ni faire disparaître.
        setFocused(null);
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

/**
 * Libellé de la portée courante, affiché en tête de la zone principale.
 *
 * Prend la fonction de traduction en paramètre plutôt que d'appeler un hook :
 * `scopeLabel` est une fonction pure, appelée depuis le rendu d'un composant
 * qui, lui, a déjà accès au catalogue. En faire un hook l'empêcherait d'être
 * utilisée dans une clé de React, ce qu'elle est.
 *
 * Deux cas ne passent pas par le catalogue : le nom d'une série et le nom d'un
 * dossier viennent des données, pas de l'interface.
 */
export function scopeLabel(
  scope: Scope,
  t: (key: MessageKey) => string,
): string {
  switch (scope.kind) {
    case "library":
      return t("scope.library");
    case "folder":
      return scope.path === ""
        ? t("scope.root")
        : (scope.path.split("/").pop() ?? scope.path);
    case "series":
      return scope.name;
    case "favorites":
      return t("sidebar.favorites");
    case "reading":
      return t("scope.reading");
    case "recent":
      return t("scope.recent");
    default:
      return t("sidebar.allAlbums");
  }
}

export type { Comic };
