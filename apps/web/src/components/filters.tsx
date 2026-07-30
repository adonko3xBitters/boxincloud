"use client";

import { cx } from "./ui";

/**
 * Barre de filtres et de tri de la bibliothèque.
 *
 * Des puces plutôt qu'un panneau repliable : les trois axes tiennent sur une
 * ligne, et masquer un filtre actif derrière un bouton fait qu'on oublie
 * pourquoi la grille est incomplète.
 */

export type ReadStatus = "" | "unread" | "in_progress" | "read";
export type SortOrder = "recent" | "title" | "released";

const READ_STATUSES: Array<{ value: ReadStatus; label: string }> = [
  { value: "", label: "Tous" },
  { value: "unread", label: "Non lus" },
  { value: "in_progress", label: "En cours" },
  { value: "read", label: "Lus" },
];

const SORTS: Array<{ value: SortOrder; label: string }> = [
  { value: "recent", label: "Ajout récent" },
  { value: "title", label: "Titre" },
  { value: "released", label: "Parution" },
];

export function FilterChip({
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
        "rounded-full px-3 py-1.5 text-sm font-medium",
        "transition-colors duration-[--motion-duration-fast]",
        active
          ? "bg-accent text-inverted"
          : "bg-surface-raised text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      {children}
    </button>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-xs uppercase tracking-wide text-subtle">{label}</span>
      <div className="flex flex-wrap gap-1" role="group" aria-label={label}>
        {children}
      </div>
    </div>
  );
}

export function LibraryFilters({
  libraries,
  libraryId,
  onLibraryChange,
  readStatus,
  onReadStatusChange,
  sort,
  onSortChange,
}: {
  libraries: Array<{ id: string; name: string }>;
  libraryId: string | undefined;
  onLibraryChange: (id: string | undefined) => void;
  readStatus: ReadStatus;
  onReadStatusChange: (status: ReadStatus) => void;
  sort: SortOrder;
  onSortChange: (sort: SortOrder) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
      {/* Le sélecteur de bibliothèque n'apparaît que s'il y a un choix à
          faire : sur une instance à bibliothèque unique, il n'ajouterait que
          du bruit. */}
      {libraries.length > 1 && (
        <Group label="Bibliothèque">
          <FilterChip active={libraryId === undefined} onClick={() => onLibraryChange(undefined)}>
            Toutes
          </FilterChip>
          {libraries.map((library) => (
            <FilterChip
              key={library.id}
              active={libraryId === library.id}
              onClick={() => onLibraryChange(library.id)}
            >
              {library.name}
            </FilterChip>
          ))}
        </Group>
      )}

      <Group label="Lecture">
        {READ_STATUSES.map((status) => (
          <FilterChip
            key={status.value}
            active={readStatus === status.value}
            onClick={() => onReadStatusChange(status.value)}
          >
            {status.label}
          </FilterChip>
        ))}
      </Group>

      <Group label="Tri">
        {SORTS.map((option) => (
          <FilterChip
            key={option.value}
            active={sort === option.value}
            onClick={() => onSortChange(option.value)}
          >
            {option.label}
          </FilterChip>
        ))}
      </Group>
    </div>
  );
}
