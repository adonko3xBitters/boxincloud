"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { buttonClass, cx } from "./ui";
import { ApiError } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";

/**
 * Stockages et bibliothèques.
 *
 * Créer était possible depuis le dialogue d'ajout ; modifier et supprimer ne
 * l'étaient pas. Une configuration qu'on ne peut que poser une fois oblige à
 * repartir de zéro pour corriger un endpoint mal tapé — et à perdre au passage
 * tout ce qui s'y rattachait.
 */
export function StoragePanel({ onClose }: { onClose: () => void }) {
  const [tab, setTab] = useState<"backends" | "libraries">("libraries");

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
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Stockage et bibliothèques"
        className="rise-in flex h-[80vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-fg">Stockage</h2>

          <div className="ml-2 flex items-center gap-0.5 rounded-md border border-border p-0.5">
            {(["libraries", "backends"] as const).map((option) => (
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
                {option === "libraries" ? "Bibliothèques" : "Espaces de stockage"}
              </button>
            ))}
          </div>

          <button
            onClick={onClose}
            aria-label="Fermer"
            className="pressable ml-auto grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
            </svg>
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {tab === "libraries" ? <Libraries /> : <Backends />}
        </div>
      </div>
    </div>
  );
}

// ─── Bibliothèques ───────────────────────────────────────────────────────────

function Libraries() {
  const [creating, setCreating] = useState(false);

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const backends = useQuery({ queryKey: ["backends"], queryFn: api.listBackends });

  const list = libraries.data?.libraries ?? [];
  const hasBackend = (backends.data?.backends.length ?? 0) > 0;

  if (libraries.isLoading) {
    return <p className="text-ui text-muted">Chargement…</p>;
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
          title="Aucun espace de stockage"
          detail="Une bibliothèque désigne un emplacement dans un espace de stockage. Commencez par en déclarer un dans l'onglet « Espaces de stockage »."
        />
      ) : creating ? (
        <NewLibrary
          backends={backends.data?.backends ?? []}
          onDone={() => setCreating(false)}
        />
      ) : (
        <button onClick={() => setCreating(true)} className={buttonClass("primary", "sm")}>
          Nouvelle bibliothèque
        </button>
      )}

      {list.length === 0 && hasBackend && !creating && (
        <p className="text-ui text-muted">Aucune bibliothèque pour l&apos;instant.</p>
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
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={create} className="flex flex-col gap-3 rounded-lg border border-accent/40 bg-accent/5 p-3">
      <h3 className="text-ui font-semibold text-fg">Nouvelle bibliothèque</h3>

      <Field label="Nom" value={name} onChange={setName} placeholder="Mes BD" />

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">Espace de stockage</span>
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
        label="Sous-dossier"
        value={prefix}
        onChange={setPrefix}
        placeholder="bd/"
        mono
        hint="Emplacement dans le stockage. Laissez vide pour prendre tout le contenu."
      />

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button type="button" onClick={onDone} className={buttonClass("secondary", "sm")}>
          Annuler
        </button>
        <button type="submit" disabled={busy || !name || !backendId} className={buttonClass("primary", "sm")}>
          {busy ? "Création…" : "Créer"}
        </button>
      </div>
    </form>
  );
}

function LibraryCard({ id, name, count }: { id: string; name: string; count: number }) {
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
      setError(describe(err));
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
      setError(describe(err));
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
            {count} album{count > 1 ? "s" : ""}
            {last && ` · dernier parcours ${statusLabel(last.status)}`}
          </p>
        </div>

        <button onClick={() => void scan()} className={buttonClass("secondary", "sm")}>
          Analyser
        </button>
        <button onClick={() => setEditing((v) => !v)} className={buttonClass("secondary", "sm")}>
          {editing ? "Annuler" : "Modifier"}
        </button>
      </div>

      {editing && (
        <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
          <Field label="Nom" value={nextName} onChange={setNextName} />
          <Field
            label="Préfixe racine"
            value={prefix}
            onChange={setPrefix}
            placeholder="inchangé"
            mono
            hint="Changer le préfixe ne déplace rien : les albums déjà indexés pointent l'ancien. Le changement dit où chercher désormais, et un nouveau parcours reconstruit le catalogue."
          />

          <div className="flex justify-end gap-2">
            <button onClick={() => void save()} disabled={busy} className={buttonClass("primary", "sm")}>
              {busy ? "Enregistrement…" : "Enregistrer"}
            </button>
          </div>

          <div className="border-t border-border pt-3">
            {!confirming ? (
              <button
                onClick={() => setConfirming(true)}
                className="pressable rounded-md border border-danger/40 px-3 py-1.5 text-ui font-medium text-danger hover:bg-danger/10"
              >
                Supprimer cette bibliothèque
              </button>
            ) : (
              <div className="flex flex-col gap-2 rounded-md border border-danger/40 bg-danger/10 p-3">
                <p className="text-meta leading-relaxed text-fg">
                  Albums, dossiers, progression de lecture, favoris, notes et
                  partages disparaissent. <strong>Vos fichiers restent intacts</strong> —
                  recréer la bibliothèque sur le même préfixe les retrouve tous.
                  L&apos;historique de lecture, lui, ne revient pas.
                </p>
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
                <div className="flex justify-end gap-2">
                  <button onClick={() => setConfirming(false)} className={buttonClass("secondary", "sm")}>
                    Annuler
                  </button>
                  <button
                    onClick={() => void remove()}
                    disabled={confirmed.trim().toLowerCase() !== "supprimer" || busy}
                    className="pressable rounded-md bg-danger px-3 py-1.5 text-ui font-medium text-white disabled:opacity-40"
                  >
                    Supprimer définitivement
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
  const [open, setOpen] = useState(false);

  return (
    <div className="mt-2.5 border-t border-border pt-2.5">
      <button
        onClick={() => setOpen((v) => !v)}
        className="pressable text-meta text-accent-text hover:underline"
      >
        {open ? "Masquer" : "Voir"} les {runs.length} derniers parcours
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
                <span className="text-meta text-fg">{statusLabel(run.status)}</span>
                <span className="text-meta tabular-nums text-subtle">
                  {new Date(run.startedAt).toLocaleString("fr-FR")}
                </span>
                <span className="ml-auto text-meta tabular-nums text-subtle">
                  {run.objectsSeen} vus · {run.added} ajoutés · {run.updated} modifiés
                  {run.removed > 0 && ` · ${run.removed} disparus`}
                  {run.errors > 0 && ` · ${run.errors} erreurs`}
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
  const [creating, setCreating] = useState(false);
  const backends = useQuery({ queryKey: ["backends"], queryFn: api.listBackends });
  const list = backends.data?.backends ?? [];

  if (backends.isLoading) return <p className="text-ui text-muted">Chargement…</p>;

  return (
    <div className="flex flex-col gap-3">
      {creating ? (
        <NewBackend onDone={() => setCreating(false)} />
      ) : (
        <button onClick={() => setCreating(true)} className={buttonClass("primary", "sm")}>
          Nouvel espace de stockage
        </button>
      )}

      {list.length === 0 && !creating && (
        <Notice
          title="Aucun espace de stockage"
          detail="Un espace de stockage est l'endroit où vivent réellement vos fichiers : un dossier du serveur, ou un bucket S3 / MinIO. boxincloud n'en héberge aucun — il lit le vôtre."
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
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={create} className="flex flex-col gap-3 rounded-lg border border-accent/40 bg-accent/5 p-3">
      <h3 className="text-ui font-semibold text-fg">Nouvel espace de stockage</h3>

      <Field label="Nom" value={name} onChange={setName} placeholder="NAS du salon" />

      <div className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">Type</span>
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
              {option === "local" ? "Dossier du serveur" : "S3 / MinIO"}
            </button>
          ))}
        </div>
      </div>

      {kind === "local" ? (
        <Field
          label="Chemin du dossier"
          value={root}
          onChange={setRoot}
          placeholder="/var/lib/boxincloud/bd"
          mono
          hint="Chemin tel que le SERVEUR le voit, pas votre poste."
        />
      ) : (
        <>
          <Field label="Endpoint" value={endpoint} onChange={setEndpoint} mono />
          <Field label="Bucket" value={bucket} onChange={setBucket} mono />
          <Field label="Clé d'accès" value={accessKey} onChange={setAccessKey} />
          <Field label="Clé secrète" value={secretKey} onChange={setSecretKey} type="password" />
        </>
      )}

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="flex justify-end gap-2">
        <button type="button" onClick={onDone} className={buttonClass("secondary", "sm")}>
          Annuler
        </button>
        <button type="submit" disabled={busy || !name} className={buttonClass("primary", "sm")}>
          {busy ? "Vérification du stockage…" : "Déclarer"}
        </button>
      </div>

      <p className="text-meta leading-relaxed text-subtle">
        Le stockage est joint avant d&apos;être enregistré : un chemin ou des
        identifiants erronés sont signalés tout de suite, pas au premier scan.
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
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  async function test() {
    setTested(null);
    const result = await api.testBackend(backend.id);
    setTested(result.ok ? "Le stockage répond." : (result.detail ?? "Injoignable."));
  }

  async function remove() {
    setBusy(true);
    setError(null);
    try {
      await api.deleteBackend(backend.id);
      await queryClient.invalidateQueries({ queryKey: ["backends"] });
    } catch (err) {
      setError(describe(err));
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
                par défaut
              </span>
            )}
            {backend.readOnly && (
              <span className="rounded bg-surface-sunken px-1.5 py-0.5 text-micro text-subtle">
                lecture seule
              </span>
            )}
          </p>
          <p className="text-meta text-subtle">
            {backend.kind === "local" ? "Dossier local" : "S3 / MinIO"} ·{" "}
            {backend.kind === "local" ? backend.config.root : backend.config.endpoint}
          </p>
        </div>

        <button onClick={() => void test()} className={buttonClass("secondary", "sm")}>
          Tester
        </button>
        <button onClick={() => setEditing((v) => !v)} className={buttonClass("secondary", "sm")}>
          {editing ? "Annuler" : "Modifier"}
        </button>
      </div>

      {tested && (
        <p className="mt-2 rounded-md border border-border bg-surface-sunken px-2.5 py-1.5 text-meta text-muted">
          {tested}
        </p>
      )}

      {editing && (
        <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
          <Field label="Nom" value={name} onChange={setName} />

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
                  placeholder="inchangé"
                />
              ))}
              <p className="text-meta leading-relaxed text-subtle">
                Laissez vides pour conserver les identifiants actuels : ils ne
                ressortent jamais de la base, pas même pour un administrateur.
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
                Définir par défaut
              </button>
            )}

            <button
              onClick={() => void remove()}
              disabled={busy}
              className="pressable rounded-md border border-danger/40 px-3 py-1.5 text-ui font-medium text-danger hover:bg-danger/10"
            >
              Supprimer
            </button>

            <button
              onClick={() => void save()}
              disabled={busy}
              className={cx(buttonClass("primary", "sm"), "ml-auto")}
            >
              {busy ? "Vérification…" : "Enregistrer"}
            </button>
          </div>

          <p className="text-meta leading-relaxed text-subtle">
            Le stockage est joint avant d&apos;être enregistré. Sa suppression est
            refusée tant qu&apos;une bibliothèque s&apos;y appuie — vos fichiers ne
            sont jamais touchés.
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

function statusLabel(status: string): string {
  switch (status) {
    case "success":
      return "réussi";
    case "running":
      return "en cours";
    case "failed":
      return "en échec";
    case "cancelled":
      return "interrompu";
    default:
      return status;
  }
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
