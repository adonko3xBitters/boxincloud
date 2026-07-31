"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";
import { useRouter } from "next/navigation";

import { cx } from "./ui";
import { useT } from "@/i18n";
import { imageURL } from "@/lib/api/client";
import type { Comic } from "@/lib/api/client";
import { useWorkspace } from "@/lib/workspace";

/**
 * Carrousel de couvertures.
 *
 * Une bibliothèque se feuillette autant qu'elle se consulte. Le tableau donne
 * les faits ; le carrousel donne les couvertures, qui sont la façon dont on
 * reconnaît réellement un album. Les deux sont liés : déplacer l'un déplace
 * l'autre, parce que ce sont deux vues d'une même sélection, pas deux écrans.
 *
 * Rendu en transformations 3D CSS plutôt qu'en canvas ou WebGL : le compositeur
 * du navigateur anime `transform` et `opacity` sans repasser par la mise en
 * page, ce qui tient soixante images par seconde sur une machine modeste, et
 * laisse les couvertures sélectionnables et accessibles au clavier.
 */

/** Couvertures visibles de chaque côté du centre. */
const WING = 5;

/**
 * Dimensions de la plaque : couverture, puis reflet.
 *
 * Elles sont fixées ici plutôt qu'en CSS parce que le placement vertical en
 * dépend — la plaque est positionnée par rapport au centre du carrousel, et ce
 * calcul doit voir les mêmes nombres que le rendu.
 */
const COVER_W = 200;
const COVER_H = Math.round(COVER_W / 0.7);
const REFLECT_H = 76;
const TOP_PAD = 26;
const STAGE_H = TOP_PAD + COVER_H + REFLECT_H + 52;

/** Géométrie du carrousel. Les valeurs sont en pixels, sauf l'angle. */
const CENTER_GAP = 118; // écart du premier voisin, qui dégage la couverture centrale
const STEP = 46; // resserrement des suivantes : l'empilement suggère la profondeur
const ANGLE = 56;
const DEPTH = 120;

export function Coverflow({ comics }: { comics: Comic[] }) {
  const t = useT();
  const router = useRouter();
  const { focused, select, favorites } = useWorkspace();
  const trackRef = useRef<HTMLDivElement>(null);

  const ids = useMemo(() => comics.map((c) => c.id), [comics]);

  // Le centre suit la sélection ; à défaut, le premier album. Sans ce repli, un
  // carrousel vide s'afficherait au chargement alors que la liste, elle, est
  // déjà remplie.
  const center = useMemo(() => {
    const index = focused ? ids.indexOf(focused) : -1;
    return index >= 0 ? index : 0;
  }, [focused, ids]);

  const goTo = useCallback(
    (index: number) => {
      const clamped = Math.max(0, Math.min(comics.length - 1, index));
      const comic = comics[clamped];
      if (comic) {
        select(comic.id, "replace", ids);
      }
    },
    [comics, ids, select],
  );

  // Flèches et molette. La molette horizontale d'un pavé tactile est le geste
  // naturel sur ce type de vue ; l'ignorer donnerait un carrousel qui ne répond
  // pas au seul mouvement qu'on lui adresse spontanément.
  useEffect(() => {
    const track = trackRef.current;
    if (!track) return undefined;

    // Seuil : sans lui, un seul geste de pavé tactile traverserait vingt albums
    // d'un coup. Un cran par franchissement garde le défilement lisible.
    const THRESHOLD = 40;
    let accumulated = 0;

    function onWheel(event: WheelEvent) {
      const delta =
        Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
      if (delta === 0) return;

      event.preventDefault();
      accumulated += delta;

      if (Math.abs(accumulated) >= THRESHOLD) {
        goTo(center + Math.sign(accumulated));
        accumulated = 0;
      }
    }

    track.addEventListener("wheel", onWheel, { passive: false });
    return () => track.removeEventListener("wheel", onWheel);
  }, [center, goTo]);

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      goTo(center - 1);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      goTo(center + 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      goTo(0);
    } else if (event.key === "End") {
      event.preventDefault();
      goTo(comics.length - 1);
    } else if (event.key === "Enter" && comics[center]) {
      event.preventDefault();
      router.push(`/read?id=${comics[center]!.id}`);
    }
  }

  if (comics.length === 0) return null;

  const current = comics[center];

  return (
    <section
      ref={trackRef}
      tabIndex={0}
      onKeyDown={onKeyDown}
      aria-label={t("coverflow.label")}
      aria-roledescription="carrousel"
      className={cx(
        "relative shrink-0 select-none border-b border-border outline-none",
        "bg-gradient-to-b from-surface-sunken to-surface",
      )}
      style={{ height: STAGE_H }}
    >
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{ perspective: 1500, perspectiveOrigin: "50% 42%" }}
      >
        <div className="relative size-full" style={{ transformStyle: "preserve-3d" }}>
          {comics.map((comic, index) => {
            const offset = index - center;
            if (Math.abs(offset) > WING) return null;

            const direction = Math.sign(offset);
            const distance = Math.abs(offset);

            const x =
              distance === 0 ? 0 : direction * (CENTER_GAP + (distance - 1) * STEP);
            const z = distance === 0 ? 0 : -DEPTH - (distance - 1) * 26;
            const rotate = distance === 0 ? 0 : -direction * ANGLE;
            const scale = distance === 0 ? 1 : 0.86 - (distance - 1) * 0.03;
            const opacity = distance === 0 ? 1 : Math.max(0.18, 0.85 - (distance - 1) * 0.2);

            return (
              <button
                key={comic.id}
                type="button"
                tabIndex={-1}
                aria-current={distance === 0 ? "true" : undefined}
                aria-label={comic.title}
                onClick={() => (distance === 0 ? router.push(`/read?id=${comic.id}`) : goTo(index))}
                className="absolute left-1/2 top-1/2 origin-center cursor-pointer"
                style={{
                  width: COVER_W,
                  marginLeft: -COVER_W / 2,
                  // La plaque est ancrée par son sommet plutôt que centrée : le
                  // reflet appartient au décor, pas au sujet, et le centrer
                  // ferait flotter les couvertures au-dessus de leur socle.
                  marginTop: TOP_PAD - STAGE_H / 2,
                  zIndex: 100 - distance,
                  opacity,
                  transform: `translate3d(${x}px, 0, ${z}px) rotateY(${rotate}deg) scale(${scale})`,
                  transformStyle: "preserve-3d",
                  transition:
                    "transform var(--motion-duration-(--motion-duration-deliberate)) var(--motion-easing-spring)," +
                    "opacity var(--motion-duration-(--motion-duration-slow)) var(--motion-easing-standard)",
                }}
              >
                <CoverPlate comic={comic} lifted={distance === 0} favorite={favorites.has(comic.id)} />
              </button>
            );
          })}
        </div>
      </div>

      {/* Légende du centre. Placée sous le carrousel plutôt que sur la
          couverture : rien ne doit recouvrir l'illustration, qui est le sujet. */}
      {current && (
        <div className="pointer-events-none absolute inset-x-0 bottom-3 flex flex-col items-center gap-0.5 px-4">
          <p className="max-w-[60ch] truncate text-title font-semibold text-fg">
            {current.title}
          </p>
          <p className="text-meta text-muted">
            {current.seriesName ? `${current.seriesName} · ` : ""}
            {current.pageCount} pages
            <span className="ml-2 tabular-nums text-subtle">
              {center + 1} / {comics.length}
            </span>
          </p>
        </div>
      )}

      <Arrow side="left" disabled={center === 0} onClick={() => goTo(center - 1)} />
      <Arrow
        side="right"
        disabled={center === comics.length - 1}
        onClick={() => goTo(center + 1)}
      />
    </section>
  );
}

/**
 * Une couverture et son reflet.
 *
 * Le reflet n'est pas de l'ornement : il pose les couvertures sur une surface
 * et donne au carrousel sa profondeur. Sans lui, les images flottent.
 */
function CoverPlate({
  comic,
  lifted,
  favorite,
}: {
  comic: Comic;
  lifted: boolean;
  favorite: boolean;
}) {
  const src = imageURL(comic.coverPath, { width: 640 });

  return (
    <span className="block" style={{ transformStyle: "preserve-3d" }}>
      <span
        className={cx(
          "relative block overflow-hidden rounded-[4px] bg-surface-sunken transition-shadow",
          lifted ? "shadow-[0_24px_48px_-12px_rgb(0_0_0/0.55)]" : "shadow-[0_8px_20px_-8px_rgb(0_0_0/0.4)]",
        )}
        style={{ aspectRatio: 0.7 }}
      >
        {comic.coverPlaceholder && (
          <span
            aria-hidden="true"
            className="absolute inset-0 scale-110 blur-lg"
            style={{
              backgroundImage: `url("${comic.coverPlaceholder}")`,
              backgroundSize: "cover",
              backgroundPosition: "center",
            }}
          />
        )}
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          draggable={false}
          className="relative size-full object-cover"
        />

        {favorite && (
          <span className="absolute left-1.5 top-1.5 grid size-5 place-items-center rounded-full bg-black/60">
            <svg viewBox="0 0 16 16" fill="currentColor" className="size-3 text-danger" aria-hidden="true">
              <path d="M8 14S2 10.4 2 6.5A3.5 3.5 0 0 1 8 4a3.5 3.5 0 0 1 6 2.5C14 10.4 8 14 8 14Z" />
            </svg>
          </span>
        )}
      </span>

      {/*
        Le reflet montre le BAS de la couverture, retourné.

        L'image est rendue à sa hauteur pleine et retournée sur place ; la boîte,
        elle, n'en laisse voir que les premiers pixels. Le retournement amène
        donc le bas de l'illustration contre le bord de la couverture, ce qui est
        la seule disposition qui se lise comme un reflet — mirer le haut
        produirait une image sans rapport avec ce qui la surplombe.
      */}
      <span
        aria-hidden="true"
        className="block overflow-hidden"
        style={{
          height: REFLECT_H,
          maskImage: "linear-gradient(to bottom, rgb(0 0 0 / 0.3), transparent 92%)",
          WebkitMaskImage: "linear-gradient(to bottom, rgb(0 0 0 / 0.3), transparent 92%)",
        }}
      >
        <img
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          draggable={false}
          className="w-full object-cover"
          style={{ height: COVER_H, transform: "scaleY(-1)" }}
        />
      </span>
    </span>
  );
}

function Arrow({
  side,
  disabled,
  onClick,
}: {
  side: "left" | "right";
  disabled: boolean;
  onClick: () => void;
}) {
  const t = useT();
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={side === "left" ? t("coverflow.previous") : t("coverflow.next")}
      className={cx(
        "pressable absolute top-1/2 z-[200] grid size-9 -translate-y-1/2 place-items-center rounded-full",
        "border border-border bg-surface/80 text-muted backdrop-blur",
        "hover:bg-surface-hover hover:text-fg disabled:pointer-events-none disabled:opacity-0",
        side === "left" ? "left-3" : "right-3",
      )}
    >
      <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
        <path
          d={side === "left" ? "M10 3 5 8l5 5" : "M6 3l5 5-5 5"}
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </button>
  );
}
