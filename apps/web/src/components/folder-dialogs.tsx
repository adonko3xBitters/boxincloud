"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

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
  | { kind: "delete"; libraryId: string; path: string; comicCount: number }
  | { kind: "lock"; libraryId: string; path: string; readOnly: boolean; hasCode: boolean }
  | { kind: "unlock"; libraryId: string; path: string }
  | { kind: "share"; libraryId: string; path: string; hasCode: boolean };

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
  if (dialog.kind === "delete") {
    return (
      <DeleteFolder
        libraryId={dialog.libraryId}
        path={dialog.path}
        comicCount={dialog.comicCount}
        onClose={onClose}
      />
    );
  }
  if (dialog.kind === "lock") {
    return (
      <LockFolder
        libraryId={dialog.libraryId}
        path={dialog.path}
        readOnly={dialog.readOnly}
        hasCode={dialog.hasCode}
        onClose={onClose}
      />
    );
  }
  if (dialog.kind === "unlock") {
    return <UnlockFolder libraryId={dialog.libraryId} path={dialog.path} onClose={onClose} />;
  }
  return (
    <ShareFolder
      libraryId={dialog.libraryId}
      path={dialog.path}
      hasCode={dialog.hasCode}
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

// ─── Verrouillage ────────────────────────────────────────────────────────────

/**
 * Réglage des deux verrous.
 *
 * Ils sont présentés côte à côte parce qu'ils se confondent facilement : l'un
 * protège sans cacher, l'autre cache sans protéger. Les décrire l'un à côté de
 * l'autre est la seule façon de rendre la différence évidente.
 */
export function LockFolder({
  libraryId,
  path,
  readOnly,
  hasCode,
  onClose,
}: {
  libraryId: string;
  path: string;
  readOnly: boolean;
  hasCode: boolean;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [protectedMode, setProtected] = useState(readOnly);
  const [code, setCode] = useState("");
  const [removeCode, setRemoveCode] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  async function run(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const lock: { readOnly?: boolean; code?: string } = {};
      if (protectedMode !== readOnly) lock.readOnly = protectedMode;
      if (removeCode) lock.code = "";
      else if (code) lock.code = code;

      if (Object.keys(lock).length > 0) {
        await api.setFolderLock(libraryId, path, lock);
      }
      await refreshFolders(queryClient);
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Verrouiller le dossier" onClose={onClose}>
      <form onSubmit={run} className="flex flex-col gap-4">
        <p className="text-meta text-muted">
          <code className="text-fg">{path}</code>
        </p>

        <label className="flex items-start gap-2.5 rounded-md border border-border p-3">
          <input
            type="checkbox"
            checked={protectedMode}
            onChange={(e) => setProtected(e.target.checked)}
            className="mt-0.5 size-4 accent-[var(--accent)]"
          />
          <span>
            <span className="block text-ui font-medium text-fg">Lecture seule</span>
            <span className="block text-meta leading-relaxed text-muted">
              Le dossier reste visible de tous, mais ne peut plus être renommé,
              déplacé, ni recevoir ou perdre un album. La protection s&apos;étend
              aux sous-dossiers.
            </span>
          </span>
        </label>

        <div className="rounded-md border border-border p-3">
          <p className="text-ui font-medium text-fg">Code d&apos;accès</p>
          <p className="mt-0.5 text-meta leading-relaxed text-muted">
            Masque le dossier et son contenu — listes, recherche, accès direct —
            tant que le code n&apos;a pas été saisi. Utile sur un serveur partagé.
          </p>

          {hasCode && (
            <label className="mt-2.5 flex items-center gap-2">
              <input
                type="checkbox"
                checked={removeCode}
                onChange={(e) => {
                  setRemoveCode(e.target.checked);
                  if (e.target.checked) setCode("");
                }}
                className="size-4 accent-[var(--accent)]"
              />
              <span className="text-meta text-muted">Retirer le code existant</span>
            </label>
          )}

          {!removeCode && (
            <label className="mt-2.5 flex flex-col gap-1">
              <span className="text-micro uppercase tracking-wide text-subtle">
                {hasCode ? "Nouveau code" : "Code"}
              </span>
              <input
                type="password"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder={hasCode ? "Laisser vide pour ne pas changer" : "Quatre caractères minimum"}
                autoComplete="new-password"
                className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg placeholder:text-subtle"
              />
            </label>
          )}

          {hasCode && (
            <p className="mt-2 text-meta leading-relaxed text-subtle">
              Changer ou retirer le code referme le dossier partout : un accès
              obtenu avec l&apos;ancien ne survit pas au nouveau.
            </p>
          )}
        </div>

        {error && <ErrorNote>{error}</ErrorNote>}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className={buttonClass("secondary", "sm")}>
            Annuler
          </button>
          <button type="submit" disabled={busy} className={buttonClass("primary", "sm")}>
            {busy ? "Enregistrement…" : "Enregistrer"}
          </button>
        </div>
      </form>
    </Shell>
  );
}

/** Saisie du code pour ouvrir un dossier masqué. */
export function UnlockFolder({
  libraryId,
  path,
  onClose,
}: {
  libraryId: string;
  path: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEscape(onClose);

  async function run(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.unlockFolder(libraryId, path, code);
      await refreshFolders(queryClient);
      onClose();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Shell title="Dossier verrouillé" onClose={onClose}>
      <form onSubmit={run} className="flex flex-col gap-3">
        <p className="text-meta text-muted">
          <code className="text-fg">{path}</code>
        </p>

        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">Code d&apos;accès</span>
          <input
            type="password"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            autoFocus
            autoComplete="off"
            className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg"
          />
          <span className="text-meta text-subtle">
            Le dossier reste ouvert deux heures, puis se referme de lui-même.
          </span>
        </label>

        {error && <ErrorNote>{error}</ErrorNote>}

        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className={buttonClass("secondary", "sm")}>
            Annuler
          </button>
          <button type="submit" disabled={busy} className={buttonClass("primary", "sm")}>
            {busy ? "Vérification…" : "Ouvrir"}
          </button>
        </div>
      </form>
    </Shell>
  );
}

// ─── Partage ─────────────────────────────────────────────────────────────────

/**
 * Partage d'un dossier.
 *
 * Deux moitiés aux enjeux très différents, séparées visuellement pour qu'on ne
 * les confonde pas : ouvrir à un compte du serveur, ou publier un lien que
 * n'importe qui pourra suivre.
 */
export function ShareFolder({
  libraryId,
  path,
  hasCode,
  onClose,
}: {
  libraryId: string;
  path: string;
  hasCode: boolean;
  onClose: () => void;
}) {
  useEscape(onClose);

  return (
    <Shell title="Partager le dossier" onClose={onClose}>
      <p className="text-meta text-muted">
        <code className="text-fg">{path}</code>
      </p>

      <AccountSharing libraryId={libraryId} path={path} />
      <PublicLink libraryId={libraryId} path={path} hasCode={hasCode} />

      <div className="flex justify-end">
        <button onClick={onClose} className={buttonClass("secondary", "sm")}>
          Fermer
        </button>
      </div>
    </Shell>
  );
}

/** Partage entre comptes du serveur. */
function AccountSharing({ libraryId, path }: { libraryId: string; path: string }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const accounts = useQuery({ queryKey: ["accounts"], queryFn: api.listAccounts });
  const grants = useQuery({
    queryKey: ["folder-access", libraryId, path],
    queryFn: () => api.listFolderGrants(libraryId, path),
  });

  const granted = useMemo(() => {
    const map = new Map<string, boolean>();
    for (const grant of grants.data?.grants ?? []) map.set(grant.userId, grant.canWrite);
    return map;
  }, [grants.data]);

  async function toggle(userId: string, on: boolean, canWrite: boolean) {
    setError(null);
    try {
      if (on) await api.grantFolderAccess(libraryId, path, userId, canWrite);
      else await api.revokeFolderAccess(libraryId, path, userId);
      await queryClient.invalidateQueries({ queryKey: ["folder-access", libraryId, path] });
      await queryClient.invalidateQueries({ queryKey: ["folders"] });
    } catch (err) {
      setError(describe(err));
    }
  }

  const list = accounts.data?.accounts ?? [];

  return (
    <section className="rounded-md border border-border p-3">
      <h3 className="text-ui font-medium text-fg">Comptes du serveur</h3>
      <p className="mt-0.5 text-meta leading-relaxed text-muted">
        Un dossier sans accès explicite est visible de tous ceux qui voient la
        bibliothèque. En accorder un ici le referme pour tous les autres.
      </p>

      <ul className="mt-2.5 flex flex-col gap-1">
        {list.map((account) => {
          const on = granted.has(account.id);
          const canWrite = granted.get(account.id) ?? false;

          return (
            <li key={account.id} className="flex items-center gap-2 rounded-md border border-border px-2.5 py-1.5">
              <input
                type="checkbox"
                checked={on}
                onChange={(e) => void toggle(account.id, e.target.checked, canWrite)}
                aria-label={`Partager avec ${account.username}`}
                className="size-4 accent-[var(--accent)]"
              />
              <span className="min-w-0 flex-1 truncate text-ui text-fg">
                {account.displayName || account.username}
              </span>
              <label className={cx("flex items-center gap-1.5 text-meta", on ? "text-muted" : "text-subtle opacity-50")}>
                <input
                  type="checkbox"
                  checked={canWrite}
                  disabled={!on}
                  onChange={(e) => void toggle(account.id, true, e.target.checked)}
                  className="size-3.5 accent-[var(--accent)]"
                />
                écriture
              </label>
            </li>
          );
        })}
        {list.length === 0 && <li className="text-meta text-subtle">Aucun autre compte.</li>}
      </ul>

      {error && <div className="mt-2"><ErrorNote>{error}</ErrorNote></div>}
    </section>
  );
}

/**
 * Lien public.
 *
 * L'avertissement n'est pas décoratif : c'est le seul mécanisme de boxincloud
 * qui donne accès sans compte, et quelqu'un qui crée un lien doit comprendre ce
 * qu'il ouvre au moment où il l'ouvre.
 */
function PublicLink({
  libraryId,
  path,
  hasCode,
}: {
  libraryId: string;
  path: string;
  hasCode: boolean;
}) {
  const queryClient = useQueryClient();
  const [days, setDays] = useState(7);
  const [label, setLabel] = useState("");
  const [created, setCreated] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const links = useQuery({ queryKey: ["share-links"], queryFn: api.listShareLinks });
  const mine = (links.data?.links ?? []).filter((l) => l.folderPath === path);

  async function create() {
    setBusy(true);
    setError(null);
    try {
      const expiresAt = new Date(Date.now() + days * 86_400_000).toISOString();
      const link = await api.createShareLink({ libraryId, folderPath: path, label, expiresAt });
      setCreated(`${window.location.origin}/partage?t=${link.token}`);
      await queryClient.invalidateQueries({ queryKey: ["share-links"] });
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    await api.revokeShareLink(id);
    setCreated(null);
    await queryClient.invalidateQueries({ queryKey: ["share-links"] });
  }

  if (hasCode) {
    return (
      <section className="rounded-md border border-border p-3">
        <h3 className="text-ui font-medium text-fg">Lien public</h3>
        <p className="mt-0.5 text-meta leading-relaxed text-muted">
          Indisponible : ce dossier est masqué par un code d&apos;accès. Publier
          ce qu&apos;on vient de cacher annulerait le code sans le dire.
        </p>
      </section>
    );
  }

  return (
    <section className="rounded-md border border-border p-3">
      <h3 className="text-ui font-medium text-fg">Lien public</h3>
      <p className="mt-0.5 text-meta leading-relaxed text-warning">
        Un lien public ouvre ce dossier <strong>sans aucun compte</strong> : qui
        a l&apos;adresse voit le contenu, et peut la transmettre.
      </p>

      {mine.length > 0 && (
        <ul className="mt-2.5 flex flex-col gap-1">
          {mine.map((link) => (
            <li key={link.id} className="flex items-center gap-2 rounded-md border border-border px-2.5 py-1.5">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-ui text-fg">{link.label || "Sans nom"}</span>
                <span className="block text-meta text-subtle">
                  expire le {new Date(link.expiresAt).toLocaleDateString("fr-FR")} ·{" "}
                  {link.useCount} ouverture{link.useCount > 1 ? "s" : ""}
                </span>
              </span>
              <button
                onClick={() => void revoke(link.id)}
                className="pressable rounded px-2 py-1 text-meta text-danger hover:bg-danger/10"
              >
                Révoquer
              </button>
            </li>
          ))}
        </ul>
      )}

      {created && (
        <div className="mt-2.5 rounded-md border border-success/40 bg-success/10 p-2.5">
          <p className="text-meta text-muted">
            Copiez ce lien maintenant : il ne sera plus affiché.
          </p>
          <input
            readOnly
            value={created}
            onFocus={(e) => e.currentTarget.select()}
            className="mt-1 w-full rounded border border-border bg-surface px-2 py-1 font-mono text-meta text-fg"
          />
        </div>
      )}

      <div className="mt-2.5 flex flex-wrap items-end gap-2">
        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">Nom</span>
          <input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="Pour Camille"
            className="h-8 w-40 rounded-md border border-border bg-surface px-2 text-meta text-fg placeholder:text-subtle"
          />
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">Expire dans</span>
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="h-8 rounded-md border border-border bg-surface px-2 text-meta text-fg"
          >
            <option value={1}>1 jour</option>
            <option value={7}>7 jours</option>
            <option value={30}>30 jours</option>
            <option value={365}>1 an</option>
          </select>
        </label>

        <button onClick={() => void create()} disabled={busy} className={buttonClass("primary", "sm")}>
          {busy ? "Création…" : "Créer un lien"}
        </button>
      </div>

      {error && <div className="mt-2"><ErrorNote>{error}</ErrorNote></div>}
    </section>
  );
}
