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

import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

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

/*
  ─── Tableau à filtre et pagination ─────────────────────────────────────────

  Les tableaux du module ne comptaient pas leurs lignes. Une liste de serveurs
  importée fait couramment trois cents entrées, une file d'attente d'envois en
  fait autant : tout afficher donnait une page qui défile sur des mètres, où
  retrouver une ligne demandait un Cmd-F du navigateur — qui ne cherche que ce
  qui est déjà rendu.

  Deux ajouts, et seulement deux.

  Un FILTRE, sur un texte que le panneau fournit lui-même pour chaque ligne.
  C'est le panneau qui sait ce qui a un sens à chercher — le nom et l'adresse
  d'un serveur, pas son compteur d'échecs — et une recherche sur tout produirait
  des correspondances incompréhensibles sur des chiffres.

  Une PAGINATION, à taille choisie. Elle est purement locale : l'API rend
  l'instantané entier, parce que le démon le rend entier lui aussi. Paginer côté
  serveur n'économiserait rien et ferait perdre le filtre sur les pages non
  chargées.

  Ce qui n'y est PAS, et pourquoi. Pas de tri par colonne : l'ordre vient du
  démon et porte du sens — la file est dans l'ordre où elle sera servie, les
  sources dans l'ordre où elles ont répondu. Un tri par défaut différent de cet
  ordre-là ferait croire à une priorité qui n'existe pas.
*/

const PAGE_SIZES = [25, 50, 100] as const;

export function DataTable<T>({
  items,
  columns,
  minWidth,
  label,
  searchText,
  renderRow,
  /** Placeholder du filtre, propre au panneau : « nom ou adresse… ». */
  filterHint,
}: {
  items: readonly T[];
  columns: Column[];
  minWidth: number;
  label: string;
  searchText: (item: T) => string;
  renderRow: (item: T, index: number) => ReactNode;
  filterHint?: string;
}) {
  const t = useT();
  const [filter, setFilter] = useState("");
  const [size, setSize] = useState<number>(PAGE_SIZES[0]);
  const [page, setPage] = useState(0);

  const needle = filter.trim().toLowerCase();
  const matching = useMemo(
    () => (needle === "" ? items : items.filter((i) => searchText(i).toLowerCase().includes(needle))),
    // `searchText` est redéfinie à chaque rendu du panneau appelant ; la
    // mémoïser sur elle rendrait le filtre inutile. Les lignes et le motif
    // suffisent à décider.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [items, needle],
  );

  const pages = Math.max(1, Math.ceil(matching.length / size));

  /*
    La page courante est bornée au RENDU, sans effet ni état à resynchroniser.

    Filtrer depuis la page 7 laisse un numéro de page qui n'existe plus. Le
    corriger dans un `useEffect` afficherait un tableau vide le temps d'un
    rendu, ce qui se voit — et se lit comme « aucun résultat ».
  */
  const current = Math.min(page, pages - 1);
  const visible = matching.slice(current * size, current * size + size);

  return (
    <div className="flex min-h-0 flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <FilterField
          value={filter}
          onChange={(v) => {
            setFilter(v);
            setPage(0);
          }}
          placeholder={filterHint ?? t("ed2k.table.filter")}
        />

        <span className="text-meta tabular-nums text-subtle">
          {needle === ""
            ? t("ed2k.table.count", { count: String(items.length) })
            : t("ed2k.table.matching", {
                count: String(matching.length),
                total: String(items.length),
              })}
        </span>

        <div className="ml-auto flex items-center gap-2">
          <PageSize
            value={size}
            onChange={(v) => {
              setSize(v);
              setPage(0);
            }}
          />
          <Pager page={current} pages={pages} onGo={setPage} />
        </div>
      </div>

      <Table columns={columns} minWidth={minWidth} label={label}>
        {visible.map((item, index) => renderRow(item, index))}
      </Table>

      {/*
        Un filtre qui ne rend rien doit le DIRE. Un tableau vide sous un champ
        rempli se lit comme un chargement qui n'aboutit pas.
      */}
      {matching.length === 0 && (
        <p className="px-3 py-6 text-center text-meta text-subtle">
          {t("ed2k.table.noMatch", { filter })}
        </p>
      )}
    </div>
  );
}

function FilterField({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  const t = useT();

  return (
    <div className="relative">
      <svg
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
        className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-subtle"
      >
        <circle cx="7" cy="7" r="4.5" stroke="currentColor" strokeWidth="1.4" />
        <path d="m10.5 10.5 3 3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
      </svg>

      <input
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        aria-label={t("ed2k.table.filter")}
        className={cx(
          "h-8 w-56 rounded-md border border-border-strong bg-surface pl-8 pr-2.5 text-meta text-fg",
          "placeholder:text-subtle",
          "transition-colors duration-(--motion-duration-fast)",
          "focus:border-accent focus:outline-none",
        )}
      />
    </div>
  );
}

function PageSize({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  const t = useT();

  return (
    <label className="flex items-center gap-1.5 text-meta text-subtle">
      <span className="sr-only sm:not-sr-only">{t("ed2k.table.perPage")}</span>
      <select
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        className={cx(
          "h-8 rounded-md border border-border-strong bg-surface px-1.5 text-meta text-fg",
          "focus:border-accent focus:outline-none",
        )}
      >
        {PAGE_SIZES.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

/**
 * Précédent / suivant, et le rang courant.
 *
 * Pas de liste de numéros de page : sur trois cents serveurs à vingt-cinq par
 * page, elle en compterait douze, et aucun de ces numéros ne veut rien dire —
 * personne ne sait ce qu'il y a « page 7 ». Le filtre est le vrai moyen
 * d'atteindre une ligne ; la pagination ne sert qu'à borner ce qui est rendu.
 */
function Pager({
  page,
  pages,
  onGo,
}: {
  page: number;
  pages: number;
  onGo: (page: number) => void;
}) {
  const t = useT();
  if (pages <= 1) return null;

  return (
    <div className="flex items-center gap-1">
      <PagerButton
        label={t("ed2k.table.previous")}
        disabled={page === 0}
        onClick={() => onGo(page - 1)}
        glyph="‹"
      />
      <span className="min-w-16 text-center text-meta tabular-nums text-muted">
        {t("ed2k.table.page", { page: String(page + 1), pages: String(pages) })}
      </span>
      <PagerButton
        label={t("ed2k.table.next")}
        disabled={page >= pages - 1}
        onClick={() => onGo(page + 1)}
        glyph="›"
      />
    </div>
  );
}

function PagerButton({
  label,
  glyph,
  disabled,
  onClick,
}: {
  label: string;
  glyph: string;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      title={label}
      className={cx(
        "pressable grid size-8 place-items-center rounded-md border border-border text-ui",
        "text-muted hover:bg-surface-hover hover:text-fg",
        "disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent",
      )}
    >
      <span aria-hidden="true">{glyph}</span>
    </button>
  );
}
