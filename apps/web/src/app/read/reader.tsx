"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { useT } from "@/i18n";
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
import { useColumnZoom } from "@/lib/reader/column";
import { useZoom } from "@/lib/reader/zoom";
import { ErrorState, Spinner, cx } from "@/components/ui";
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

  // Le repli `?? []` produirait un tableau neuf à chaque rendu, et les mémos
  // qui en dépendent se recalculeraient tous — sur un album de deux cents
  // pages, à chaque frappe.
  const pages = useMemo(() => manifest.data?.pages ?? [], [manifest.data]);
  const pageCount = manifest.data?.pageCount ?? 0;

  /*
    Deux zooms, parce que ce sont deux gestes différents : agrandir une planche
    et s'y déplacer, ou élargir un ruban vertical.

    Celui des modes page vit dans `SpreadReader`, qui le remet à plat à chaque
    feuillet. Celui du défilement vit ici, parce qu'il traverse tout l'album —
    on règle sa largeur de colonne une fois, pas à chaque page.
  */
  const column = useColumnZoom();
  useZoomKeyboard(column, settings.mode === "scroll");

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

  // Le lecteur revient à l'espace de travail : c'est le seul écran qui existe
  // désormais, l'ancienne page d'album ayant disparu avec la refonte.
  const close = useCallback(() => {
    router.push("/");
  }, [router]);

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
      comicId={comicId}
      pages={pages}
      onClose={close}
      onSeek={(page) => goTo(spreadIndexOfPage(spreads, page))}
    >
      {settings.mode === "scroll" ? (
        <ScrollReader
          column={column}
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
  const t = useT();
  const key = spread?.pages.join("-") ?? "";
  const zoom = useZoom(key);

  // Pas de condition : `SpreadReader` n'est monté qu'en mode page, et le
  // défilement continu a son propre zoom, piloté un cran au-dessus.
  useZoomKeyboard(zoom);

  if (!spread) return null;

  // En lecture manga, les deux pages d'un feuillet s'inversent : la première
  // se lit à droite.
  const ordered = direction === "rtl" ? [...spread.pages].reverse() : spread.pages;

  const fitClass =
    fit === "width" ? "w-full h-auto"
    : fit === "height" ? "h-full w-auto"
    : "max-h-full max-w-full";

  // Agrandir une image déjà servie à sa taille d'affichage ne révèle que des
  // pixels. On redemande donc la planche à une définition supérieure — arrondie
  // aux paliers du serveur, sinon chaque niveau de zoom créerait sa variante de
  // cache.
  const sharpWidth = zoom.settledScale > 1.2 ? pageWidthFor(width * zoom.settledScale) : width;

  return (
    <div
      ref={zoom.viewportRef}
      className={cx(
        "relative flex h-dvh w-full items-center justify-center overflow-hidden",
        zoom.zoomed && "cursor-grab active:cursor-grabbing",
      )}
      style={{ touchAction: "none" }}
    >
      <div
        ref={zoom.contentRef}
        className="flex h-full items-center justify-center gap-0.5 will-change-transform"
      >
        {ordered.map((page) => (
          <PageImage
            key={page}
            comicId={comicId}
            page={page}
            width={width}
            sharpWidth={sharpWidth}
            fitClass={fitClass}
          />
        ))}
      </div>

      {/*
        Zones de navigation invisibles, à gauche et à droite.
        Un lecteur clique naturellement sur le bord de la page pour tourner —
        c'est le geste d'un livre. Les zones sont larges (30 %) pour rester
        atteignables au pouce sur tablette, et laissent le tiers central libre
        pour révéler l'interface.

        Elles s'effacent dès que la planche est agrandie : le même glissement
        sert alors à se déplacer dans l'image, et tourner la page en tentant de
        cadrer une case serait la pire des surprises.
      */}
      {!zoom.zoomed && (
        <>
          <button
            onClick={onBackward}
            aria-label={t("reader.previousPage")}
            className="absolute inset-y-0 left-0 w-[30%] cursor-w-resize focus-visible:bg-white/5"
          />
          <button
            onClick={onForward}
            aria-label={t("reader.nextPage")}
            className="absolute inset-y-0 right-0 w-[30%] cursor-e-resize focus-visible:bg-white/5"
          />
        </>
      )}

      {zoom.zoomed && <ZoomBadge scale={zoom.settledScale} onReset={zoom.reset} />}
    </div>
  );
}

/**
 * Une planche, éventuellement doublée d'une version nette.
 *
 * La version haute définition est superposée et ne devient visible qu'une fois
 * chargée. Remplacer la source de l'image d'origine viderait le cadre le temps
 * du téléchargement — un clignotement noir en plein zoom, exactement au moment
 * où l'on regarde de près.
 */
function PageImage({
  comicId,
  page,
  width,
  sharpWidth,
  fitClass,
}: {
  comicId: string;
  page: number;
  width: number;
  sharpWidth: number;
  fitClass: string;
}) {
  const [sharpReady, setSharpReady] = useState(false);

  useEffect(() => setSharpReady(false), [sharpWidth]);

  return (
    <span className="relative inline-flex h-full items-center">
      <img
        src={imageURL(`/comics/${comicId}/pages/${page}`, { width })}
        alt={`Page ${page + 1}`}
        decoding="async"
        className={`${fitClass} select-none object-contain`}
        draggable={false}
      />

      {sharpWidth > width && (
        <img
          src={imageURL(`/comics/${comicId}/pages/${page}`, { width: sharpWidth })}
          alt=""
          aria-hidden="true"
          decoding="async"
          onLoad={() => setSharpReady(true)}
          draggable={false}
          className={cx(
            "absolute inset-0 size-full select-none object-contain",
            "transition-opacity duration-(--motion-duration-normal)",
            sharpReady ? "opacity-100" : "opacity-0",
          )}
        />
      )}
    </span>
  );
}

/** Niveau de zoom courant, avec un retour à la taille normale d'un clic. */
function ZoomBadge({ scale, onReset }: { scale: number; onReset: () => void }) {
  return (
    <button
      onClick={onReset}
      className={cx(
        "pressable absolute bottom-20 left-1/2 z-20 -translate-x-1/2 rounded-full",
        "border border-white/15 bg-black/70 px-3 py-1.5 font-mono text-meta tabular-nums",
        "text-white/80 backdrop-blur hover:bg-black/85 hover:text-white",
      )}
    >
      {scale.toFixed(1)}× · réinitialiser
    </button>
  );
}

/**
 * Raccourcis de zoom.
 *
 * Les mêmes que partout ailleurs : `+`, `-`, et `0` pour revenir à la taille
 * normale. Les réinventer n'apporterait rien.
 */
/** Ce que le clavier demande d'un zoom, quel qu'il soit. */
type Zoomable = { zoomBy: (factor: number) => void; reset: () => void };

function useZoomKeyboard(zoom: Zoomable, active = true) {
  useEffect(() => {
    if (!active) return undefined;

    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      if (target?.tagName === "INPUT" || target?.isContentEditable) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "+" || event.key === "=") {
        event.preventDefault();
        zoom.zoomBy(1.25);
      } else if (event.key === "-") {
        event.preventDefault();
        zoom.zoomBy(1 / 1.25);
      } else if (event.key === "0") {
        event.preventDefault();
        zoom.reset();
      }
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [zoom, active]);
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
