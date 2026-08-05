"use client";

/**
 * Les gestes qui agissent sur le démon.
 *
 * # Ce qu'un bouton doit faire ici, et pourquoi c'est particulier
 *
 * amuled n'accuse que réception : il dit « reçu », jamais « fait ». Une
 * commande ne peut donc pas rendre son résultat, et un bouton qui repasserait
 * à l'état « prêt » dès la réponse mentirait — l'effet n'est pas encore
 * visible.
 *
 * D'où la règle tenue par ce fichier : un bouton reste occupé jusqu'à ce que
 * l'INSTANTANÉ ait été redemandé. C'est un peu plus long, et c'est honnête.
 *
 * Le serveur y aide : une commande réveille la scrutation, si bien que le
 * prochain instantané arrive dans la seconde plutôt qu'au bout de cinq.
 */

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { Button, cx } from "@/components/ui";
import { useT } from "@/i18n";

import { useEd2kError } from "./errors";

/**
 * useCommand exécute une commande et rafraîchit ce qu'elle a changé.
 *
 * L'erreur est GARDÉE plutôt que jetée : un refus du démon — « Kad is disabled
 * in preferences » — est exactement ce qu'il faut montrer, et le perdre
 * laisserait un bouton qui ne fait rien sans dire pourquoi.
 *
 * Elle est aussi TRADUITE. Afficher `err.message` revenait à montrer le `detail`
 * du serveur, qui est en anglais : une interface française annonçait « no aMule
 * daemon has been declared » sans dire où aller le déclarer.
 */
export function useCommand() {
  const queryClient = useQueryClient();
  const describe = useEd2kError();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function run(command: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await command();
      // Tout ce qui vient du démon peut avoir changé : une pause modifie la
      // file, une connexion modifie l'état et la liste des serveurs.
      await queryClient.invalidateQueries({ queryKey: ["ed2k"] });
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return { run, busy, error, clearError: () => setError(null) };
}

/**
 * Bandeau d'erreur d'une commande.
 *
 * Placé au-dessus du tableau plutôt qu'à côté du bouton : un refus concerne le
 * panneau entier, et le coller au bouton obligerait à réserver de la place sur
 * chaque ligne pour un message qui n'apparaît presque jamais.
 */
export function CommandError({
  error,
  onDismiss,
}: {
  error: string | null;
  onDismiss: () => void;
}) {
  const t = useT();
  if (!error) return null;

  return (
    <div
      role="alert"
      className="mb-2 flex items-start gap-2 rounded-md border border-danger/40 bg-danger/10 px-3 py-2"
    >
      <p className="min-w-0 flex-1 text-meta text-danger">{error}</p>
      <button
        onClick={onDismiss}
        aria-label={t("action.close")}
        className="pressable shrink-0 text-meta text-danger hover:underline"
      >
        {t("action.close")}
      </button>
    </div>
  );
}

/**
 * Bouton d'action compact, pour une cellule de tableau.
 *
 * Le libellé reste TEXTUEL et non iconographique. Une rangée d'icônes sur une
 * ligne dense est indéchiffrable sans les survoler une par une, et « pause »
 * tient dans la même place qu'un pictogramme ambigu.
 */
export function ActionButton({
  label,
  onClick,
  disabled,
  tone = "normal",
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  tone?: "normal" | "danger";
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cx(
        "pressable rounded px-1.5 py-0.5 text-meta",
        "disabled:cursor-not-allowed disabled:opacity-40",
        tone === "danger"
          ? "text-danger hover:bg-danger/10"
          : "text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      {label}
    </button>
  );
}

/**
 * Confirmation en deux temps, sans boîte de dialogue.
 *
 * La suppression d'un téléchargement efface ce qui a été reçu et ne se défait
 * pas. Une modale par ligne serait lourde ; un second clic sur un bouton qui a
 * changé de libellé demande le même engagement et reste sur place.
 *
 * L'état retombe seul au bout de quelques secondes : un « confirmer » laissé
 * armé sur une ligne qu'on a quittée du regard est un piège.
 */
export function ConfirmButton({
  label,
  confirmLabel,
  onConfirm,
  disabled,
}: {
  label: string;
  confirmLabel: string;
  onConfirm: () => void;
  disabled?: boolean;
}) {
  const [armed, setArmed] = useState(false);

  function arm() {
    setArmed(true);
    window.setTimeout(() => setArmed(false), 4000);
  }

  return (
    <ActionButton
      label={armed ? confirmLabel : label}
      tone="danger"
      disabled={disabled}
      onClick={() => {
        if (!armed) {
          arm();
          return;
        }
        setArmed(false);
        onConfirm();
      }}
    />
  );
}

/** Bouton d'en-tête de panneau, pour les gestes qui ne visent pas une ligne. */
export function PanelAction({
  label,
  onClick,
  busy,
  variant = "secondary",
}: {
  label: string;
  onClick: () => void;
  busy?: boolean;
  variant?: "primary" | "secondary" | "ghost";
}) {
  return (
    <Button variant={variant} size="sm" loading={busy} onClick={onClick}>
      {label}
    </Button>
  );
}
