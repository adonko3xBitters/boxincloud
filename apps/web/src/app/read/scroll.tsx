"use client";

import { useEffect, useRef } from "react";

import { imageURL } from "@/lib/api/client";
import type { ColumnZoom } from "@/lib/reader/column";
import type { ManifestPage } from "@/lib/reader/pages";

/**
 * Mode défilement continu — la lecture des webtoons, et le mode le plus
 * naturel sur téléphone.
 *
 * Les pages s'enchaînent verticalement, sans coupure. Deux exigences :
 *
 *  - la place de chaque page est réservée par son ratio, connu du manifeste ;
 *    sans cela la barre de défilement bondirait à chaque image chargée, et la
 *    position de lecture sauterait ;
 *  - la page courante est déduite de ce qui occupe le centre de l'écran, pour
 *    que la progression enregistrée corresponde à ce qu'on regarde.
 */
export function ScrollReader({
  comicId,
  pages,
  width,
  startPage,
  onPageChange,
  column,
}: {
  comicId: string;
  pages: ManifestPage[];
  width: number;
  startPage: number;
  onPageChange: (page: number) => void;

  // Fourni par le lecteur plutôt que créé ici : le clavier doit pouvoir piloter
  // le même zoom, et il vit un cran au-dessus.
  column: ColumnZoom;
}) {
  /*
    Le zoom du défilement continu élargit la COLONNE, il ne transforme pas la
    page. Transformer casserait tout le reste : le conteneur ne connaîtrait
    plus la vraie hauteur de son contenu, la barre de défilement sauterait, et
    la détection de la page courante — qui repose sur ce qui occupe le centre
    de l'écran — deviendrait fausse.
  */
  const container = column.containerRef;
  const refs = useRef(new Map<number, HTMLElement>());
  const jumped = useRef(false);

  // Position d'entrée : on saute à la page où l'on s'était arrêté. Une seule
  // fois — sans ce garde, tout rendu ramènerait au point de départ.
  useEffect(() => {
    if (jumped.current || pages.length === 0) return;

    const target = refs.current.get(startPage);
    if (target) {
      target.scrollIntoView({ block: "start" });
      jumped.current = true;
    }
  }, [pages, startPage]);

  /*
   * Détection de la page courante.
   *
   * IntersectionObserver plutôt qu'un écouteur de défilement : le navigateur
   * calcule les intersections hors du fil principal, là où un `onScroll`
   * exécuterait du JavaScript à chaque image de l'animation — ce qui se voit
   * immédiatement sur un défilement long.
   *
   * La marge réduit la zone d'observation à une bande horizontale au centre de
   * l'écran : la page « courante » est celle qu'on regarde, pas celle qui
   * dépasse en haut.
   */
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const index = Number((entry.target as HTMLElement).dataset.page);
          if (Number.isFinite(index)) onPageChange(index);
        }
      },
      { rootMargin: "-45% 0px -45% 0px", threshold: 0 },
    );

    for (const element of refs.current.values()) observer.observe(element);
    return () => observer.disconnect();
  }, [pages, onPageChange]);

  return (
    <div
      ref={container}
      className="h-dvh overflow-y-auto overscroll-contain"
      // Le navigateur ne doit pas s'occuper du pincement : c'est nous qui le
      // traduisons en largeur de colonne. Le laisser faire superposerait son
      // zoom de page au nôtre.
      style={{ touchAction: "pan-y" }}
    >
      <div
        className="mx-auto flex flex-col"
        style={{ maxWidth: `${Math.round(1000 * column.scale)}px` }}
      >
        {pages.map((page, position) => {
          // Ratio connu du manifeste : la place est réservée avant l'arrivée
          // de l'image, donc aucun saut de défilement.
          const ratio =
            page.width && page.height ? `${page.width} / ${page.height}` : "0.7";

          return (
            <div
              key={page.index}
              data-page={page.index}
              ref={(element) => {
                if (element) refs.current.set(page.index, element);
                else refs.current.delete(page.index);
              }}
              style={{ aspectRatio: ratio }}
              className="w-full bg-neutral-900"
            >
              <img
                src={imageURL(`/comics/${comicId}/pages/${page.index}`, { width })}
                alt={`Page ${page.index + 1}`}
                // Les trois premières pages sans attendre : ce sont celles
                // qu'on voit en arrivant. Le reste au défilement, sans quoi
                // ouvrir un album de deux cents planches déclencherait deux
                // cents requêtes d'un coup.
                loading={position < 3 ? "eager" : "lazy"}
                decoding="async"
                draggable={false}
                className="size-full select-none object-contain"
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}
