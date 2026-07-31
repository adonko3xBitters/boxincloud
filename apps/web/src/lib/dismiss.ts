"use client";

import { useEffect } from "react";
import type { RefObject } from "react";

/**
 * Referme un calque quand le pointeur va ailleurs, ou à Échap.
 *
 * Le test est un test d'appartenance — « la cible est-elle dans le calque ? » —
 * et non un `stopPropagation()` posé sur le calque. La nuance décide du
 * fonctionnement : sous l'App Router, React délègue ses écouteurs à `document`,
 * donc l'écouteur du calque et celui-ci vivent sur le MÊME nœud.
 * `stopPropagation()` n'arrête que les nœuds suivants, jamais les colisteners du
 * même nœud. Le calque se démontait donc au `pointerdown`, et le `click` qui
 * suivait ne trouvait plus l'élément visé : le menu se fermait sans rien faire.
 *
 * Un test d'appartenance ne dépend d'aucun ordre d'inscription.
 */
export function useDismissOnOutside(
  open: boolean,
  ref: RefObject<HTMLElement | null>,
  onClose: () => void,
) {
  useEffect(() => {
    if (!open) return undefined;

    function onPointerDown(event: PointerEvent) {
      if (!ref.current?.contains(event.target as Node)) onClose();
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown, true);

    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [open, ref, onClose]);
}
