"use client";

/**
 * Tableaux denses du module.
 *
 * Un gestionnaire de téléchargements n'est pas une galerie : on y compare des
 * lignes entre elles, on cherche celle qui ne bouge plus, on repère la source
 * qui monopolise la bande passante. Ce travail demande des lignes serrées et
 * des chiffres alignés — pas des cartes ni des espacements confortables, qui
 * font tenir six lignes là où il en faudrait trente.
 *
 * Une grille CSS plutôt qu'un `<table>` : les colonnes gardent la même largeur
 * d'un panneau à l'autre sans qu'un navigateur ne les recalcule au gré du
 * contenu, et une ligne peut se déplier pour en révéler d'autres sans casser
 * l'alignement de celles du dessus.
 */

import { createContext, useContext, type ReactNode } from "react";

import { cx } from "@/components/ui";
import { useT, type MessageKey } from "@/i18n";

export type Column = {
  key: string;
  /** `null` pour une colonne sans en-tête — une pastille d'état, par exemple. */
  label: MessageKey | null;
  /** Largeur de piste CSS : `120px`, `minmax(160px, 2fr)`… */
  width: string;
  align?: "left" | "right" | "center";
};

/*
  Le gabarit des colonnes voyage par contexte.

  L'alternative serait de le passer à chaque ligne. Sur un tableau de trois
  cents lignes, la moindre divergence entre l'en-tête et une ligne décale
  toute la grille — et c'est le genre de faute qu'un oubli de prop produit
  sans rien signaler.
*/
const TemplateContext = createContext<string>("");

export function Table({
  columns,
  minWidth,
  label,
  children,
}: {
  columns: Column[];
  /** Largeur en dessous de laquelle le tableau défile plutôt que de se tasser. */
  minWidth: number;
  label: string;
  children: ReactNode;
}) {
  const t = useT();
  const template = columns.map((column) => column.width).join(" ");

  return (
    <div className="min-w-0 overflow-x-auto rounded-lg border border-border bg-surface">
      <div style={{ minWidth }} role="table" aria-label={label}>
        <div
          role="row"
          className={cx(
            "sticky top-0 z-10 grid items-center gap-x-3 border-b border-border bg-surface px-3 py-1.5",
            "text-micro font-semibold uppercase tracking-wide text-subtle",
          )}
          style={{ gridTemplateColumns: template }}
        >
          {columns.map((column) => (
            <span
              key={column.key}
              role="columnheader"
              className={cx(
                "truncate",
                column.align === "right" && "text-right",
                column.align === "center" && "text-center",
              )}
            >
              {column.label ? t(column.label) : ""}
            </span>
          ))}
        </div>

        <TemplateContext.Provider value={template}>{children}</TemplateContext.Provider>
      </div>
    </div>
  );
}

export function Row({
  children,
  onClick,
  expanded,
  label,
  striped = false,
}: {
  children: ReactNode;
  onClick?: () => void;
  /** Présent, la ligne se déplie : elle devient un bouton pour le clavier. */
  expanded?: boolean;
  label?: string;
  striped?: boolean;
}) {
  const template = useContext(TemplateContext);

  const common = cx(
    "grid w-full items-center gap-x-3 border-b border-border/60 px-3 py-1.5 text-left text-meta",
    "transition-colors duration-(--motion-duration-fast)",
    striped && "bg-surface-sunken/40",
    onClick && "cursor-default hover:bg-surface-hover",
  );

  if (!onClick) {
    return (
      <div role="row" className={common} style={{ gridTemplateColumns: template }}>
        {children}
      </div>
    );
  }

  /*
    Un vrai `<button>` pour une ligne qui se déplie.

    Un `<div onClick>` marche à la souris et nulle part ailleurs : ni au
    clavier, ni au lecteur d'écran, qui n'annoncerait même pas qu'il y a
    quelque chose à ouvrir.
  */
  return (
    <button
      type="button"
      onClick={onClick}
      aria-expanded={expanded}
      aria-label={label}
      className={cx(common, "pressable")}
      style={{ gridTemplateColumns: template }}
    >
      {children}
    </button>
  );
}

/** Cellule numérique : alignée à droite, chiffres de largeur fixe. */
export function Num({
  children,
  title,
  tone = "muted",
}: {
  children: ReactNode;
  title?: string;
  tone?: "fg" | "muted" | "subtle" | "success" | "danger" | "warning";
}) {
  const tones = {
    fg: "text-fg",
    muted: "text-muted",
    subtle: "text-subtle",
    success: "text-success",
    danger: "text-danger",
    warning: "text-warning",
  };

  return (
    <span className={cx("truncate text-right tabular-nums", tones[tone])} title={title}>
      {children}
    </span>
  );
}

/** Cellule textuelle, tronquée plutôt que repliée : une ligne reste une ligne. */
export function Text({
  children,
  title,
  tone = "muted",
}: {
  children: ReactNode;
  title?: string;
  tone?: "fg" | "muted" | "subtle" | "danger";
}) {
  const tones = {
    fg: "text-fg",
    muted: "text-muted",
    subtle: "text-subtle",
    // Un détail d'échec doit se distinguer d'un détail ordinaire : c'est la
    // seule colonne où l'on cherche du regard ce qui ne va pas.
    danger: "text-danger",
  };

  return (
    <span className={cx("truncate", tones[tone])} title={title}>
      {children}
    </span>
  );
}

/**
 * Barre de progression d'une ligne.
 *
 * Deux pixels de haut, dans la ligne et non en dessous : la hauteur de ligne ne
 * change pas, et l'œil compare trente barres d'un seul balayage vertical — ce
 * qu'un pourcentage écrit ne permet jamais.
 *
 * La teinte porte l'état plutôt que de le répéter : une barre rouge se repère
 * avant qu'on ait lu la colonne « État ». Elle ne le porte pas SEULE — la
 * colonne existe, pour qui ne distingue pas ces couleurs.
 */
export function Progress({
  value,
  tone = "accent",
  label,
}: {
  /** Entre 0 et 1. */
  value: number;
  tone?: "accent" | "success" | "danger" | "idle";
  label: string;
}) {
  const tones = {
    accent: "bg-accent",
    success: "bg-success",
    danger: "bg-danger",
    idle: "bg-border-strong",
  };

  return (
    <span
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(value * 100)}
      aria-label={label}
      className="mt-1 block h-[3px] w-full overflow-hidden rounded-full bg-surface-sunken"
    >
      <span
        className={cx("block h-full rounded-full", tones[tone])}
        style={{ width: `${value * 100}%` }}
      />
    </span>
  );
}

/**
 * Rangée dépliée, sur toute la largeur du tableau.
 *
 * Elle sort de la grille des colonnes : ce qu'elle contient — les sources d'un
 * fichier — n'a pas les mêmes colonnes que la ligne qui l'ouvre, et les y
 * forcer produirait un alignement faux qu'on lirait comme une correspondance.
 */
export function Expansion({ children }: { children: ReactNode }) {
  return (
    <div className="border-b border-border bg-surface-sunken/60 px-3 py-2">{children}</div>
  );
}
