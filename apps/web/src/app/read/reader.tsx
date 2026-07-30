"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { imageURL } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import {
  buildSpreads,
  pageWidthFor,
  spreadIndexOfPage,
  usePrefetch,
  type Spread,
} from "@/lib/reader/pages";
import { useProgressSaver } from "@/lib/reader/progress";
import { useReaderSettings } from "@/lib/reader/store";
import { ErrorState, Spinner } from "@/components/ui";
import { ReaderChrome } from "./chrome";
import { ScrollReader } from "./scroll";

/**
 * Lecteur.
 *
 * La pièce sur laquelle le projet sera jugé. Trois principes la gouvernent.
 *
 * L'interface s'efface : on lit une planche, pas une application. Les barres
 * apparaissent au mouvement de la souris ou au tap central, et disparaissent.
 *
 * La page suivante est déjà là : le préchargement glissant fait que tourner une
 * page est instantané, jamais un chargement.
 *
 * Rien ne saute : les dimensions viennent du manifeste, donc la place est
 * réservée avant que l'image n'arrive.
 */
export function ReaderView() {
  const router = useRouter();
  const params = useSearchParams();
  const comicId = params.get("id") ?? "";
  const startPage = Number(params.get("page") ?? "0");

  const settings = useReaderSettings();

  const comic = useQuery({
    queryKey: ["comic", comicId],
    queryFn: () => api.getComic(comicId),
    enabled: Boolean(comicId),
  });

  const manifest = useQuery({
    queryKey: ["manifest", comicId],
    queryFn: () => api.getManifest(comicId),
    enabled: Boolean(comicId),
  });

  const progress = useQuery({
    queryKey: ["progress", comicId],
    queryFn: () => api.getProgress(comicId),
    enabled: Boolean(comicId),
  });

  const pages = manifest.data?.pages ?? [];
  const pageCount = manifest.data?.pageCount ?? 0;

  const spreads = useMemo(
    () => buildSpreads(pages, settings.mode === "double"),
    [pages, settings.mode],
  );

  const [spreadIndex, setSpreadIndex] = useState(0);
  const initialised = useRef(false);

  // Position de départ : le paramètre d'URL s'il est fourni, sinon la
  // progression enregistrée. Appliquée une seule fois — sans ce garde, un
  // rafraîchissement de la progression ramènerait le lecteur en arrière en
  // pleine lecture.
  useEffect(() => {
    if (initialised.current || spreads.length === 0) return;
    if (progress.isLoading) return;

    const target = Number.isFinite(startPage) && startPage > 0 ? startPage : (progress.data?.page ?? 0);
    setSpreadIndex(spreadIndexOfPage(spreads, target));
    initialised.current = true;
  }, [spreads, progress.isLoading, progress.data, startPage]);

  // Changer de mode ne doit pas perdre la page : on retrouve le feuillet qui
  // contient la page courante dans la nouvelle composition.
  const currentPage = spreads[spreadIndex]?.pages[0] ?? 0;
  const previousMode = useRef(settings.mode);

  useEffect(() => {
    if (previousMode.current === settings.mode) return;
    previousMode.current = settings.mode;
    if (spreads.length > 0) {
      setSpreadIndex(spreadIndexOfPage(spreads, currentPage));
    }
    // currentPage est volontairement hors dépendances : on ne veut réagir
    // qu'au changement de mode.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings.mode, spreads]);

  const record = useProgressSaver(comicId, pageCount);

  useEffect(() => {
    if (!initialised.current || pageCount === 0) return;
    record(currentPage);
  }, [currentPage, pageCount, record]);

  const width = usePageWidth(settings.mode === "double" ? 2 : 1);
  const flatPages = useMemo(() => pages.map((p) => p.index), [pages]);
  usePrefetch(comicId, flatPages, currentPage, width);

  // ─── Navigation ───────────────────────────────────────────────────────────

  const goTo = useCallback(
    (index: number) => {
      setSpreadIndex(Math.max(0, Math.min(spreads.length - 1, index)));
    },
    [spreads.length],
  );

  const next = useCallback(() => goTo(spreadIndex + 1), [goTo, spreadIndex]);
  const previous = useCallback(() => goTo(spreadIndex - 1), [goTo, spreadIndex]);

  // En lecture manga, les flèches suivent le sens de lecture : la flèche droite
  // remonte le récit. Inverser serait déroutant pour qui lit de droite à gauche.
  const forward = settings.direction === "rtl" ? previous : next;
  const backward = settings.direction === "rtl" ? next : previous;

  const close = useCallback(() => {
    router.push(`/comic?id=${comicId}`);
  }, [router, comicId]);

  useKeyboard({ forward, backward, next, previous, close, goTo, last: spreads.length - 1 });

  // ─── Rendu ────────────────────────────────────────────────────────────────

  if (comic.isError || manifest.isError) {
    return (
      <div className="grid min-h-dvh place-items-center bg-black">
        <ErrorState
          error={comic.error ?? manifest.error}
          onRetry={() => {
            void comic.refetch();
            void manifest.refetch();
          }}
        />
      </div>
    );
  }

  if (manifest.isLoading || !manifest.data) {
    return (
      <div className="grid min-h-dvh place-items-center bg-black">
        <Spinner className="size-7 text-white/60" />
      </div>
    );
  }

  const title = comic.data?.title ?? "";

  return (
    <ReaderChrome
      title={title}
      page={currentPage}
      pageCount={pageCount}
      onClose={close}
      onSeek={(page) => goTo(spreadIndexOfPage(spreads, page))}
    >
      {settings.mode === "scroll" ? (
        <ScrollReader
          comicId={comicId}
          pages={pages}
          width={width}
          startPage={currentPage}
          onPageChange={(page) => goTo(spreadIndexOfPage(spreads, page))}
        />
      ) : (
        <SpreadReader
          comicId={comicId}
          spread={spreads[spreadIndex]}
          width={width}
          fit={settings.fit}
          direction={settings.direction}
          onForward={forward}
          onBackward={backward}
        />
      )}
    </ReaderChrome>
  );
}

// ─── Affichage par feuillet ──────────────────────────────────────────────────

function SpreadReader({
  comicId,
  spread,
  width,
  fit,
  direction,
  onForward,
  onBackward,
}: {
  comicId: string;
  spread: Spread | undefined;
  width: number;
  fit: "width" | "height" | "page";
  direction: "ltr" | "rtl";
  onForward: () => void;
  onBackward: () => void;
}) {
  if (!spread) return null;

  // En lecture manga, les deux pages d'un feuillet s'inversent : la première
  // se lit à droite.
  const ordered = direction === "rtl" ? [...spread.pages].reverse() : spread.pages;

  const fitClass =
    fit === "width" ? "w-full h-auto"
    : fit === "height" ? "h-full w-auto"
    : "max-h-full max-w-full";

  return (
    <div className="relative flex h-dvh w-full items-center justify-center overflow-hidden">
      <div className="flex h-full items-center justify-center gap-0.5">
        {ordered.map((page) => (
          <img
            key={page}
            src={imageURL(`/comics/${comicId}/pages/${page}`, { width })}
            alt={`Page ${page + 1}`}
            decoding="async"
            className={`${fitClass} select-none object-contain`}
            draggable={false}
          />
        ))}
      </div>

      {/*
        Zones de navigation invisibles, à gauche et à droite.
        Un lecteur clique naturellement sur le bord de la page pour tourner —
        c'est le geste d'un livre. Les zones sont larges (30 %) pour rester
        atteignables au pouce sur tablette, et laissent le tiers central libre
        pour révéler l'interface.
      */}
      <button
        onClick={onBackward}
        aria-label="Page précédente"
        className="absolute inset-y-0 left-0 w-[30%] cursor-w-resize focus-visible:bg-white/5"
      />
      <button
        onClick={onForward}
        aria-label="Page suivante"
        className="absolute inset-y-0 right-0 w-[30%] cursor-e-resize focus-visible:bg-white/5"
      />
    </div>
  );
}

// ─── Clavier ─────────────────────────────────────────────────────────────────

/**
 * Raccourcis clavier.
 *
 * Un lecteur sur ordinateur tourne les pages au clavier, pas à la souris. Les
 * touches suivent les conventions : flèches et espace pour avancer, Home et End
 * pour les extrémités, Échap pour sortir.
 */
function useKeyboard({
  forward,
  backward,
  next,
  previous,
  close,
  goTo,
  last,
}: {
  forward: () => void;
  backward: () => void;
  next: () => void;
  previous: () => void;
  close: () => void;
  goTo: (index: number) => void;
  last: number;
}) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      // Ne pas détourner les touches quand l'utilisateur saisit du texte.
      const target = event.target as HTMLElement | null;
      if (target?.tagName === "INPUT" || target?.isContentEditable) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      switch (event.key) {
        case "ArrowRight":
          event.preventDefault();
          forward();
          break;
        case "ArrowLeft":
          event.preventDefault();
          backward();
          break;
        // Bas et espace avancent toujours, quel que soit le sens de lecture :
        // ils suivent la progression du récit, pas la géométrie.
        case "ArrowDown":
        case " ":
        case "PageDown":
          event.preventDefault();
          next();
          break;
        case "ArrowUp":
        case "PageUp":
          event.preventDefault();
          previous();
          break;
        case "Home":
          event.preventDefault();
          goTo(0);
          break;
        case "End":
          event.preventDefault();
          goTo(last);
          break;
        case "Escape":
          event.preventDefault();
          close();
          break;
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [forward, backward, next, previous, close, goTo, last]);
}

// ─── Dimensionnement ─────────────────────────────────────────────────────────

/**
 * Largeur à demander au serveur, en fonction de la fenêtre.
 *
 * Arrondie à des paliers : une largeur exacte créerait une variante de cache
 * par pixel, et un simple redimensionnement de fenêtre en générerait des
 * dizaines côté serveur.
 */
function usePageWidth(pagesPerSpread: number): number {
  const [width, setWidth] = useState(1600);

  useEffect(() => {
    function measure() {
      const viewport = window.innerWidth / pagesPerSpread;
      setWidth(pageWidthFor(viewport, window.devicePixelRatio));
    }

    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [pagesPerSpread]);

  return width;
}
