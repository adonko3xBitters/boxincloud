"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef } from "react";

import { cx } from "./ui";
import { imageURL } from "@/lib/api/client";
import type { Comic } from "@/lib/api/client";
import { useWorkspace } from "@/lib/workspace";

/**
 * Vue tableau.
 *
 * La vue dense : une ligne par album, huit colonnes d'information, sélection
 * multiple. C'est ici qu'on gère une collection — la grille sert à la
 * parcourir, le tableau à la travailler.
 */

type Column = {
  key: string;
  label: string;
  width: string;
  align?: "left" | "right" | "center";
};

const COLUMNS: Column[] = [
  { key: "index", label: "#", width: "48px", align: "right" },
  { key: "title", label: "Titre", width: "minmax(220px, 2fr)" },
  { key: "series", label: "Série", width: "minmax(140px, 1fr)" },
  { key: "number", label: "N°", width: "62px", align: "right" },
  { key: "progress", label: "Page", width: "86px", align: "right" },
  { key: "pages", label: "Feuilles", width: "84px", align: "right" },
  { key: "size", label: "Taille", width: "92px", align: "right" },
  { key: "released", label: "Parution", width: "100px", align: "right" },
  { key: "read", label: "Lu", width: "52px", align: "center" },
  { key: "rating", label: "Note", width: "96px" },
];

export function ComicTable({
  comics,
  progressByComic,
}: {
  comics: Comic[];
  progressByComic: Map<string, { page: number; status: string }>;
}) {
  const router = useRouter();
  const { isSelected, select, selection, selectAll, clearSelection, favorites, ratings, focused } =
    useWorkspace();

  const focusedRow = useRef<HTMLDivElement>(null);

  // Le carrousel et le tableau montrent la même sélection : déplacer l'un doit
  // amener l'autre au bon endroit, sinon la ligne correspondante reste hors de
  // vue et le lien entre les deux se perd.
  useEffect(() => {
    focusedRow.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [focused]);

  const visible = useMemo(() => comics.map((c) => c.id), [comics]);
  const allSelected = selection.length > 0 && selection.length === comics.length;

  const template = COLUMNS.map((c) => c.width).join(" ");

  return (
    <div className="min-w-0 overflow-x-auto">
      <div className="min-w-[900px]">
        {/* En-tête */}
        <div
          className="sticky top-0 z-10 grid items-center gap-x-3 border-b border-border bg-surface px-3 py-2 text-micro font-semibold uppercase tracking-wide text-subtle"
          style={{ gridTemplateColumns: `28px ${template}` }}
        >
          <input
            type="checkbox"
            checked={allSelected}
            onChange={() => (allSelected ? clearSelection() : selectAll(visible))}
            aria-label="Tout sélectionner"
            className="size-4 accent-[var(--accent)]"
          />
          {COLUMNS.map((column) => (
            <span
              key={column.key}
              className={cx(
                "truncate",
                column.align === "right" && "text-right",
                column.align === "center" && "text-center",
              )}
            >
              {column.label}
            </span>
          ))}
        </div>

        {/* Lignes */}
        {comics.map((comic, index) => {
          const selected = isSelected(comic.id);
          const progress = progressByComic.get(comic.id);
          const rating = ratings.get(comic.id) ?? 0;

          return (
            <div
              key={comic.id}
              ref={focused === comic.id ? focusedRow : undefined}
              onClick={(event) => {
                const mode = event.shiftKey ? "range" : event.metaKey || event.ctrlKey ? "toggle" : "replace";
                select(comic.id, mode, visible);
              }}
              onDoubleClick={() => router.push(`/read?id=${comic.id}`)}
              className={cx(
                "grid cursor-default items-center gap-x-3 border-b border-border/50 px-3 py-2 text-ui",
                "transition-colors duration-[--motion-duration-fast] ease-[--ease-standard]",
                selected
                  ? "bg-accent/15 shadow-[inset_3px_0_0_var(--accent)]"
                  : index % 2 === 1
                    ? "bg-surface-sunken/40 hover:bg-surface-hover"
                    : "hover:bg-surface-hover",
                focused === comic.id && !selected && "bg-surface-hover",
              )}
              style={{ gridTemplateColumns: `28px ${template}` }}
            >
              <input
                type="checkbox"
                checked={selected}
                onChange={() => select(comic.id, "toggle", visible)}
                onClick={(e) => e.stopPropagation()}
                aria-label={`Sélectionner ${comic.title}`}
                className="size-4 accent-[var(--accent)]"
              />

              <span className="text-right tabular-nums text-subtle">{index + 1}</span>

              <span className="flex min-w-0 items-center gap-2">
                {/* Vignette minuscule : elle suffit à reconnaître un album
                    dans une liste, sans coûter la place d'une grille. */}
                <img
                  src={imageURL(comic.coverPath, { width: 160 })}
                  alt=""
                  loading="lazy"
                  decoding="async"
                  className="h-10 w-7 shrink-0 rounded-[3px] bg-surface-sunken object-cover shadow-sm"
                />
                <span className="truncate font-medium text-fg" title={comic.fileName}>
                  {comic.title}
                </span>
                {favorites.has(comic.id) && <HeartIcon />}
              </span>

              <span className="truncate text-muted">{comic.seriesName || "—"}</span>
              <span className="text-right tabular-nums text-muted">{comic.number || "—"}</span>

              <span className="text-right tabular-nums text-muted">
                {progress && progress.page > 0 ? progress.page + 1 : "—"}
              </span>
              <span className="text-right tabular-nums text-muted">{comic.pageCount}</span>
              <span className="text-right tabular-nums text-muted">{formatBytes(comic.fileSize)}</span>
              <span className="text-right tabular-nums text-muted">
                {comic.releasedAt ? comic.releasedAt.slice(0, 4) : "—"}
              </span>

              <span className="text-center">
                <ReadMark status={progress?.status} />
              </span>

              <Rating value={rating} comicId={comic.id} />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ReadMark({ status }: { status?: string }) {
  if (status === "read") {
    return (
      <span title="Lu" className="inline-block text-success">
        <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
          <path d="M13.5 4.5 6.5 11.5 2.5 7.5l1-1 3 3 6-6 1 1Z" />
        </svg>
      </span>
    );
  }
  if (status === "in_progress") {
    return (
      <span title="En cours" className="inline-block size-2.5 rounded-full bg-accent" />
    );
  }
  return <span className="text-subtle">—</span>;
}

/**
 * Note à cinq points, modifiable au clic.
 *
 * Recliquer sur la note courante la retire — sans cette échappatoire, une note
 * posée par erreur serait impossible à défaire.
 */
function Rating({ value, comicId }: { value: number; comicId: string }) {
  const { refreshMarks } = useWorkspace();

  async function set(next: number) {
    const { setRating } = await import("@/lib/api/endpoints");
    await setRating(comicId, next === value ? 0 : next);
    refreshMarks();
  }

  return (
    <span className="flex gap-0.5" onClick={(e) => e.stopPropagation()}>
      {[1, 2, 3, 4, 5].map((step) => (
        <button
          key={step}
          onClick={() => void set(step)}
          aria-label={`Noter ${step} sur 5`}
          className={cx(
            "pressable size-3 rounded-full",
            step <= value ? "bg-warning" : "bg-border hover:bg-border-strong",
          )}
        />
      ))}
    </span>
  );
}

function HeartIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="size-3 shrink-0 text-danger" aria-hidden="true">
      <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
    </svg>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  const units = ["ko", "Mo", "Go"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}
