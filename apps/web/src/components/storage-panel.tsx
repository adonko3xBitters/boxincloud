"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button, Shell, Spinner, buttonClass, cx } from "./ui";
import * as api from "@/lib/api/endpoints";
import { describeError } from "@/lib/api/problem";
import { useLocale, useT, type MessageKey } from "@/i18n";

/**
 * Stockages et bibliothèques.
 *
 * Créer était possible depuis le dialogue d'ajout ; modifier et supprimer ne
 * l'étaient pas. Une configuration qu'on ne peut que poser une fois oblige à
 * repartir de zéro pour corriger un endpoint mal tapé — et à perdre au passage
 * tout ce qui s'y rattachait.
 */
export function StoragePanel({
  onClose,
  embedded = false,
}: {
  onClose: () => void;
  /*
    Le panneau sert deux surfaces : une boîte de dialogue empilée sur la
    bibliothèque, et une section de la page Configuration.

    `embedded` retire l'enveloppe plein écran et la croix de fermeture — sur une
    page, on revient par le fil d'Ariane ou le bouton du navigateur, et une
    croix qui ne fermerait rien serait un piège.

    Un booléen plutôt que deux composants : le corps du panneau est identique,
    et le dupliquer garantirait qu'une correction n'atteigne qu'une des deux
    copies.
  */
  embedded?: boolean;
}) {
  const t = useT();
  const [tab, setTab] = useState<"libraries" | "backends" | "cache">("libraries");

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

  return (
    <Shell
      embedded={embedded}
      label={t("storage.dialogLabel")}
      className="rise-in flex h-[80vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
    >
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-fg">{t("storage.title")}</h2>

          <div className="ml-2 flex items-center gap-0.5 rounded-md border border-border p-0.5">
            {(["libraries", "backends", "cache"] as const).map((option) => (
              <button
                key={option}
                onClick={() => setTab(option)}
                aria-pressed={tab === option}
                className={cx(
                  "pressable rounded px-2.5 py-1 text-meta font-medium",
                  tab === option
                    ? "bg-accent text-inverted shadow-sm"
                    : "text-muted hover:bg-surface-hover hover:text-fg",
                )}
              >
                {t(TAB_KEYS[option])}
              </button>
            ))}
          </div>

          {!embedded && (
            <button
              onClick={onClose}
              aria-label={t("action.close")}
              className="pressable ml-auto grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
            >
              <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
                <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
              </svg>
            </button>
          )}
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {tab === "libraries" && <Libraries />}
          {tab === "backends" && <Backends />}
          {tab === "cache" && <CacheSection />}
        </div>
    </Shell>
  );
}

const TAB_KEYS: Record<"libraries" | "backends" | "cache", MessageKey> = {
  libraries: "storage.libraries",
  backends: "storage.backends",
  cache: "cache.tab",
};

// ─── Cache dérivé ────────────────────────────────────────────────────────────

/**
 * Occupation du cache dérivé, et bouton pour le vider.
 *
 * La purge est présentée sans dramatisation parce qu'elle n'en mérite pas :
 * tout ce que le cache contient se régénère depuis les archives d'origine. La
 * confirmation existe pour le temps que coûte la régénération, pas pour un
 * risque de perte.
 */
function CacheSection() {
  const { locale, t } = useLocale();
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);
  const [freed, setFreed] = useState<{ entries: number; bytes: number } | null>(null);

  const stats = useQuery({ queryKey: ["cache"], queryFn: api.getCacheStats });

  const purge = useMutation({
    mutationFn: api.purgeCache,
    onSuccess: (result) => {
      setFreed(result);
      setConfirming(false);
      void queryClient.invalidateQueries({ queryKey: ["cache"] });
    },
  });

  if (stats.isLoading) return <Spinner className="size-5 text-muted" />;
  if (stats.error) return <ErrorNote>{describeError(stats.error, t)}</ErrorNote>;

  const data = stats.data;
  if (!data) return null;

  const ratio = data.maxBytes && data.maxBytes > 0 ? data.bytes / data.maxBytes : 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-lg border border-border bg-surface-sunken p-4">
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-semibold text-fg">{formatBytes(data.bytes)}</span>
          {data.maxBytes ? (
            <span className="text-meta text-subtle">{t("cache.of")} {formatBytes(data.maxBytes)}</span>
          ) : (
            <span className="text-meta text-subtle">{t("cache.unbounded")}</span>
          )}
        </div>

        {data.maxBytes ? (
          <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-border">
            <div
              className={cx("h-full rounded-full", ratio > 0.9 ? "bg-danger" : "bg-accent")}
              style={{ width: `${Math.min(ratio * 100, 100)}%` }}
            />
          </div>
        ) : null}

        <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 text-meta sm:grid-cols-3">
          <Stat label={t("cache.entries")} value={data.entries.toLocaleString("fr-FR")} />
          <Stat label={t("cache.hits")} value={data.hits.toLocaleString("fr-FR")} />
          {data.oldestAt && (
            <Stat label={t("cache.oldest")} value={formatDate(data.oldestAt)} />
          )}
        </dl>
      </div>

      <p className="text-meta leading-relaxed text-subtle">
{t("cache.explain")}
      </p>

      {freed && (
        <p className="rounded-md border border-border bg-surface-sunken px-3 py-2 text-meta text-muted">
{t("cache.freed", { entries: freed.entries.toLocaleString(locale), bytes: formatBytes(freed.bytes) })}
        </p>
      )}

      {purge.error && <ErrorNote>{describeError(purge.error, t)}</ErrorNote>}

      {confirming ? (
        <div className="flex items-center gap-2">
          <Button variant="danger" loading={purge.isPending} onClick={() => purge.mutate()}>
            {t("cache.purgeConfirm")}
          </Button>
          <Button variant="ghost" onClick={() => setConfirming(false)}>
            {t("action.cancel")}
          </Button>
        </div>
      ) : (
        <div>
          <Button
            variant="secondary"
            disabled={data.entries === 0}
            onClick={() => setConfirming(true)}
          >
            {t("cache.purge")}
          </Button>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-subtle">{label}</dt>
      <dd className="font-medium text-fg tabular-nums">{value}</dd>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;

  const units = ["ko", "Mo", "Go", "To"];
  let value = bytes / 1024;
  let unit = 0;

  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }

  return unit >= 1
    ? `${value.toFixed(1).replace(".", ",")} ${units[unit]}`
    : `${Math.round(value)} ${units[unit]}`;
}

function formatDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleDateString("fr-FR", { day: "numeric", month: "short", year: "numeric" });
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

function Libraries() {
  const t = useT();
  const [creating, setCreating] = useState(false);

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const backends = useQuery({ queryKey: ["backends"], queryFn: api.listBackends });

  const list = libraries.data?.libraries ?? [];
  const hasBackend = (backends.data?.backends.length ?? 0) > 0;

  if (libraries.isLoading) {
    return <p className="text-ui text-muted">{t("state.loading")}</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {/*
        Une bibliothèque a besoin d'un stockage. Le dire ici, avec le chemin
        pour en créer un, évite un formulaire qui échouerait sans expliquer
        pourquoi.
      */}
      {!hasBackend ? (
        <Notice
          title={t("storage.noBackendTitle")}
          detail={t("storage.noBackendDetail")}
        />
      ) : creating ? (
        <NewLibrary
          backends={backends.data?.backends ?? []}
          onDone={() => setCreating(false)}
        />
      ) : (
        <button onClick={() => setCreating(true)} className={buttonClass("primary", "sm")}>
          {t("storage.newLibrary")}
        </button>
      )}

      {list.length === 0 && hasBackend && !creating && (
        <p className="text-ui text-muted">{t("storage.noLibraries")}</p>
      )}

      {list.map((library) => (
        <LibraryCard key={library.id} id={library.id} name={library.name} count={library.comicCount} />
      ))}
    </div>
  );
}

/**
 * Création d'une bibliothèque.
 *
 * Une bibliothèque n'est pas un dossier : c'est un emplacement DANS un espace de
 * stockage, désigné par un préfixe. Le formulaire le dit, sans quoi le champ
 * « préfixe » ressemble à un réglage obscur qu'on laisse vide par prudence.
 */
function NewLibrary({
  backends,
  onDone,
}: {
  backends: api.StorageBackend[];
  onDone: () => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [backendId, setBackendId] = useState(
    backends.find((b) => b.isDefault)?.id ?? backends[0]?.id ?? "",
  );
  const [prefix, setPrefix] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.createLibrary({ name, backendId, rootPrefix: prefix });
      await queryClient.invalidateQueries({ queryKey: ["libraries"] });
      onDone();
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={create} className="flex flex-col gap-3 rounded-lg border border-accent/40 bg-accent/5 p-3">
      <h3 className="text-ui font-semibold text-fg">{t("storage.newLibrary")}</h3>

      <Field
        label={t("storage.fieldName")}
        value={name}
        onChange={setName}
        placeholder={t("storage.libraryNamePlaceholder")}
      />

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("storage.backendField")}</span>
        <select
          value={backendId}
          onChange={(e) => setBackendId(e.target.value)}
          className="h-9 rounded-md border border-border bg-surface px-2 text-ui text-fg"
        >
          {backends.map((backend) => (
            <option key={backend.id} value={backend.id}>
              {backend.name}
            </option>
          ))}
        </select>
      </label>

      <Field
        label={t("storage.subfolder")}
        value={prefix}
        onChange={setPrefix}
        placeholder="bd/"
        mono
        hint={t("storage.subfolderHint")}
      />

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button type="button" onClick={onDone} className={buttonClass("secondary", "sm")}>
          {t("action.cancel")}
        </button>
        <button type="submit" disabled={busy || !name || !backendId} className={buttonClass("primary", "sm")}>
          {busy ? t("storage.creating") : t("action.create")}
        </button>
      </div>
    </form>
  );
}

function LibraryCard({ id, name, count }: { id: string; name: string; count: number }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [nextName, setNextName] = useState(name);
  const [prefix, setPrefix] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [confirmed, setConfirmed] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runs = useQuery({
    queryKey: ["scan-runs", id],
    queryFn: () => api.listScanRuns(id),
  });

  const last = runs.data?.runs[0];

  async function save() {
    setBusy(true);
    setError(null);
    try {
      await api.updateLibrary(id, {
        name: nextName !== name ? nextName : undefined,
        rootPrefix: prefix || undefined,
      });
      await queryClient.invalidateQueries({ queryKey: ["libraries"] });
      setEditing(false);
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    setError(null);
    try {
      await api.deleteLibrary(id);
      await queryClient.invalidateQueries({ queryKey: ["libraries"] });
      await queryClient.invalidateQueries({ queryKey: ["comics"] });
      await queryClient.invalidateQueries({ queryKey: ["folders"] });
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  async function scan() {
    await api.scanLibrary(id);
    await queryClient.invalidateQueries({ queryKey: ["scan-runs", id] });
  }

  return (
    <section className="rounded-lg border border-border p-3">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <p className="truncate text-ui font-medium text-fg">{name}</p>
          <p className="text-meta text-subtle">
            {t(count > 1 ? "storage.albumOther" : "storage.albumOne", { count })}
            {last && ` · ${t("storage.lastScan", { status: t(statusKey(last.status)) })}`}
          </p>
        </div>

        <button onClick={() => void scan()} className={buttonClass("secondary", "sm")}>
          {t("storage.scan")}
        </button>
        <button onClick={() => setEditing((v) => !v)} className={buttonClass("secondary", "sm")}>
          {editing ? t("action.cancel") : t("storage.edit")}
        </button>
      </div>

      {editing && (
        <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
          <Field label={t("storage.fieldName")} value={nextName} onChange={setNextName} />
          <Field
            label={t("storage.rootPrefix")}
            value={prefix}
            onChange={setPrefix}
            placeholder={t("storage.unchanged")}
            mono
            hint={t("storage.rootPrefixHint")}
          />

          <div className="flex justify-end gap-2">
            <button onClick={() => void save()} disabled={busy} className={buttonClass("primary", "sm")}>
              {busy ? t("storage.saving") : t("action.save")}
            </button>
          </div>

          <div className="border-t border-border pt-3">
            {!confirming ? (
              <button
                onClick={() => setConfirming(true)}
                className="pressable rounded-md border border-danger/40 px-3 py-1.5 text-ui font-medium text-danger hover:bg-danger/10"
              >
                {t("storage.deleteLibrary")}
              </button>
            ) : (
              <div className="flex flex-col gap-2 rounded-md border border-danger/40 bg-danger/10 p-3">
                <p className="text-meta leading-relaxed text-fg">
                  {t("storage.deleteLibraryWarning")}
                </p>
                <label className="flex flex-col gap-1">
                  <span className="text-meta text-muted">
                    {t("storage.typeToConfirm", { word: t("storage.confirmWord") })}
                  </span>
                  <input
                    value={confirmed}
                    onChange={(e) => setConfirmed(e.target.value)}
                    autoFocus
                    className="h-9 rounded-md border border-danger/50 bg-surface px-2.5 text-ui text-fg"
                  />
                </label>
                <div className="flex justify-end gap-2">
                  <button onClick={() => setConfirming(false)} className={buttonClass("secondary", "sm")}>
                    {t("action.cancel")}
                  </button>
                  <button
                    onClick={() => void remove()}
                    disabled={
                      confirmed.trim().toLowerCase() !== t("storage.confirmWord").toLowerCase() ||
                      busy
                    }
                    className="pressable rounded-md bg-danger px-3 py-1.5 text-ui font-medium text-white disabled:opacity-40"
                  >
                    {t("storage.deleteForever")}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {error && <div className="mt-2"><ErrorNote>{error}</ErrorNote></div>}

      {runs.data && runs.data.runs.length > 0 && <ScanHistory runs={runs.data.runs} />}
    </section>
  );
}

/**
 * Historique des parcours.
 *
 * C'est le seul endroit où l'on voit POURQUOI un parcours a échoué. Sans lui, un
 * scan en erreur ne se manifeste que par une bibliothèque qui ne se remplit pas,
 * sans le moindre indice.
 */
function ScanHistory({ runs }: { runs: api.ScanRun[] }) {
  const { locale, t } = useLocale();
  const [open, setOpen] = useState(false);

  return (
    <div className="mt-2.5 border-t border-border pt-2.5">
      <button
        onClick={() => setOpen((v) => !v)}
        className="pressable text-meta text-accent-text hover:underline"
      >
        {open ? t("scan.hide") : t("scan.show")} {t("scan.lastRuns", { count: runs.length })}
      </button>

      {open && (
        <ul className="mt-2 flex flex-col gap-1">
          {runs.map((run) => (
            <li key={run.id} className="rounded-md bg-surface-sunken px-2.5 py-1.5">
              <div className="flex items-center gap-2">
                <span
                  className={cx(
                    "size-2 shrink-0 rounded-full",
                    run.status === "success"
                      ? "bg-success"
                      : run.status === "running"
                        ? "animate-pulse bg-accent"
                        : "bg-danger",
                  )}
                />
                <span className="text-meta text-fg">{t(statusKey(run.status))}</span>
                <span className="text-meta tabular-nums text-subtle">
                  {new Date(run.startedAt).toLocaleString(locale)}
                </span>
                <span className="ml-auto text-meta tabular-nums text-subtle">
                  {t("scan.counts", {
                    seen: run.objectsSeen,
                    added: run.added,
                    updated: run.updated,
                  })}
                  {run.removed > 0 && t("scan.removed", { count: run.removed })}
                  {run.errors > 0 && t("scan.errors", { count: run.errors })}
                </span>
              </div>
              {run.detail && run.detail !== "{}" && (
                <p className="mt-1 break-all font-mono text-micro text-danger">{run.detail}</p>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

// ─── Espaces de stockage ─────────────────────────────────────────────────────

function Backends() {
  const t = useT();
  const [creating, setCreating] = useState(false);
  const backends = useQuery({ queryKey: ["backends"], queryFn: api.listBackends });
  const list = backends.data?.backends ?? [];

  if (backends.isLoading) return <p className="text-ui text-muted">{t("state.loading")}</p>;

  return (
    <div className="flex flex-col gap-3">
      {creating ? (
        <NewBackend onDone={() => setCreating(false)} />
      ) : (
        <button onClick={() => setCreating(true)} className={buttonClass("primary", "sm")}>
          {t("storage.newBackend")}
        </button>
      )}

      {list.length === 0 && !creating && (
        <Notice
          title={t("storage.noBackendTitle")}
          detail={t("storage.noBackendsDetail")}
        />
      )}

      {list.map((backend) => (
        <BackendCard key={backend.id} backend={backend} />
      ))}
    </div>
  );
}

/**
 * Déclaration d'un espace de stockage.
 *
 * Le dossier local est proposé en premier parce qu'il ne demande qu'un chemin.
 * S3 reste disponible pour qui l'a déjà, mais l'imposer d'emblée ferait fuir
 * quelqu'un qui veut juste lire les fichiers de son NAS.
 */
function NewBackend({ onDone }: { onDone: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState<"local" | "s3">("local");
  const [name, setName] = useState("");
  const [root, setRoot] = useState("");
  const [endpoint, setEndpoint] = useState("localhost:9000");
  const [bucket, setBucket] = useState("");
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.createBackend({
        name,
        kind,
        config:
          kind === "local"
            ? { root }
            : { endpoint, bucket, use_ssl: "false", path_style: "true" },
        secrets: kind === "s3" ? { access_key: accessKey, secret_key: secretKey } : undefined,
      });
      await queryClient.invalidateQueries({ queryKey: ["backends"] });
      onDone();
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={create} className="flex flex-col gap-3 rounded-lg border border-accent/40 bg-accent/5 p-3">
      <h3 className="text-ui font-semibold text-fg">{t("storage.newBackend")}</h3>

      <Field
        label={t("storage.fieldName")}
        value={name}
        onChange={setName}
        placeholder={t("storage.backendNamePlaceholder")}
      />

      <div className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("storage.type")}</span>
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
              {option === "local" ? t("storage.kindLocal") : t("storage.kindS3")}
            </button>
          ))}
        </div>
      </div>

      {kind === "local" ? (
        <Field
          label={t("storage.folderPath")}
          value={root}
          onChange={setRoot}
          placeholder="/var/lib/boxincloud/bd"
          mono
          hint={t("storage.folderPathHint")}
        />
      ) : (
        <>
          <Field label={t("storage.endpoint")} value={endpoint} onChange={setEndpoint} mono />
          <Field label={t("storage.bucket")} value={bucket} onChange={setBucket} mono />
          <Field label={t("storage.accessKey")} value={accessKey} onChange={setAccessKey} />
          <Field
            label={t("storage.secretKey")}
            value={secretKey}
            onChange={setSecretKey}
            type="password"
          />
        </>
      )}

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button type="button" onClick={onDone} className={buttonClass("secondary", "sm")}>
          {t("action.cancel")}
        </button>
        <button type="submit" disabled={busy || !name} className={buttonClass("primary", "sm")}>
          {busy ? t("storage.checking") : t("storage.declare")}
        </button>
      </div>

      <p className="text-meta leading-relaxed text-subtle">
{t("storage.checkedBeforeSaving")}
      </p>
    </form>
  );
}

/** Message explicatif, pour un écran vide qui doit dire quoi faire. */
function Notice({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="rounded-lg border border-border bg-surface-sunken p-3">
      <p className="text-ui font-medium text-fg">{title}</p>
      <p className="mt-1 text-meta leading-relaxed text-muted">{detail}</p>
    </div>
  );
}

function BackendCard({ backend }: { backend: api.StorageBackend }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(backend.name);
  const [config, setConfig] = useState<Record<string, string>>(backend.config);
  const [secrets, setSecrets] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tested, setTested] = useState<string | null>(null);

  async function save() {
    setBusy(true);
    setError(null);
    try {
      const filled = Object.fromEntries(Object.entries(secrets).filter(([, v]) => v !== ""));
      await api.updateBackend(backend.id, {
        name: name !== backend.name ? name : undefined,
        config,
        secrets: Object.keys(filled).length > 0 ? filled : undefined,
      });
      await queryClient.invalidateQueries({ queryKey: ["backends"] });
      setSecrets({});
      setEditing(false);
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  async function test() {
    setTested(null);
    const result = await api.testBackend(backend.id);
    setTested(result.ok ? t("storage.reachable") : (result.detail ?? t("storage.unreachable")));
  }

  async function remove() {
    setBusy(true);
    setError(null);
    try {
      await api.deleteBackend(backend.id);
      await queryClient.invalidateQueries({ queryKey: ["backends"] });
    } catch (err) {
      setError(describeError(err, t));
    } finally {
      setBusy(false);
    }
  }

  const keys = backend.kind === "local" ? ["root"] : ["endpoint", "bucket", "region"];
  const secretKeys = backend.kind === "s3" ? ["access_key", "secret_key"] : [];

  return (
    <section className="rounded-lg border border-border p-3">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-2 truncate text-ui font-medium text-fg">
            {backend.name}
            {backend.isDefault && (
              <span className="rounded bg-accent-subtle px-1.5 py-0.5 text-micro text-accent-text">
                {t("storage.isDefault")}
              </span>
            )}
            {backend.readOnly && (
              <span className="rounded bg-surface-sunken px-1.5 py-0.5 text-micro text-subtle">
                {t("storage.readOnly")}
              </span>
            )}
          </p>
          <p className="text-meta text-subtle">
            {backend.kind === "local" ? t("storage.localFolder") : t("storage.kindS3")} ·{" "}
            {backend.kind === "local" ? backend.config.root : backend.config.endpoint}
          </p>
        </div>

        <button onClick={() => void test()} className={buttonClass("secondary", "sm")}>
          {t("storage.test")}
        </button>
        <button onClick={() => setEditing((v) => !v)} className={buttonClass("secondary", "sm")}>
          {editing ? t("action.cancel") : t("storage.edit")}
        </button>
      </div>

      {tested && (
        <p className="mt-2 rounded-md border border-border bg-surface-sunken px-2.5 py-1.5 text-meta text-muted">
          {tested}
        </p>
      )}

      {editing && (
        <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
          <Field label={t("storage.fieldName")} value={name} onChange={setName} />

          {keys.map((key) => (
            <Field
              key={key}
              label={key}
              value={config[key] ?? ""}
              onChange={(v) => setConfig((c) => ({ ...c, [key]: v }))}
              mono
            />
          ))}

          {secretKeys.length > 0 && (
            <>
              {secretKeys.map((key) => (
                <Field
                  key={key}
                  label={key}
                  value={secrets[key] ?? ""}
                  onChange={(v) => setSecrets((s) => ({ ...s, [key]: v }))}
                  type="password"
                  placeholder={t("storage.unchanged")}
                />
              ))}
              <p className="text-meta leading-relaxed text-subtle">
{t("storage.keepSecrets")}
              </p>
            </>
          )}

          {error && <ErrorNote>{error}</ErrorNote>}

          <div className="flex items-center gap-2">
            {!backend.isDefault && (
              <button
                onClick={async () => {
                  await api.setDefaultBackend(backend.id);
                  await queryClient.invalidateQueries({ queryKey: ["backends"] });
                }}
                className={buttonClass("secondary", "sm")}
              >
                {t("storage.setDefault")}
              </button>
            )}

            <button
              onClick={() => void remove()}
              disabled={busy}
              className="pressable rounded-md border border-danger/40 px-3 py-1.5 text-ui font-medium text-danger hover:bg-danger/10"
            >
              {t("action.delete")}
            </button>

            <button
              onClick={() => void save()}
              disabled={busy}
              className={cx(buttonClass("primary", "sm"), "ml-auto")}
            >
              {busy ? t("storage.verifying") : t("action.save")}
            </button>
          </div>

          <p className="text-meta leading-relaxed text-subtle">
{t("storage.backendFooter")}
          </p>
        </div>
      )}
    </section>
  );
}

// ─── Éléments ────────────────────────────────────────────────────────────────

function Field({
  label,
  value,
  onChange,
  placeholder,
  hint,
  type = "text",
  mono,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;
  type?: string;
  mono?: boolean;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-micro uppercase tracking-wide text-subtle">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete={type === "password" ? "new-password" : "off"}
        className={cx(
          "h-9 rounded-md border border-border bg-surface px-2.5 text-fg placeholder:text-subtle",
          mono ? "font-mono text-meta" : "text-ui",
        )}
      />
      {hint && <span className="text-meta leading-relaxed text-subtle">{hint}</span>}
    </label>
  );
}

function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-meta leading-relaxed text-danger">
      {children}
    </p>
  );
}

/**
 * Clé de traduction d'un statut de parcours.
 *
 * Une clé plutôt qu'un libellé : la fonction n'a pas accès au catalogue, et lui
 * passer `t` en paramètre pour qu'elle rende une chaîne reviendrait au même en
 * plus indirect. Un statut inconnu retombe sur « en cours » — le serveur n'en
 * produit pas d'autre, et inventer un libellé pour une valeur qu'on ne connaît
 * pas serait pire que d'en supposer une.
 */
function statusKey(status: string): MessageKey {
  switch (status) {
    case "success":
      return "scan.status.success";
    case "failed":
      return "scan.status.failed";
    case "cancelled":
      return "scan.status.cancelled";
    default:
      return "scan.status.running";
  }
}

