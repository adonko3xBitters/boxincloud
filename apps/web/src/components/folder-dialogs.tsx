"use client";

import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { buttonClass, cx } from "./ui";
import { ApiError } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";

/**
 * Création, renommage et suppression de dossiers.
 *
 * Les dossiers existent désormais en base : on peut en créer un vide pour y
 * ranger ensuite, et le renommer sans toucher au stockage à la main.
 *
 * Renommer un dossier renomme chacun des objets qu'il contient. L'interface le
 * dit — sur un backend distant, l'opération dure, et quelqu'un qui ne s'y attend
 * pas croira à un blocage.
 */

export type FolderDialog =
  | { kind: "create"; libraryId: string; parent: string }
  | { kind: "rename"; libraryId: string; path: string; name: string }
  | { kind: "delete"; libraryId: string; path: string; comicCount: number };

export function FolderDialogs({
  dialog,
  onClose,
}: {
  dialog: FolderDialog | null;
  onClose: () => void;
}) {
  if (!dialog) return null;

  if (dialog.kind === "create") {
    return <CreateFolder libraryId={dialog.libraryId} parent={dialog.parent} onClose={onClose} />;
  }
  if (dialog.kind === "rename") {
    return (
      <RenameFolder
        libraryId={dialog.libraryId}
        path={dialog.path}
        name={dialog.name}
        onClose={onClose}
      />
    );
  }
  return (
    <DeleteFolder
      libraryId={dialog.libraryId}
      path={dialog.path}
      comicCount={dialog.comicCount}
      onClose={onClose}
    />
  );
}

// ─── Création ────────────────────────────────────────────────────────────────

function CreateFolder({
  libraryId,
  parent,
  onClose,
}: {
  libraryId: string;
  parent: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  async function run(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.createFolder(libraryId, parent ? `${parent}/${name}` : name);
      await refreshFolders(queryClient);
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Nouveau dossier" onClose={onClose}>
      <form onSubmit={run} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">Nom</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            autoFocus
            placeholder="Tintin"
            className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg"
          />
          <span className="text-meta text-subtle">
            {parent ? (
              <>
                Sera créé dans <code className="text-muted">{parent}</code>.
              </>
            ) : (
              "Sera créé à la racine de la bibliothèque."
            )}
          </span>
        </label>

        <p className="text-meta leading-relaxed text-subtle">
          Rien n&apos;est écrit dans votre stockage : un magasin d&apos;objets
          n&apos;a pas de répertoires. Le dossier prendra corps au premier album
          déposé.
        </p>

        {error && <ErrorNote>{error}</ErrorNote>}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className={buttonClass("secondary", "sm")}>
            Annuler
          </button>
          <button type="submit" disabled={busy} className={buttonClass("primary", "sm")}>
            {busy ? "Création…" : "Créer"}
          </button>
        </div>
      </form>
    </Shell>
  );
}

// ─── Renommage ───────────────────────────────────────────────────────────────

function RenameFolder({
  libraryId,
  path,
  name,
  onClose,
}: {
  libraryId: string;
  path: string;
  name: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [next, setNext] = useState(name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  const parent = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";

  async function run(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.relocateFolder(libraryId, path, parent ? `${parent}/${next}` : next);
      await refreshFolders(queryClient);
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Renommer le dossier" onClose={onClose}>
      <form onSubmit={run} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">Nom</span>
          <input
            value={next}
            onChange={(e) => setNext(e.target.value)}
            required
            autoFocus
            className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg"
          />
        </label>

        <p className="text-meta leading-relaxed text-subtle">
          Chaque album du dossier est renommé dans votre stockage. Sur un backend
          distant, une branche volumineuse peut demander plusieurs minutes.
        </p>

        {error && <ErrorNote>{error}</ErrorNote>}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className={buttonClass("secondary", "sm")}>
            Annuler
          </button>
          <button type="submit" disabled={busy} className={buttonClass("primary", "sm")}>
            {busy ? "Renommage…" : "Renommer"}
          </button>
        </div>
      </form>
    </Shell>
  );
}

// ─── Suppression ─────────────────────────────────────────────────────────────

function DeleteFolder({
  libraryId,
  path,
  comicCount,
  onClose,
}: {
  libraryId: string;
  path: string;
  comicCount: number;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [confirmed, setConfirmed] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  const armed = !deleteFiles || confirmed.trim().toLowerCase() === "supprimer";

  async function run() {
    setBusy(true);
    setError(null);
    try {
      await api.deleteFolder(libraryId, path, {
        deleteComics: comicCount > 0,
        deleteFiles,
      });
      await refreshFolders(queryClient);
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Supprimer le dossier" onClose={onClose}>
      <div className="flex flex-col gap-3">
        <p className="text-ui text-fg">
          <code className="text-muted">{path}</code>
        </p>

        {comicCount === 0 ? (
          <p className="text-meta leading-relaxed text-muted">
            Ce dossier est vide. Sa suppression ne touche à rien d&apos;autre.
          </p>
        ) : (
          <>
            <p className="text-meta leading-relaxed text-muted">
              Ce dossier et ses sous-dossiers contiennent{" "}
              <strong className="text-fg">{comicCount} album{comicCount > 1 ? "s" : ""}</strong>.
            </p>

            <label className="flex items-start gap-2.5 rounded-md border border-border p-3">
              <input
                type="checkbox"
                checked={deleteFiles}
                onChange={(e) => {
                  setDeleteFiles(e.target.checked);
                  setConfirmed("");
                }}
                className="mt-0.5 size-4 accent-[var(--danger)]"
              />
              <span>
                <span className="block text-ui font-medium text-fg">
                  Supprimer aussi les fichiers
                </span>
                <span className="block text-meta leading-relaxed text-muted">
                  Sans cette option, les albums sont retirés du catalogue et les
                  fichiers restent dans votre stockage. Avec, ils sont effacés —
                  irréversible.
                </span>
              </span>
            </label>

            {deleteFiles && (
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
          </>
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
              deleteFiles ? "bg-danger hover:opacity-90" : "bg-accent hover:bg-accent-hover",
            )}
          >
            {busy ? "Suppression…" : "Supprimer"}
          </button>
        </div>
      </div>
    </Shell>
  );
}

// ─── Éléments ────────────────────────────────────────────────────────────────

function Shell({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-[65] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="rise-in flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-4 shadow-2xl"
      >
        <div className="flex items-center justify-between">
          <h2 className="text-title font-semibold text-fg">{title}</h2>
          <button
            onClick={onClose}
            aria-label="Fermer"
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

async function refreshFolders(queryClient: ReturnType<typeof useQueryClient>) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["folders"] }),
    queryClient.invalidateQueries({ queryKey: ["comics"] }),
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
