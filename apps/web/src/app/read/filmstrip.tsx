"use client";

import { useEffect, useRef } from "react";

import { useT } from "@/i18n";
import { cx } from "@/components/ui";
import { imageURL } from "@/lib/api/client";
import type { ManifestPage } from "@/lib/reader/pages";

/**
 * Sélecteur de pages en vignettes.
 *
 * Le curseur de progression dit où l'on est ; il ne dit pas ce qu'il y a. Pour
 * retrouver une planche — celle avec le vaisseau, celle où le récit bascule —
 * il faut la voir. C'est la seule façon de naviguer dans un album qu'on relit.
 *
 * Les vignettes ne sont montées que lorsque la bande est ouverte : un album de
 * deux cents planches déclencherait sinon deux cents requêtes au serveur
 * d'objets, pour une bande que personne n'a demandée.
 */
/**
 * Largeur demandée pour une vignette.
 *
 * Elle correspond exactement au plus petit palier du serveur. Demander moins
 * ne servirait à rien — la largeur serait arrondie au même palier — et le
 * demander explicitement garde l'URL du client identique à ce qui est servi,
 * donc une seule entrée dans le cache du navigateur.
 */
const THUMB_WIDTH = 320;

export function Filmstrip({
  open,
  comicId,
  pages,
  current,
  onSelect,
  onClose,
}: {
  open: boolean;
  comicId: string;
  pages: ManifestPage[];
  current: number;
  onSelect: (page: number) => void;
  onClose: () => void;
}) {
  const t = useT();
  const scroller = useRef<HTMLDivElement>(null);
  const activeItem = useRef<HTMLButtonElement>(null);

  // La vignette courante doit être sous les yeux à l'ouverture, et le rester
  // quand on tourne les pages sans refermer la bande.
  useEffect(() => {
    if (!open) return;
    activeItem.current?.scrollIntoView({
      block: "nearest",
      inline: "center",
      behavior: "smooth",
    });
  }, [open, current]);

  // Le défilement vertical d'une molette classique est converti en défilement
  // horizontal : c'est le seul geste dont dispose une souris sans molette
  // latérale, et une bande qui ne défile pas est inutilisable.
  useEffect(() => {
    const node = scroller.current;
    if (!node || !open) return undefined;

    function onWheel(event: WheelEvent) {
      if (event.ctrlKey || Math.abs(event.deltaX) > Math.abs(event.deltaY)) return;
      event.preventDefault();
      node!.scrollLeft += event.deltaY;
    }

    node.addEventListener("wheel", onWheel, { passive: false });
    return () => node.removeEventListener("wheel", onWheel);
  }, [open]);

  return (
    <div
      inert={!open}
      aria-label={t("filmstrip.label")}
      className={cx(
        "absolute inset-x-0 bottom-0 z-30 border-t border-white/10 bg-black/85 backdrop-blur",
        "transition-transform duration-(--motion-duration-slow) ease-emphasized",
        open ? "translate-y-0" : "translate-y-full",
      )}
    >
      <div className="flex items-center justify-between px-4 pb-1 pt-2">
        <p className="text-micro uppercase tracking-wider text-white/40">
          {pages.length} pages
        </p>
        <button
          onClick={onClose}
          aria-label={t("filmstrip.close")}
          className="pressable grid size-7 place-items-center rounded text-white/60 hover:bg-white/10 hover:text-white"
        >
          <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
            <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          </svg>
        </button>
      </div>

      <div
        ref={scroller}
        className="no-scrollbar flex gap-2 overflow-x-auto px-4 pb-3"
      >
        {/* Le montage conditionnel est le seul frein au coût réseau : `lazy`
            ne suffirait pas, un conteneur horizontal considérant comme visible
            tout ce qui s'y trouve. */}
        {open &&
          pages.map((page) => {
            const active = page.index === current;
            return (
              <button
                key={page.index}
                ref={active ? activeItem : undefined}
                onClick={() => onSelect(page.index)}
                aria-current={active ? "true" : undefined}
                aria-label={t("filmstrip.goToPage", { page: page.index + 1 })}
                className={cx(
                  "group relative shrink-0 overflow-hidden rounded-[3px]",
                  "transition-transform duration-(--motion-duration-fast) ease-emphasized",
                  "hover:-translate-y-0.5",
                  active
                    ? "outline outline-2 outline-accent"
                    : "opacity-60 hover:opacity-100",
                )}
                style={{ width: 68 }}
              >
                <img
                  src={imageURL(`/comics/${comicId}/pages/${page.index}`, { width: THUMB_WIDTH })}
                  alt=""
                  loading="lazy"
                  decoding="async"
                  draggable={false}
                  className="block w-full bg-neutral-800 object-cover"
                  style={{ aspectRatio: page.width && page.height ? page.width / page.height : 0.7 }}
                />
                <span
                  className={cx(
                    "absolute inset-x-0 bottom-0 bg-black/70 py-0.5 text-center font-mono text-micro tabular-nums",
                    active ? "text-white" : "text-white/60",
                  )}
                >
                  {page.index + 1}
                </span>
              </button>
            );
          })}
      </div>
    </div>
  );
}
