import { cx } from "./ui";

/**
 * Marque du projet.
 *
 * Une boîte ouverte dont s'échappe une planche : le nom dit « box in cloud »,
 * le logo dit qu'il s'agit de bandes dessinées. Dessiné en SVG plutôt qu'en
 * image — il doit rester net à 20 px comme à 200, et suivre la couleur du texte
 * qui l'entoure.
 */
export function Logo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      className={cx("size-8", className)}
      role="img"
      aria-label="boxincloud"
    >
      {/* Planche qui s'échappe, en arrière-plan */}
      <rect
        x="11" y="2" width="13" height="17" rx="1.5"
        className="fill-accent/25"
        transform="rotate(8 17.5 10.5)"
      />
      <rect
        x="9" y="4" width="13" height="17" rx="1.5"
        className="fill-accent/60"
      />
      {/* Boîte, au premier plan */}
      <path
        d="M4 13.5 16 9l12 4.5v11L16 29 4 24.5v-11Z"
        className="fill-accent"
      />
      <path
        d="M4 13.5 16 18l12-4.5M16 18v11"
        className="stroke-background"
        strokeWidth="1.4"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function Wordmark({ className }: { className?: string }) {
  return (
    <span className={cx("text-lg font-semibold tracking-tight text-fg", className)}>
      boxin<span className="text-accent">cloud</span>
    </span>
  );
}

export function BrandLockup({ className }: { className?: string }) {
  return (
    <div className={cx("flex items-center gap-2.5", className)}>
      <Logo />
      <Wordmark />
    </div>
  );
}
