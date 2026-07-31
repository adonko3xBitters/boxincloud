"use client";

import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Zoom du mode défilement continu.
 *
 * Il n'a rien à voir avec celui des modes page : là-bas, on agrandit une planche
 * et on s'y déplace. Ici, la lecture est un ruban vertical, et « agrandir »
 * signifie **élargir la colonne** — exactement ce que fait un lecteur de manga.
 *
 * La distinction n'est pas cosmétique. Appliquer une transformation à la page
 * casserait le défilement : le conteneur ne connaîtrait plus la vraie hauteur
 * de son contenu, la barre sauterait, et la détection de la page courante — qui
 * repose sur ce qui occupe le centre de l'écran — deviendrait fausse. En jouant
 * sur la largeur, tout le reste continue de fonctionner sans le savoir.
 *
 * Le facteur est persisté : quelqu'un qui lit des webtoons a réglé sa largeur
 * une fois et n'a pas envie de la régler à chaque album.
 */

const STORAGE_KEY = "boxincloud.reader.column";

/** Bornes du facteur de largeur. */
const MIN = 0.4;
const MAX = 2.5;

export type ColumnZoom = {
  /** Facteur appliqué à la largeur de la colonne. */
  scale: number;
  /** À poser sur le conteneur défilant : il porte les gestes. */
  containerRef: React.RefObject<HTMLDivElement | null>;
  zoomBy: (factor: number) => void;
  reset: () => void;
};

function clamp(value: number): number {
  return Math.min(MAX, Math.max(MIN, value));
}

export function useColumnZoom(): ColumnZoom {
  const containerRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(1);

  // Lu après le montage, jamais pendant : le rendu statique ne connaît pas le
  // stockage local, et l'y lire ferait diverger le HTML du serveur.
  useEffect(() => {
    const stored = Number(localStorage.getItem(STORAGE_KEY));
    if (stored > 0) setScale(clamp(stored));
  }, []);

  const zoomBy = useCallback((factor: number) => {
    setScale((current) => {
      const next = clamp(current * factor);
      localStorage.setItem(STORAGE_KEY, String(next));
      return next;
    });
  }, []);

  const reset = useCallback(() => {
    setScale(1);
    localStorage.setItem(STORAGE_KEY, "1");
  }, []);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;

    /** Pointeurs actifs : deux doigts font un pincement, un seul défile. */
    const pointers = new Map<number, { x: number; y: number }>();
    let pinchDistance = 0;

    function spread(): number {
      const [a, b] = [...pointers.values()];
      if (!a || !b) return 0;
      return Math.hypot(a.x - b.x, a.y - b.y);
    }

    function onPointerDown(event: PointerEvent) {
      pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });
      if (pointers.size === 2) pinchDistance = spread();
    }

    function onPointerMove(event: PointerEvent) {
      if (!pointers.has(event.pointerId)) return;
      pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });

      if (pointers.size !== 2) return;

      const distance = spread();
      if (pinchDistance > 0 && distance > 0) {
        zoomBy(distance / pinchDistance);
        // Le défilement natif ne doit pas s'ajouter au pincement : deux doigts
        // qui s'écartent feraient sinon défiler la page en même temps.
        event.preventDefault();
      }
      pinchDistance = distance;
    }

    function onPointerUp(event: PointerEvent) {
      pointers.delete(event.pointerId);
      if (pointers.size < 2) pinchDistance = 0;
    }

    /*
      Pincement de pavé tactile.

      Il arrive sous forme de `wheel` avec `ctrlKey` — convention du navigateur,
      pas de l'utilisateur, qui n'a appuyé sur aucune touche. La molette seule
      reste au défilement, qui est ce qu'on attend d'elle ici.
    */
    function onWheel(event: WheelEvent) {
      if (!event.ctrlKey) return;
      event.preventDefault();
      zoomBy(event.deltaY < 0 ? 1.08 : 1 / 1.08);
    }

    container.addEventListener("pointerdown", onPointerDown);
    container.addEventListener("pointermove", onPointerMove, { passive: false });
    container.addEventListener("pointerup", onPointerUp);
    container.addEventListener("pointercancel", onPointerUp);
    container.addEventListener("wheel", onWheel, { passive: false });

    return () => {
      container.removeEventListener("pointerdown", onPointerDown);
      container.removeEventListener("pointermove", onPointerMove);
      container.removeEventListener("pointerup", onPointerUp);
      container.removeEventListener("pointercancel", onPointerUp);
      container.removeEventListener("wheel", onWheel);
    };
  }, [zoomBy]);

  return { scale, containerRef, zoomBy, reset };
}
