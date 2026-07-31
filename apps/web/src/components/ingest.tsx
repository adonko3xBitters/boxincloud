"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { buttonClass, cx } from "./ui";
import { ApiError } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useT } from "@/i18n";
import { useCurrentUser } from "@/lib/auth";

/**
 * Ajout de contenu.
 *
 * Jusqu'ici, remplir boxincloud demandait un terminal : déposer les fichiers
 * dans le bucket avec un autre outil, puis lancer un scan en ligne de commande.
 * Une bibliothèque qu'on ne peut pas alimenter depuis son interface n'en est
 * pas une.
 *
 * Trois situations, un seul point d'entrée :
 *
 *  - aucun backend déclaré — on en crée un, puis une bibliothèque ;
 *  - une bibliothèque existe — on y dépose des fichiers ;
 *  - des fichiers sont arrivés autrement — on relance un parcours.
 */

const ACCEPTED = ".cbz,.zip,.cbr,.rar,.pdf";

type QueueItem = {
  id: string;
  file: File;
  progress: number;
  status: "pending" | "sending" | "done" | "failed";
  error?: string;
};

type IngestValue = {
  open: (files?: File[]) => void;
  uploading: boolean;
};

const IngestContext = createContext<IngestValue | null>(null);

export function useIngest(): IngestValue {
  const value = useContext(IngestContext);
  if (!value) throw new Error("useIngest hors de IngestProvider");
  return value;
}

export function IngestProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const [isOpen, setOpen] = useState(false);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [target, setTarget] = useState<string>("");
  const [folder, setFolder] = useState("");
  const pending = useRef<File[]>([]);

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const items = libraries.data?.libraries ?? [];

  // La destination par défaut est la première bibliothèque : dans le cas
  // courant — une seule — il n'y a alors rien à choisir.
  useEffect(() => {
    if (!target && items.length > 0) setTarget(items[0]!.id);
  }, [items, target]);

  const open = useCallback((files?: File[]) => {
    if (files?.length) {
      pending.current = files;
    }
    setOpen(true);
  }, []);

  // Les fichiers déposés hors du dialogue entrent dans la file à l'ouverture.
  useEffect(() => {
    if (!isOpen || pending.current.length === 0) return;
    const files = pending.current;
    pending.current = [];
    setQueue((current) => [...current, ...files.map(toQueueItem)]);
  }, [isOpen]);

  const uploading = queue.some((item) => item.status === "sending" || item.status === "pending");

  /**
   * Envoie la file, un fichier après l'autre.
   *
   * En série plutôt qu'en parallèle : plusieurs envois simultanés se partagent
   * la bande passante montante, ce qui ne fait rien gagner et rend chaque barre
   * de progression trompeuse. Un fichier à la fois arrive plus vite au premier
   * album lisible.
   */
  const send = useCallback(async () => {
    if (!target) return;

    for (const item of queue) {
      if (item.status !== "pending") continue;

      setQueue((current) =>
        current.map((q) => (q.id === item.id ? { ...q, status: "sending" } : q)),
      );

      try {
        await api.uploadComic(target, item.file, {
          folder,
          onProgress: (fraction) =>
            setQueue((current) =>
              current.map((q) => (q.id === item.id ? { ...q, progress: fraction } : q)),
            ),
        });

        setQueue((current) =>
          current.map((q) => (q.id === item.id ? { ...q, status: "done", progress: 1 } : q)),
        );
      } catch (error) {
        setQueue((current) =>
          current.map((q) =>
            q.id === item.id ? { ...q, status: "failed", error: describe(error) } : q,
          ),
        );
      }
    }

    await queryClient.invalidateQueries({ queryKey: ["comics"] });
    await queryClient.invalidateQueries({ queryKey: ["folders"] });
    await queryClient.invalidateQueries({ queryKey: ["libraries"] });
    await queryClient.invalidateQueries({ queryKey: ["series"] });
  }, [queue, target, folder, queryClient]);

  const value = useMemo<IngestValue>(() => ({ open, uploading }), [open, uploading]);

  return (
    <IngestContext.Provider value={value}>
      {children}

      {isOpen && (
        <IngestDialog
          libraries={items}
          librariesLoading={libraries.isLoading}
          target={target}
          onTarget={setTarget}
          folder={folder}
          onFolder={setFolder}
          queue={queue}
          onAddFiles={(files) => setQueue((current) => [...current, ...files.map(toQueueItem)])}
          onClearDone={() => setQueue((current) => current.filter((q) => q.status !== "done"))}
          onSend={() => void send()}
          onClose={() => setOpen(false)}
          onLibraryCreated={() => void libraries.refetch()}
        />
      )}
    </IngestContext.Provider>
  );
}

// ─── Dialogue ────────────────────────────────────────────────────────────────

function IngestDialog({
  libraries,
  librariesLoading,
  target,
  onTarget,
  folder,
  onFolder,
  queue,
  onAddFiles,
  onClearDone,
  onSend,
  onClose,
  onLibraryCreated,
}: {
  libraries: Array<{ id: string; name: string }>;
  librariesLoading: boolean;
  target: string;
  onTarget: (id: string) => void;
  folder: string;
  onFolder: (value: string) => void;
  queue: QueueItem[];
  onAddFiles: (files: File[]) => void;
  onClearDone: () => void;
  onSend: () => void;
  onClose: () => void;
  onLibraryCreated: () => void;
}) {
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

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

  const t = useT();
  const waiting = queue.filter((q) => q.status === "pending").length;
  const busy = queue.some((q) => q.status === "sending");

  return (
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("ingest.title")}
        className="rise-in flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        <header className="flex items-center justify-between border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-fg">{t("ingest.title")}</h2>
          <button
            onClick={onClose}
            aria-label={t("action.close")}
            className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {librariesLoading ? (
            <p className="py-8 text-center text-ui text-muted">{t("state.loading")}</p>
          ) : libraries.length === 0 ? (
            <FirstLibrary onCreated={onLibraryCreated} />
          ) : (
            <div className="flex flex-col gap-4">
              <Destination
                libraries={libraries}
                target={target}
                onTarget={onTarget}
                folder={folder}
                onFolder={onFolder}
              />

              {/*
                Zone de dépôt. Le glisser-déposer est le geste attendu pour
                ajouter un fichier ; le sélecteur reste là pour qui préfère,
                ou pour un navigateur qui ne le gère pas.
              */}
              <div
                onDragOver={(e) => {
                  e.preventDefault();
                  setDragging(true);
                }}
                onDragLeave={() => setDragging(false)}
                onDrop={(e) => {
                  e.preventDefault();
                  setDragging(false);
                  onAddFiles(acceptedFiles(e.dataTransfer.files));
                }}
                onClick={() => inputRef.current?.click()}
                className={cx(
                  "grid cursor-pointer place-items-center rounded-lg border-2 border-dashed p-8 text-center",
                  "transition-colors duration-(--motion-duration-fast)",
                  dragging
                    ? "border-accent bg-accent/10"
                    : "border-border hover:border-border-strong hover:bg-surface-hover",
                )}
              >
                <input
                  ref={inputRef}
                  type="file"
                  multiple
                  accept={ACCEPTED}
                  onChange={(e) => {
                    onAddFiles(acceptedFiles(e.target.files));
                    e.target.value = "";
                  }}
                  className="hidden"
                />
                <UploadIcon />
                <p className="mt-2 text-ui font-medium text-fg">
                  {t("ingest.dropHere")}
                </p>
                <p className="mt-1 text-meta text-subtle">{t("ingest.formats")}</p>
              </div>

              {queue.length > 0 && <Queue items={queue} onClearDone={onClearDone} />}
            </div>
          )}
        </div>

        {libraries.length > 0 && (
          <footer className="flex items-center gap-2 border-t border-border px-4 py-3">
            <ScanButton libraryId={target} />
            <span className="ml-auto text-meta text-subtle">
              {waiting > 0 ? t("ingest.waiting", { count: waiting }) : ""}
            </span>
            <button onClick={onClose} className={buttonClass("secondary", "sm")}>
              {t("action.close")}
            </button>
            <button
              onClick={onSend}
              disabled={waiting === 0 || busy || !target}
              className={buttonClass("primary", "sm")}
            >
              {busy
                ? t("ingest.sending")
                : waiting > 0
                  ? t("ingest.sendWithCount", { count: waiting })
                  : t("ingest.send")}
            </button>
          </footer>
        )}
      </div>
    </div>
  );
}

function Destination({
  libraries,
  target,
  onTarget,
  folder,
  onFolder,
}: {
  libraries: Array<{ id: string; name: string }>;
  target: string;
  onTarget: (id: string) => void;
  folder: string;
  onFolder: (value: string) => void;
}) {
  const t = useT();

  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("ingest.library")}</span>
        <select
          value={target}
          onChange={(e) => onTarget(e.target.value)}
          className="h-9 rounded-md border border-border bg-surface px-2 text-ui text-fg"
        >
          {libraries.map((library) => (
            <option key={library.id} value={library.id}>
              {library.name}
            </option>
          ))}
        </select>
      </label>

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">
          {t("ingest.folder")}{" "}
          <span className="normal-case tracking-normal">{t("ingest.optional")}</span>
        </span>
        <input
          value={folder}
          onChange={(e) => onFolder(e.target.value)}
          placeholder={t("ingest.folderPlaceholder")}
          className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg placeholder:text-subtle"
        />
      </label>
    </div>
  );
}

function Queue({ items, onClearDone }: { items: QueueItem[]; onClearDone: () => void }) {
  const t = useT();
  const done = items.filter((i) => i.status === "done").length;

  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="text-micro uppercase tracking-wide text-subtle">
          {t(items.length > 1 ? "ingest.fileOther" : "ingest.fileOne", { count: items.length })}
        </p>
        {done > 0 && (
          <button onClick={onClearDone} className="text-meta text-accent-text hover:underline">
            {t(done > 1 ? "ingest.clearDoneOther" : "ingest.clearDoneOne", { count: done })}
          </button>
        )}
      </div>

      <ul className="flex flex-col gap-1">
        {items.map((item) => (
          <li
            key={item.id}
            className="relative overflow-hidden rounded-md border border-border bg-surface-sunken px-3 py-2"
          >
            {/* La barre est en fond plutôt qu'à côté : elle occupe la largeur
                de la ligne sans lui voler de place. */}
            {item.status === "sending" && (
              <span
                aria-hidden="true"
                className="absolute inset-y-0 left-0 bg-accent/20 transition-[width] duration-(--motion-duration-fast)"
                style={{ width: `${item.progress * 100}%` }}
              />
            )}

            <div className="relative flex items-center gap-2">
              <StatusMark status={item.status} />
              <span className="min-w-0 flex-1 truncate text-ui text-fg" title={item.file.name}>
                {item.file.name}
              </span>
              <span className="shrink-0 text-meta tabular-nums text-subtle">
                {item.status === "sending"
                  ? `${Math.round(item.progress * 100)} %`
                  : formatBytes(item.file.size)}
              </span>
            </div>

            {item.error && (
              <p className="relative mt-1 text-meta text-danger">{item.error}</p>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

function StatusMark({ status }: { status: QueueItem["status"] }) {
  const t = useT();

  if (status === "done") {
    return (
      <svg viewBox="0 0 16 16" fill="currentColor" className="size-4 shrink-0 text-success" aria-label={t("ingest.markSent")}>
        <path d="M13.5 4.5 6.5 11.5 2.5 7.5l1-1 3 3 6-6 1 1Z" />
      </svg>
    );
  }
  if (status === "failed") {
    return (
      <svg viewBox="0 0 16 16" fill="none" className="size-4 shrink-0 text-danger" aria-label={t("ingest.markFailed")}>
        <circle cx="8" cy="8" r="6" stroke="currentColor" strokeWidth="1.5" />
        <path d="M8 5v4M8 11h.01" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    );
  }
  if (status === "sending") {
    return <span className="size-2 shrink-0 animate-pulse rounded-full bg-accent" aria-label={t("ingest.markSending")} />;
  }
  return <span className="size-2 shrink-0 rounded-full bg-border" aria-label={t("ingest.markPending")} />;
}

// ─── Première bibliothèque ───────────────────────────────────────────────────

/**
 * Création du premier stockage et de la première bibliothèque.
 *
 * C'est l'écran qui décide si quelqu'un adopte boxincloud ou le désinstalle :
 * une installation neuve n'a ni backend ni bibliothèque, et sans ce formulaire
 * il fallait un accès shell au serveur pour aller plus loin.
 *
 * Le dossier local est proposé par défaut, parce qu'il ne demande rien d'autre
 * qu'un chemin. MinIO reste disponible pour qui l'a déjà.
 */
function FirstLibrary({ onCreated }: { onCreated: () => void }) {
  const t = useT();
  const [kind, setKind] = useState<"local" | "s3">("local");
  const [name, setName] = useState(t("first.defaultName"));
  const [root, setRoot] = useState("");
  const [endpoint, setEndpoint] = useState("localhost:9000");
  const [bucket, setBucket] = useState("");
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [prefix, setPrefix] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);

    try {
      const backend = await api.createBackend({
        name: `${name} (stockage)`,
        kind,
        config:
          kind === "local"
            ? { root }
            : { endpoint, bucket, use_ssl: "false", path_style: "true" },
        secrets: kind === "s3" ? { access_key: accessKey, secret_key: secretKey } : undefined,
        isDefault: true,
      });

      await api.createLibrary({ name, backendId: backend.id, rootPrefix: prefix });
      onCreated();
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <div>
        <h3 className="text-ui font-semibold text-fg">{t("first.title")}</h3>
        <p className="mt-1 text-meta leading-relaxed text-muted">{t("first.intro")}</p>
      </div>

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("storage.fieldName")}</span>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg"
        />
      </label>

      <div className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("first.storageType")}</span>
        <div className="flex gap-1.5">
          {(["local", "s3"] as const).map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => setKind(option)}
              aria-pressed={kind === option}
              className={cx(
                "pressable rounded-md border px-3 py-1.5 text-ui font-medium",
                kind === option
                  ? "border-accent bg-accent text-inverted"
                  : "border-border text-muted hover:bg-surface-hover hover:text-fg",
              )}
            >
              {option === "local" ? t("storage.localFolder") : t("storage.kindS3")}
            </button>
          ))}
        </div>
      </div>

      {kind === "local" ? (
        <label className="flex flex-col gap-1">
          <span className="text-micro uppercase tracking-wide text-subtle">
            {t("first.localPath")}
          </span>
          <input
            value={root}
            onChange={(e) => setRoot(e.target.value)}
            required
            placeholder="/var/lib/boxincloud/bd"
            className="h-9 rounded-md border border-border bg-surface px-2.5 font-mono text-meta text-fg"
          />
        </label>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Endpoint" value={endpoint} onChange={setEndpoint} placeholder="localhost:9000" />
          <Field label="Bucket" value={bucket} onChange={setBucket} placeholder="boxincloud" />
          <Field label={t("storage.accessKey")} value={accessKey} onChange={setAccessKey} />
          <Field
            label={t("storage.secretKey")}
            value={secretKey}
            onChange={setSecretKey}
            type="password"
          />
        </div>
      )}

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">
          {t("first.subfolder")}{" "}
          <span className="normal-case tracking-normal">{t("ingest.optional")}</span>
        </span>
        <input
          value={prefix}
          onChange={(e) => setPrefix(e.target.value)}
          placeholder="bd/"
          className="h-9 rounded-md border border-border bg-surface px-2.5 font-mono text-meta text-fg"
        />
      </label>

      {error && (
        <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-meta text-danger">
          {error}
        </p>
      )}

      <button type="submit" disabled={busy} className={buttonClass("primary", "md")}>
        {busy ? t("storage.checking") : t("first.create")}
      </button>
    </form>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-micro uppercase tracking-wide text-subtle">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg placeholder:text-subtle"
      />
    </label>
  );
}

// ─── Parcours ────────────────────────────────────────────────────────────────

/**
 * Relance un parcours de la bibliothèque.
 *
 * Pour les fichiers arrivés autrement qu'en les déposant ici : un montage
 * réseau, un rsync, un bucket alimenté par ailleurs.
 */
function ScanButton({ libraryId }: { libraryId: string }) {
  const t = useT();
  const [state, setState] = useState<"idle" | "queued" | "failed">("idle");

  async function scan() {
    if (!libraryId) return;
    try {
      await api.scanLibrary(libraryId);
      setState("queued");
    } catch {
      setState("failed");
    }
  }

  return (
    <button
      onClick={() => void scan()}
      disabled={!libraryId || state === "queued"}
      title={t("scan.tooltip")}
      className={cx(buttonClass("secondary", "sm"), "disabled:opacity-60")}
    >
      {state === "queued"
        ? t("scan.queued")
        : state === "failed"
          ? t("scan.failed")
          : t("scan.button")}
    </button>
  );
}

// ─── Bouton et zone de dépôt globale ─────────────────────────────────────────

export function AddContentButton() {
  const t = useT();
  const { open, uploading } = useIngest();
  const { data: user } = useCurrentUser();

  // La création de stockage est réservée aux administrateurs ; le bouton reste
  // visible pour les autres, qui peuvent déposer dans une bibliothèque
  // existante.
  if (!user) return null;

  return (
    <button
      onClick={() => open()}
      className={cx(
        "pressable flex h-8 items-center gap-1.5 rounded-md bg-accent px-3 text-ui font-medium text-inverted",
        "hover:bg-accent-hover",
      )}
    >
      <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
        <path d="M8 3.5v9M3.5 8h9" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
      <span className="hidden sm:inline">{uploading ? t("ingest.sending") : t("ingest.add")}</span>
    </button>
  );
}

/**
 * Dépôt sur toute la fenêtre.
 *
 * Déposer un fichier n'importe où doit fonctionner : exiger de viser une zone
 * précise avant même d'avoir ouvert un dialogue serait une contrainte que rien
 * ne justifie.
 */
export function GlobalDropZone({ children }: { children: React.ReactNode }) {
  const t = useT();
  const { open } = useIngest();
  const [active, setActive] = useState(false);
  const depth = useRef(0);

  return (
    <div
      onDragEnter={(e) => {
        if (!hasFiles(e.dataTransfer)) return;
        depth.current += 1;
        setActive(true);
      }}
      onDragOver={(e) => {
        if (hasFiles(e.dataTransfer)) e.preventDefault();
      }}
      onDragLeave={() => {
        // Le compteur est nécessaire : survoler un enfant émet un dragleave sur
        // le parent, et sans lui le voile clignoterait à chaque frontière.
        depth.current -= 1;
        if (depth.current <= 0) {
          depth.current = 0;
          setActive(false);
        }
      }}
      onDrop={(e) => {
        if (!hasFiles(e.dataTransfer)) return;
        e.preventDefault();
        depth.current = 0;
        setActive(false);

        const files = acceptedFiles(e.dataTransfer.files);
        if (files.length > 0) open(files);
      }}
      className="contents"
    >
      {children}

      {active && (
        <div className="pointer-events-none fixed inset-0 z-[70] grid place-items-center bg-accent/15 backdrop-blur-[2px]">
          <div className="rounded-xl border-2 border-dashed border-accent bg-surface/90 px-8 py-6 text-center shadow-2xl">
            <UploadIcon />
            <p className="mt-2 text-title font-semibold text-fg">{t("ingest.dropToAdd")}</p>
            <p className="mt-1 text-meta text-muted">CBZ, CBR, PDF, ZIP, RAR</p>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Utilitaires ─────────────────────────────────────────────────────────────

function UploadIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" className="mx-auto size-8 text-accent" aria-hidden="true">
      <path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

let counter = 0;
function toQueueItem(file: File): QueueItem {
  counter += 1;
  return { id: `${file.name}-${counter}`, file, progress: 0, status: "pending" };
}

const EXTENSIONS = [".cbz", ".zip", ".cbr", ".rar", ".pdf"];

/**
 * Ne retient que les fichiers que le serveur accepterait.
 *
 * Filtrer ici évite d'envoyer un dossier entier d'images pour se voir refuser
 * chaque fichier un par un — le refus serait correct, mais l'aller-retour
 * inutile.
 */
function acceptedFiles(list: FileList | null): File[] {
  if (!list) return [];
  return Array.from(list).filter((file) =>
    EXTENSIONS.some((ext) => file.name.toLowerCase().endsWith(ext)),
  );
}

function hasFiles(transfer: DataTransfer): boolean {
  return Array.from(transfer.types ?? []).includes("Files");
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

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  const units = ["ko", "Mo", "Go"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}
