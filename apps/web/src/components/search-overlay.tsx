"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { cx } from "./ui";
import { imageURL } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useT } from "@/i18n";
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
  const t = useT();
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
      {/* Une icône seule : le raccourci « / » se retient à l'usage, et le champ
          factice occupait la place d'une barre d'outils sans rien apporter. */}
      <button
        onClick={() => setOpen(true)}
        aria-label={t("search.action")}
        title={t("search.shortcut")}
        className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
      >
        <SearchIcon className="size-[18px]" />
      </button>

      {/* Voile : ferme au clic, et assombrit sans masquer complètement — on
          garde conscience de ce qui se trouve derrière. */}
      <div
        onClick={() => setOpen(false)}
        aria-hidden={!open}
        className={cx(
          "fixed inset-0 z-50 bg-[var(--overlay)] transition-opacity duration-(--motion-duration-normal)",
          open ? "opacity-100" : "pointer-events-none opacity-0",
        )}
      />

      {/*
        Le panneau reste monté en permanence : c'est ce qui lui permet de glisser
        aussi bien à la fermeture qu'à l'ouverture. Un panneau démonté disparaît
        d'un coup, puisqu'il n'y a plus rien à animer.

        La contrepartie doit être payée : fermé, il est hors de l'écran mais
        toujours dans le document. `inert` le retire du parcours au clavier et de
        l'arbre d'accessibilité — sans quoi la tabulation irait se perdre dans un
        formulaire de recherche invisible.
      */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("search.center")}
        inert={!open}
        className={cx(
          "fixed inset-y-0 right-0 z-50 flex w-full max-w-[480px] flex-col border-l border-border bg-surface shadow-2xl",
          "transition-transform duration-(--motion-duration-slow) ease-emphasized",
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
            placeholder={t("search.placeholder")}
            aria-label={t("search.action")}
            className="min-w-0 flex-1 bg-transparent text-title text-fg outline-none placeholder:text-subtle"
          />
          <button
            onClick={() => setOpen(false)}
            aria-label={t("action.close")}
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
                <Group label={t("search.series")}>
                  {series.map((item, index) => (
                    <button
                      key={item.id}
                      onClick={() => openResult(index)}
                      onMouseEnter={() => setCursor(index)}
                      className={cx(
                        "flex w-full items-center gap-3 px-3 py-2.5 text-left",
                        "transition-colors duration-(--motion-duration-fast)",
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
                <Group label={t("search.albums")}>
                  {comics.map((comic, i) => {
                    const index = series.length + i;
                    return (
                      <button
                        key={comic.id}
                        onClick={() => openResult(index)}
                        onMouseEnter={() => setCursor(index)}
                        className={cx(
                          "flex w-full items-center gap-3 px-3 py-2.5 text-left",
                          "transition-colors duration-(--motion-duration-fast)",
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
  const t = useT();

  return (
    <div className="px-4 py-8 text-center">
      <p className="text-ui text-muted">{t("search.hint")}</p>
      <p className="mt-1.5 text-meta text-subtle">
        {t("search.tolerance")}
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
