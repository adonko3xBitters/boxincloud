"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { cx } from "./ui";
import { imageURL } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useWorkspace } from "@/lib/workspace";

const MIN_QUERY = 2;
const DEBOUNCE_MS = 180;

/**
 * Centre de recherche.
 *
 * Un panneau qui glisse depuis la droite, ouvert au clavier. Il se superpose
 * plutôt que de remplacer la vue : on cherche sans perdre le contexte, et on
 * revient d'un coup d'Échap.
 *
 * Raccourcis : « / » et Cmd-K pour ouvrir, Échap pour fermer, flèches pour
 * parcourir, Entrée pour ouvrir le résultat.
 */
export function SearchOverlay() {
  const router = useRouter();
  const { setScope } = useWorkspace();

  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // Ouverture au clavier. Les deux conventions sont acceptées : « / » comme
  // dans les gestionnaires de fichiers, Cmd-K comme dans les palettes de
  // commandes — deux publics, deux réflexes.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      const typing =
        target?.tagName === "INPUT" || target?.tagName === "TEXTAREA" || target?.isContentEditable;

      if ((event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen(true);
        return;
      }
      if (event.key === "/" && !typing && !event.metaKey && !event.ctrlKey) {
        event.preventDefault();
        setOpen(true);
        return;
      }
      if (event.key === "Escape" && open) {
        event.preventDefault();
        setOpen(false);
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open]);

  useEffect(() => {
    if (open) {
      // Un court délai laisse la transition démarrer avant que le focus ne
      // fasse défiler le panneau ; sans lui, l'ouverture saccade.
      const timer = setTimeout(() => inputRef.current?.focus(), 60);
      return () => clearTimeout(timer);
    }
    setValue("");
    setQuery("");
    setCursor(0);
    return undefined;
  }, [open]);

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(value.trim());
      setCursor(0);
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [value]);

  const results = useQuery({
    queryKey: ["search", query],
    queryFn: ({ signal }) => api.search({ q: query, limit: 30 }, signal),
    enabled: open && query.length >= MIN_QUERY,
  });

  const comics = results.data?.comics ?? [];
  const series = results.data?.series ?? [];
  const total = comics.length + series.length;

  function onInputKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setCursor((c) => Math.min(total - 1, c + 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setCursor((c) => Math.max(0, c - 1));
    } else if (event.key === "Enter" && total > 0) {
      event.preventDefault();
      openResult(cursor);
    }
  }

  function openResult(index: number) {
    if (index < series.length) {
      const item = series[index]!;
      setScope({ kind: "series", seriesId: item.id, name: item.name });
      setOpen(false);
      return;
    }
    const comic = comics[index - series.length];
    if (comic) {
      router.push(`/read?id=${comic.id}`);
      setOpen(false);
    }
  }

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="pressable flex h-9 items-center gap-2 rounded-md border border-border bg-surface px-3 text-ui text-subtle hover:border-border-strong hover:text-muted"
        aria-label="Ouvrir la recherche"
      >
        <SearchIcon />
        <span className="hidden sm:inline">Rechercher</span>
        <kbd className="ml-2 hidden rounded border border-border px-1.5 py-0.5 font-mono text-micro lg:inline">/</kbd>
      </button>

      {/* Voile : ferme au clic, et assombrit sans masquer complètement — on
          garde conscience de ce qui se trouve derrière. */}
      <div
        onClick={() => setOpen(false)}
        aria-hidden={!open}
        className={cx(
          "fixed inset-0 z-50 bg-[var(--overlay)] transition-opacity duration-[--motion-duration-normal]",
          open ? "opacity-100" : "pointer-events-none opacity-0",
        )}
      />

      <div
        role="dialog"
        aria-modal="true"
        aria-label="Centre de recherche"
        className={cx(
          "fixed inset-y-0 right-0 z-50 flex w-full max-w-[480px] flex-col border-l border-border bg-surface shadow-2xl",
          "transition-transform duration-[--motion-duration-slow] ease-[--ease-emphasized]",
          open ? "translate-x-0" : "translate-x-full",
        )}
      >
        <div className="flex items-center gap-2 border-b border-border p-3">
          <SearchIcon className="size-4 shrink-0 text-subtle" />
          <input
            ref={inputRef}
            type="search"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={onInputKeyDown}
            placeholder="Titre, série, numéro…"
            aria-label="Rechercher"
            className="min-w-0 flex-1 bg-transparent text-title text-fg outline-none placeholder:text-subtle"
          />
          <button
            onClick={() => setOpen(false)}
            aria-label="Fermer"
            className="pressable grid size-8 shrink-0 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <kbd className="font-mono text-micro">esc</kbd>
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {query.length < MIN_QUERY ? (
            <Hint />
          ) : total === 0 && !results.isLoading ? (
            <p className="px-4 py-8 text-center text-ui text-muted">
              Aucun résultat pour « {query} »
            </p>
          ) : (
            <>
              {series.length > 0 && (
                <Group label="Séries">
                  {series.map((item, index) => (
                    <button
                      key={item.id}
                      onClick={() => openResult(index)}
                      onMouseEnter={() => setCursor(index)}
                      className={cx(
                        "flex w-full items-center gap-3 px-3 py-2.5 text-left",
                        "transition-colors duration-[--motion-duration-fast]",
                        cursor === index ? "bg-accent/15 shadow-[inset_3px_0_0_var(--accent)]" : "hover:bg-surface-hover",
                      )}
                    >
                      {item.coverPath ? (
                        <img src={imageURL(item.coverPath, { width: 160 })} alt="" loading="lazy"
                             className="h-14 w-10 shrink-0 rounded-[3px] object-cover shadow-sm" />
                      ) : (
                        <span className="h-14 w-10 shrink-0 rounded-[3px] bg-surface-sunken" />
                      )}
                      <span className="min-w-0">
                        <span className="block truncate text-ui font-medium text-fg">{item.name}</span>
                        <span className="block text-meta text-muted">{item.comicCount} albums</span>
                      </span>
                    </button>
                  ))}
                </Group>
              )}

              {comics.length > 0 && (
                <Group label="Albums">
                  {comics.map((comic, i) => {
                    const index = series.length + i;
                    return (
                      <button
                        key={comic.id}
                        onClick={() => openResult(index)}
                        onMouseEnter={() => setCursor(index)}
                        className={cx(
                          "flex w-full items-center gap-3 px-3 py-2.5 text-left",
                          "transition-colors duration-[--motion-duration-fast]",
                          cursor === index ? "bg-accent/15 shadow-[inset_3px_0_0_var(--accent)]" : "hover:bg-surface-hover",
                        )}
                      >
                        <img src={imageURL(comic.coverPath, { width: 160 })} alt="" loading="lazy"
                             className="h-14 w-10 shrink-0 rounded-[3px] object-cover shadow-sm" />
                        <span className="min-w-0">
                          <span className="block truncate text-ui font-medium text-fg">{comic.title}</span>
                          <span className="block truncate text-meta text-muted">
                            {comic.seriesName ? `${comic.seriesName} · ` : ""}{comic.pageCount} pages
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </Group>
              )}
            </>
          )}
        </div>

        <div className="border-t border-border px-3 py-2.5 text-meta text-subtle">
          <kbd className="rounded border border-border px-1 font-mono">↑↓</kbd> parcourir ·{" "}
          <kbd className="rounded border border-border px-1 font-mono">⏎</kbd> ouvrir ·{" "}
          <kbd className="rounded border border-border px-1 font-mono">esc</kbd> fermer
        </div>
      </div>
    </>
  );
}

function Group({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="sticky top-0 z-10 bg-surface px-3 py-2 text-micro font-semibold uppercase tracking-wider text-subtle">
        {label}
      </p>
      {children}
    </div>
  );
}

function Hint() {
  return (
    <div className="px-4 py-8 text-center">
      <p className="text-ui text-muted">Cherchez dans toute la bibliothèque</p>
      <p className="mt-1.5 text-meta text-subtle">
        Les accents et les fautes de frappe sont tolérés — « asterics » trouve « Astérix ».
      </p>
    </div>
  );
}

function SearchIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 20 20" fill="none" className={cx("size-4", className)} aria-hidden="true">
      <circle cx="9" cy="9" r="5.5" stroke="currentColor" strokeWidth="1.6" />
      <path d="m13.5 13.5 3 3" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}
