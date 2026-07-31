"use client";

import { useT } from "@/i18n";
import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { buttonClass, cx } from "./ui";
import { ApiError } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useWorkspace } from "@/lib/workspace";

/**
 * Suppression et rangement.
 *
 * Deux gestes irréversibles à des degrés différents, et l'interface doit dire
 * lequel est lequel. Retirer un album du catalogue se rattrape en relançant un
 * parcours ; effacer son fichier non. La boîte pose donc la question au lieu de
 * choisir, et n'active la seconde option que sur un geste explicite.
 */

// ─── Suppression ─────────────────────────────────────────────────────────────

export function DeleteDialog({
  ids,
  titles,
  onClose,
}: {
  ids: string[];
  titles: string[];
  onClose: () => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const { clearSelection } = useWorkspace();
  const [deleteFile, setDeleteFile] = useState(false);
  const [confirmed, setConfirmed] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  // Effacer des fichiers demande de taper le mot : une case à cocher se coche
  // par réflexe, un mot ne s'écrit pas par accident.
  const needsTyping = deleteFile;
  const armed = !needsTyping || confirmed.trim().toLowerCase() === "supprimer";

  async function run() {
    setBusy(true);
    setError(null);
    try {
      if (ids.length === 1) {
        await api.deleteComic(ids[0]!, deleteFile);
      } else {
        await api.manageComics("delete", ids, { deleteFile });
      }
      await refreshCatalog(queryClient);
      clearSelection();
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      title={
        ids.length > 1
          ? t("manage.removeMany", { count: ids.length })
          : t("manage.removeOne")
      }
      onClose={onClose}
    >
      <TargetList titles={titles} count={ids.length} />

      <div className="flex flex-col gap-2">
        <Choice
          selected={!deleteFile}
          onSelect={() => setDeleteFile(false)}
          title={t("manage.fromCatalog")}
          description={t("manage.fromCatalogHint")}
        />
        <Choice
          selected={deleteFile}
          onSelect={() => setDeleteFile(true)}
          danger
          title={t("manage.deleteFile")}
          description={t("manage.deleteFileHint")}
        />
      </div>

      {needsTyping && (
        <label className="flex flex-col gap-1">
          <span className="text-meta text-muted">
            Tapez <strong className="text-danger">supprimer</strong> pour confirmer.
          </span>
          <input
            value={confirmed}
            onChange={(e) => setConfirmed(e.target.value)}
            autoFocus
            className="h-9 rounded-md border border-danger/50 bg-surface px-2.5 text-ui text-fg"
          />
        </label>
      )}

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button onClick={onClose} className={buttonClass("secondary", "sm")}>
          Annuler
        </button>
        <button
          onClick={() => void run()}
          disabled={!armed || busy}
          className={cx(
            "pressable rounded-md px-3 py-1.5 text-ui font-medium text-white disabled:opacity-40",
            deleteFile ? "bg-danger hover:opacity-90" : "bg-accent hover:bg-accent-hover",
          )}
        >
          {busy
            ? t("manage.working")
            : deleteFile
              ? t("storage.deleteForever")
              : t("manage.remove")}
        </button>
      </div>
    </Dialog>
  );
}

// ─── Rangement ───────────────────────────────────────────────────────────────

export function MoveDialog({
  ids,
  titles,
  onClose,
}: {
  ids: string[];
  titles: string[];
  onClose: () => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const { clearSelection } = useWorkspace();
  const [folder, setFolder] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  const folders = useQuery({ queryKey: ["folders", undefined], queryFn: () => api.listFolders() });

  // Les dossiers existants sont proposés : taper un chemin de mémoire est le
  // meilleur moyen d'en créer un deuxième à une lettre près.
  const suggestions = useMemo(
    () => (folders.data?.folders ?? []).map((f) => f.path).filter((path) => path !== ""),
    [folders.data],
  );

  async function run() {
    setBusy(true);
    setError(null);
    try {
      if (ids.length === 1) {
        await api.moveComic(ids[0]!, folder);
      } else {
        await api.manageComics("move", ids, { folder });
      }
      await refreshCatalog(queryClient);
      clearSelection();
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      title={
        ids.length > 1 ? t("manage.moveMany", { count: ids.length }) : t("manage.moveOne")
      }
      onClose={onClose}
    >
      <TargetList titles={titles} count={ids.length} />

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">Dossier de destination</span>
        <input
          value={folder}
          onChange={(e) => setFolder(e.target.value)}
          list="dossiers-existants"
          placeholder={t("manage.rootPlaceholder")}
          autoFocus
          className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg placeholder:text-subtle"
        />
        <datalist id="dossiers-existants">
          {suggestions.map((path) => (
            <option key={path} value={path} />
          ))}
        </datalist>
        <span className="text-meta leading-relaxed text-subtle">
          Le fichier est déplacé dans votre stockage. Sur un backend S3, la copie
          se fait côté serveur : les octets ne transitent pas par boxincloud.
        </span>
      </label>

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button onClick={onClose} className={buttonClass("secondary", "sm")}>
          Annuler
        </button>
        <button onClick={() => void run()} disabled={busy} className={buttonClass("primary", "sm")}>
          {busy ? t("manage.moving") : t("manage.move")}
        </button>
      </div>
    </Dialog>
  );
}

// ─── Éléments partagés ───────────────────────────────────────────────────────

function Dialog({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const t = useT();
  return (
    <div className="fixed inset-0 z-[65] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="rise-in flex w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-4 shadow-2xl"
      >
        <div className="flex items-center justify-between">
          <h2 className="text-title font-semibold text-fg">{title}</h2>
          <button
            onClick={onClose}
            aria-label={t("action.close")}
            className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Rappelle sur quoi l'action va porter — trois titres suffisent à situer. */
function TargetList({ titles, count }: { titles: string[]; count: number }) {
  const shown = titles.slice(0, 3);
  const rest = count - shown.length;

  return (
    <ul className="rounded-md border border-border bg-surface-sunken px-3 py-2 text-meta text-muted">
      {shown.map((title, index) => (
        <li key={index} className="truncate">
          {title}
        </li>
      ))}
      {rest > 0 && <li className="text-subtle">…et {rest} autre{rest > 1 ? "s" : ""}</li>}
    </ul>
  );
}

function Choice({
  selected,
  onSelect,
  title,
  description,
  danger,
}: {
  selected: boolean;
  onSelect: () => void;
  title: string;
  description: string;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onSelect}
      aria-pressed={selected}
      className={cx(
        "flex gap-2.5 rounded-md border p-3 text-left transition-colors duration-(--motion-duration-fast)",
        selected
          ? danger
            ? "border-danger bg-danger/10"
            : "border-accent bg-accent/10"
          : "border-border hover:bg-surface-hover",
      )}
    >
      <span
        className={cx(
          "mt-0.5 grid size-4 shrink-0 place-items-center rounded-full border-2",
          selected ? (danger ? "border-danger" : "border-accent") : "border-border-strong",
        )}
      >
        {selected && (
          <span className={cx("size-2 rounded-full", danger ? "bg-danger" : "bg-accent")} />
        )}
      </span>
      <span>
        <span className={cx("block text-ui font-medium", danger && selected ? "text-danger" : "text-fg")}>
          {title}
        </span>
        <span className="block text-meta leading-relaxed text-muted">{description}</span>
      </span>
    </button>
  );
}

function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-meta leading-relaxed text-danger">
      {children}
    </p>
  );
}

function useEscape(onClose: () => void) {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose]);
}

async function refreshCatalog(queryClient: ReturnType<typeof useQueryClient>) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["comics"] }),
    queryClient.invalidateQueries({ queryKey: ["folders"] }),
    queryClient.invalidateQueries({ queryKey: ["series"] }),
    queryClient.invalidateQueries({ queryKey: ["libraries"] }),
    queryClient.invalidateQueries({ queryKey: ["comic"] }),
  ]);
}

function describe(error: unknown): string {
  if (error instanceof ApiError) {
    const fields = error.problem?.errors;
    if (fields) {
      const first = Object.values(fields)[0];
      if (first) return first;
    }
    return error.problem?.detail ?? error.problem?.title ?? error.message;
  }
  return error instanceof Error ? error.message : "erreur inconnue";
}
