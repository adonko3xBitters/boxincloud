"use client";

/**
 * Composition des pages et préchargement.
 *
 * Deux préoccupations que le composant de lecture n'a pas à porter :
 *
 *  - appairer les pages en mode double, en tenant compte des doubles planches
 *    qui doivent rester seules ;
 *  - précharger les pages voisines et libérer celles qui s'éloignent.
 */

import { useEffect, useMemo, useRef } from "react";

import type { Manifest } from "@/lib/api/client";
import { imageURL } from "@/lib/api/client";

export type ManifestPage = Manifest["pages"][number];

/** Un feuillet affiché : une page, ou deux appariées. */
export type Spread = {
  /** Index des pages composant le feuillet, dans l'ordre d'affichage. */
  pages: number[];
};

/**
 * Compose les feuillets d'un album.
 *
 * En mode double, la couverture reste seule — comme sur un album physique,
 * dont la première de couverture n'a pas de vis-à-vis. Une double planche
 * reste seule également : l'apparier écraserait les deux images.
 */
export function buildSpreads(pages: ManifestPage[], double: boolean): Spread[] {
  if (!double) {
    return pages.map((page) => ({ pages: [page.index] }));
  }

  const spreads: Spread[] = [];
  let i = 0;

  // La couverture ouvre seule.
  if (pages.length > 0) {
    spreads.push({ pages: [pages[0]!.index] });
    i = 1;
  }

  while (i < pages.length) {
    const page = pages[i]!;

    if (page.isDouble) {
      spreads.push({ pages: [page.index] });
      i += 1;
      continue;
    }

    const next = pages[i + 1];
    if (next && !next.isDouble) {
      spreads.push({ pages: [page.index, next.index] });
      i += 2;
    } else {
      spreads.push({ pages: [page.index] });
      i += 1;
    }
  }

  return spreads;
}

/** Trouve le feuillet contenant une page donnée. */
export function spreadIndexOfPage(spreads: Spread[], page: number): number {
  const found = spreads.findIndex((spread) => spread.pages.includes(page));
  return found >= 0 ? found : 0;
}

// ─── Préchargement ───────────────────────────────────────────────────────────

/**
 * Fenêtre de préchargement.
 *
 * Trois pages en avant, une en arrière. En avant parce que c'est le sens de
 * lecture ; une seule en arrière parce que revenir est rare, mais assez
 * fréquent pour qu'un retour instantané se remarque.
 *
 * Plus large gaspillerait de la bande passante sur un backend distant — et le
 * coût y est réel, chaque page étant une requête au serveur d'objets.
 */
const AHEAD = 3;
const BEHIND = 1;

/**
 * Précharge les pages voisines de la position courante.
 *
 * Les images préchargées sont conservées dans une Map pour que le navigateur
 * ne les libère pas immédiatement, puis relâchées dès qu'elles sortent de la
 * fenêtre — sans quoi lire un album de deux cents planches finirait par saturer
 * la mémoire de l'onglet.
 */
export function usePrefetch(comicId: string, pages: number[], current: number, width: number) {
  const held = useRef(new Map<number, HTMLImageElement>());

  const window = useMemo(() => {
    const from = Math.max(0, current - BEHIND);
    const to = Math.min(pages.length - 1, current + AHEAD);

    const indices: number[] = [];
    for (let i = from; i <= to; i++) {
      const page = pages[i];
      if (page !== undefined) indices.push(page);
    }
    return indices;
  }, [pages, current]);

  useEffect(() => {
    if (!comicId) return;

    const cache = held.current;
    const wanted = new Set(window);

    for (const page of window) {
      if (cache.has(page)) continue;

      const img = new Image();
      img.decoding = "async";
      img.src = imageURL(`/comics/${comicId}/pages/${page}`, { width });
      cache.set(page, img);
    }

    // Libère ce qui est sorti de la fenêtre.
    for (const [page, img] of cache) {
      if (!wanted.has(page)) {
        img.src = "";
        cache.delete(page);
      }
    }
  }, [comicId, window, width]);

  // Au démontage, tout est relâché : quitter le lecteur ne doit rien laisser
  // derrière.
  useEffect(() => {
    const cache = held.current;
    return () => {
      for (const img of cache.values()) img.src = "";
      cache.clear();
    };
  }, []);
}

/**
 * Largeur à demander au serveur.
 *
 * Arrondie à des paliers plutôt qu'exacte : chaque largeur distincte crée une
 * variante dans le cache serveur, et un redimensionnement de fenêtre en
 * générerait des dizaines. Les paliers correspondent aux tailles d'écran
 * usuelles.
 */
export function pageWidthFor(viewportWidth: number, devicePixelRatio = 1): number {
  const target = viewportWidth * Math.min(devicePixelRatio, 2);

  for (const step of [800, 1200, 1600, 2000, 2400]) {
    if (target <= step) return step;
  }
  return 2400;
}
