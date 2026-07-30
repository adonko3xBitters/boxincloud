"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { cx } from "./ui";

/**
 * Menu contextuel au clic droit.
 *
 * Une bibliothèque se gère comme un gestionnaire de fichiers : le clic droit y
 * est le geste attendu pour agir sur ce qu'on désigne. L'imposer via un bouton
 * caché au survol oblige à viser une cible minuscule pour une action courante.
 *
 * Le menu s'ouvre là où le curseur se trouve, et se recadre s'il déborde — un
 * menu dont la moitié sort de l'écran ne vaut pas mieux que pas de menu.
 */

export type MenuItem =
  | { kind: "separator" }
  | {
      kind?: "item";
      label: string;
      onSelect: () => void;
      destructive?: boolean;
      disabled?: boolean;
    };

type Position = { x: number; y: number };

export function ContextMenu({
  position,
  items,
  onClose,
}: {
  position: Position | null;
  items: MenuItem[];
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [placed, setPlaced] = useState<Position | null>(null);

  // Le recadrage se fait avant peinture : mesurer après aurait fait apparaître
  // le menu au mauvais endroit pendant une image.
  useLayoutEffect(() => {
    if (!position || !ref.current) {
      setPlaced(null);
      return;
    }

    const box = ref.current.getBoundingClientRect();
    const margin = 8;

    setPlaced({
      x: Math.min(position.x, window.innerWidth - box.width - margin),
      y: Math.min(position.y, window.innerHeight - box.height - margin),
    });
  }, [position, items.length]);

  useEffect(() => {
    if (!position) return undefined;

    function close() {
      onClose();
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }

    // Un clic ailleurs referme, y compris un clic droit ailleurs : sans quoi
    // deux menus se superposeraient.
    document.addEventListener("pointerdown", close);
    document.addEventListener("contextmenu", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    document.addEventListener("keydown", onKeyDown, true);

    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("contextmenu", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [position, onClose]);

  if (!position) return null;

  return (
    <div
      ref={ref}
      role="menu"
      onPointerDown={(e) => e.stopPropagation()}
      onContextMenu={(e) => e.preventDefault()}
      style={{
        left: placed?.x ?? position.x,
        top: placed?.y ?? position.y,
        // Invisible tant que la position n'est pas arrêtée : un menu qui saute
        // d'un coin à l'autre au premier rendu est plus gênant qu'un menu qui
        // apparaît une image plus tard.
        visibility: placed ? "visible" : "hidden",
      }}
      className="fade-in fixed z-[80] w-56 rounded-lg border border-border bg-surface-raised p-1 shadow-2xl"
    >
      {items.map((item, index) =>
        item.kind === "separator" ? (
          <span key={index} className="my-1 block h-px bg-border" />
        ) : (
          <button
            key={index}
            role="menuitem"
            disabled={item.disabled}
            onClick={() => {
              onClose();
              item.onSelect();
            }}
            className={cx(
              "pressable w-full rounded px-2.5 py-1.5 text-left text-ui",
              "disabled:opacity-40 disabled:cursor-not-allowed",
              item.destructive
                ? "text-danger hover:bg-danger/10"
                : "text-muted hover:bg-surface-hover hover:text-fg",
            )}
          >
            {item.label}
          </button>
        ),
      )}
    </div>
  );
}

/**
 * État d'un menu contextuel.
 *
 * Regroupé dans un hook : chaque appelant aurait sinon sa propre paire
 * position/contenu, et une façon différente de la remettre à zéro.
 */
export function useContextMenu<T>() {
  const [state, setState] = useState<{ position: Position; target: T } | null>(null);

  return {
    position: state?.position ?? null,
    target: state?.target ?? null,
    open: (event: React.MouseEvent, target: T) => {
      event.preventDefault();
      event.stopPropagation();
      setState({ position: { x: event.clientX, y: event.clientY }, target });
    },
    close: () => setState(null),
  };
}
