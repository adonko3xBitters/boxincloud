"use client";

/**
 * Composants d'interface de base.
 *
 * Écrits à la main plutôt qu'importés d'une bibliothèque : le besoin tient en
 * quelques dizaines de lignes, et chaque composant peut ainsi s'appuyer
 * directement sur les rôles de couleur du projet plutôt que sur une palette
 * étrangère qu'il faudrait faire correspondre.
 */

import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";
import { useT } from "@/i18n";
import { forwardRef } from "react";

export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// ─── Bouton ──────────────────────────────────────────────────────────────────

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  loading?: boolean;
};

const buttonVariants: Record<NonNullable<ButtonProps["variant"]>, string> = {
  primary: "bg-accent text-inverted hover:bg-accent-hover",
  secondary: "bg-surface-raised text-fg border border-border hover:bg-surface-hover",
  ghost: "text-muted hover:text-fg hover:bg-surface-hover",
  danger: "bg-danger text-white hover:opacity-90",
};

const buttonSizes: Record<NonNullable<ButtonProps["size"]>, string> = {
  sm: "h-8 px-3 text-sm gap-1.5",
  md: "h-10 px-4 text-sm gap-2",
  lg: "h-12 px-6 text-base gap-2",
};

/**
 * Classes d'un bouton, applicables à un élément qui n'en est pas un.
 *
 * Un lien de navigation doit rester un `<a>` — pour l'ouverture en nouvel
 * onglet, la copie d'adresse, l'annonce par un lecteur d'écran — tout en
 * ressemblant à un bouton. Partager les classes évite de dupliquer le style,
 * sans introduire le mécanisme `asChild` d'une bibliothèque de composants.
 */
export function buttonClass(
  variant: NonNullable<ButtonProps["variant"]> = "primary",
  size: NonNullable<ButtonProps["size"]> = "md",
): string {
  return cx(
    "inline-flex items-center justify-center rounded-md font-medium",
    "transition-colors duration-(--motion-duration-fast)",
    buttonVariants[variant],
    buttonSizes[size],
  );
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "primary", size = "md", loading = false, disabled, className, children, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cx(
        "inline-flex items-center justify-center rounded-md font-medium",
        "transition-colors duration-(--motion-duration-fast)",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        buttonVariants[variant],
        buttonSizes[size],
        className,
      )}
      {...props}
    >
      {loading && <Spinner className="size-4" />}
      {children}
    </button>
  );
});

// ─── Champ de saisie ─────────────────────────────────────────────────────────

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
  error?: string;
  hint?: string;
};

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { label, error, hint, className, id, ...props },
  ref,
) {
  const inputId = id ?? props.name;
  const describedBy = error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined;

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={inputId} className="text-sm font-medium text-fg">
          {label}
        </label>
      )}
      <input
        ref={ref}
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cx(
          "h-10 rounded-md border bg-surface px-3 text-sm text-fg",
          "placeholder:text-subtle",
          "transition-colors duration-(--motion-duration-fast)",
          // `border-strong` et non `border` : un champ dont on ne voit pas le
          // contour est un champ qu'on ne trouve pas. C'est la seule bordure du
          // système qui porte de l'information, et la seule tenue à 3:1.
          error ? "border-danger" : "border-border-strong focus:border-accent",
          className,
        )}
        {...props}
      />
      {error ? (
        <p id={`${inputId}-error`} className="text-xs text-danger">
          {error}
        </p>
      ) : hint ? (
        <p id={`${inputId}-hint`} className="text-xs text-subtle">
          {hint}
        </p>
      ) : null}
    </div>
  );
});

// ─── Indicateurs ─────────────────────────────────────────────────────────────

export function Spinner({ className }: { className?: string }) {
  return (
    <svg
      className={cx("animate-spin", className)}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" />
      <path
        className="opacity-90"
        fill="currentColor"
        d="M12 2a10 10 0 0 1 10 10h-3a7 7 0 0 0-7-7V2Z"
      />
    </svg>
  );
}

/**
 * État vide.
 *
 * Un écran vide sans explication laisse l'utilisateur se demander si
 * l'application est cassée. Chaque état vide dit ce qui manque et, quand c'est
 * possible, propose l'action qui le remplit.
 */
export function EmptyState({
  title,
  description,
  action,
  icon,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
      {icon && <div className="text-subtle">{icon}</div>}
      <h2 className="text-lg font-semibold text-fg">{title}</h2>
      {description && <p className="max-w-md text-sm text-muted">{description}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const t = useT();
  const message = error instanceof Error ? error.message : t("error.generic");

  return (
    <EmptyState
      title={t("error.pageFailed")}
      description={message}
      action={
        onRetry && (
          <Button variant="secondary" onClick={onRetry}>
            Réessayer
          </Button>
        )
      }
    />
  );
}

/** Réserve la place exacte d'un élément en cours de chargement. */
export function Skeleton({ className }: { className?: string }) {
  return <div className={cx("skeleton rounded-md", className)} aria-hidden="true" />;
}

// ─── Badge ───────────────────────────────────────────────────────────────────

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "accent" | "success" | "warning" | "danger";
}) {
  const tones = {
    neutral: "bg-surface-hover text-muted",
    accent: "bg-accent-subtle text-accent-text",
    success: "bg-success/15 text-success",
    warning: "bg-warning/15 text-warning",
    danger: "bg-danger/15 text-danger",
  };

  return (
    <span
      className={cx(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        tones[tone],
      )}
    >
      {children}
    </span>
  );
}
