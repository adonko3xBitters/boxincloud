"use client";

/**
 * Zoom et déplacement dans une planche.
 *
 * Une case de BD porte souvent un texte petit et un dessin dense : pouvoir
 * agrandir n'est pas un agrément, c'est ce qui rend certaines planches lisibles.
 *
 * La transformation est appliquée directement au style de l'élément, pas via
 * l'état React. Un pincement émet plus de cent événements par seconde ; les
 * faire passer par un rendu React ferait saccader le geste. React ne voit ici
 * que ce dont l'interface a besoin : le niveau de zoom, et seulement quand il
 * s'est stabilisé.
 */

import { useCallback, useEffect, useRef, useState } from "react";

const MIN_SCALE = 1;
const MAX_SCALE = 5;

/** Niveau atteint par un double-clic. Assez pour lire une bulle, pas plus. */
const DOUBLE_TAP_SCALE = 2.5;

/** En deçà, on considère qu'on est revenu à la taille normale. */
const ZOOMED_EPSILON = 1.01;

type Transform = { scale: number; x: number; y: number };

const IDENTITY: Transform = { scale: 1, x: 0, y: 0 };

export type ZoomController = {
  /** Élément qui capte les gestes. */
  viewportRef: React.RefObject<HTMLDivElement | null>;
  /** Élément transformé. */
  contentRef: React.RefObject<HTMLDivElement | null>;
  /**
   * Niveau de zoom stabilisé.
   *
   * Mis à jour à la fin d'un geste, pas pendant : il sert à décider quelle
   * définition d'image demander, et changer d'image à chaque image du
   * pincement n'aurait aucun sens.
   */
  settledScale: number;
  /** Vrai dès que la planche est agrandie. */
  zoomed: boolean;
  reset: () => void;
  zoomBy: (factor: number) => void;
};

export function useZoom(resetKey: unknown): ZoomController {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);

  const transform = useRef<Transform>({ ...IDENTITY });
  const frame = useRef<number | null>(null);

  const [settledScale, setSettledScale] = useState(1);
  const [zoomed, setZoomed] = useState(false);

  /**
   * Borne le déplacement pour que la planche ne puisse pas quitter l'écran.
   *
   * Sans cette contrainte, un geste un peu vif fait disparaître la page et
   * laisse un rectangle noir, sans indice pour revenir.
   */
  const clamp = useCallback((next: Transform): Transform => {
    const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, next.scale));

    const box = viewportRef.current?.getBoundingClientRect();
    if (!box) return { ...next, scale };

    const maxX = (box.width * (scale - 1)) / 2;
    const maxY = (box.height * (scale - 1)) / 2;

    return {
      scale,
      x: Math.min(maxX, Math.max(-maxX, next.x)),
      y: Math.min(maxY, Math.max(-maxY, next.y)),
    };
  }, []);

  const paint = useCallback(() => {
    if (frame.current !== null) return;
    frame.current = requestAnimationFrame(() => {
      frame.current = null;
      const node = contentRef.current;
      if (!node) return;
      const { scale, x, y } = transform.current;
      node.style.transform = `translate3d(${x}px, ${y}px, 0) scale(${scale})`;
    });
  }, []);

  const apply = useCallback(
    (next: Transform) => {
      transform.current = clamp(next);
      paint();
      setZoomed(transform.current.scale > ZOOMED_EPSILON);
    },
    [clamp, paint],
  );

  const settle = useCallback(() => {
    setSettledScale(transform.current.scale);
  }, []);

  const reset = useCallback(() => {
    apply({ ...IDENTITY });
    setSettledScale(1);
  }, [apply]);

  /**
   * Zoome autour d'un point.
   *
   * Le point visé doit rester sous le doigt : c'est ce qui distingue un zoom
   * qui obéit d'un zoom qui dérive. On corrige donc le déplacement de la
   * différence d'échelle appliquée à la distance entre ce point et le centre.
   */
  const zoomAt = useCallback(
    (factor: number, clientX: number, clientY: number) => {
      const box = viewportRef.current?.getBoundingClientRect();
      const current = transform.current;
      const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, current.scale * factor));
      const ratio = scale / current.scale;

      if (!box) {
        apply({ scale, x: current.x, y: current.y });
        return;
      }

      const originX = clientX - (box.left + box.width / 2);
      const originY = clientY - (box.top + box.height / 2);

      apply({
        scale,
        x: originX - (originX - current.x) * ratio,
        y: originY - (originY - current.y) * ratio,
      });
    },
    [apply],
  );

  const zoomBy = useCallback(
    (factor: number) => {
      const box = viewportRef.current?.getBoundingClientRect();
      zoomAt(
        factor,
        box ? box.left + box.width / 2 : 0,
        box ? box.top + box.height / 2 : 0,
      );
      settle();
    },
    [zoomAt, settle],
  );

  // Changer de page remet la planche à plat : rester zoomé sur un coin
  // arbitraire de la page suivante n'a jamais de sens.
  useEffect(() => {
    reset();
  }, [resetKey, reset]);

  // ─── Gestes ────────────────────────────────────────────────────────────────

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;

    /** Pointeurs actifs, pour distinguer le déplacement du pincement. */
    const pointers = new Map<number, { x: number; y: number }>();
    let pinchDistance = 0;
    let lastTap = 0;

    function distanceBetween(points: Array<{ x: number; y: number }>): number {
      const [a, b] = points;
      if (!a || !b) return 0;
      return Math.hypot(a.x - b.x, a.y - b.y);
    }

    function midpointOf(points: Array<{ x: number; y: number }>) {
      const [a, b] = points;
      if (!a || !b) return { x: 0, y: 0 };
      return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
    }

    function onPointerDown(event: PointerEvent) {
      pointers.set(event.pointerId, { x: event.clientX, y: event.clientY });

      if (pointers.size === 2) {
        pinchDistance = distanceBetween([...pointers.values()]);
      }
    }

    function onPointerMove(event: PointerEvent) {
      const previous = pointers.get(event.pointerId);
      if (!previous) return;

      const position = { x: event.clientX, y: event.clientY };
      pointers.set(event.pointerId, position);

      if (pointers.size === 2) {
        const points = [...pointers.values()];
        const distance = distanceBetween(points);
        if (pinchDistance > 0 && distance > 0) {
          const centre = midpointOf(points);
          zoomAt(distance / pinchDistance, centre.x, centre.y);
        }
        pinchDistance = distance;
        event.preventDefault();
        return;
      }

      // Un seul doigt ne déplace que si la planche est agrandie : sinon le
      // glissement appartient aux zones de navigation, qui tournent la page.
      if (transform.current.scale > ZOOMED_EPSILON) {
        const current = transform.current;
        apply({
          scale: current.scale,
          x: current.x + (position.x - previous.x),
          y: current.y + (position.y - previous.y),
        });
        event.preventDefault();
      }
    }

    function onPointerUp(event: PointerEvent) {
      pointers.delete(event.pointerId);
      if (pointers.size < 2) pinchDistance = 0;
      if (pointers.size === 0) settle();
    }

    /**
     * Molette.
     *
     * Un pincement de pavé tactile arrive sous forme de `wheel` avec `ctrlKey` —
     * convention du navigateur, pas de l'utilisateur, qui n'a appuyé sur rien.
     * La molette seule est laissée au défilement.
     */
    function onWheel(event: WheelEvent) {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      zoomAt(Math.exp(-event.deltaY / 180), event.clientX, event.clientY);
      settle();
    }

    /** Double-clic ou double-tap : bascule entre taille normale et agrandie. */
    function onDoubleActivate(clientX: number, clientY: number) {
      if (transform.current.scale > ZOOMED_EPSILON) {
        reset();
      } else {
        zoomAt(DOUBLE_TAP_SCALE, clientX, clientY);
        settle();
      }
    }

    function onDoubleClick(event: MouseEvent) {
      event.preventDefault();
      onDoubleActivate(event.clientX, event.clientY);
    }

    // Le double-tap tactile est reconstruit à la main : `dblclick` n'est pas
    // émis de façon fiable sur tous les navigateurs mobiles.
    function onPointerUpTap(event: PointerEvent) {
      if (event.pointerType !== "touch") return;
      const now = event.timeStamp;
      if (now - lastTap < 300) {
        onDoubleActivate(event.clientX, event.clientY);
        lastTap = 0;
      } else {
        lastTap = now;
      }
    }

    viewport.addEventListener("pointerdown", onPointerDown);
    viewport.addEventListener("pointermove", onPointerMove, { passive: false });
    viewport.addEventListener("pointerup", onPointerUp);
    viewport.addEventListener("pointerup", onPointerUpTap);
    viewport.addEventListener("pointercancel", onPointerUp);
    viewport.addEventListener("wheel", onWheel, { passive: false });
    viewport.addEventListener("dblclick", onDoubleClick);

    return () => {
      viewport.removeEventListener("pointerdown", onPointerDown);
      viewport.removeEventListener("pointermove", onPointerMove);
      viewport.removeEventListener("pointerup", onPointerUp);
      viewport.removeEventListener("pointerup", onPointerUpTap);
      viewport.removeEventListener("pointercancel", onPointerUp);
      viewport.removeEventListener("wheel", onWheel);
      viewport.removeEventListener("dblclick", onDoubleClick);
    };
  }, [apply, reset, settle, zoomAt]);

  // Une fenêtre redimensionnée invalide les bornes de déplacement calculées à
  // l'ancienne taille : la planche pourrait rester hors champ.
  useEffect(() => {
    function onResize() {
      apply(transform.current);
    }
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [apply]);

  useEffect(() => {
    return () => {
      if (frame.current !== null) cancelAnimationFrame(frame.current);
    };
  }, []);

  return { viewportRef, contentRef, settledScale, zoomed, reset, zoomBy };
}
