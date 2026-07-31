"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Badge, Button, EmptyState, Input, Spinner, cx } from "./ui";
import * as api from "@/lib/api/endpoints";
import { describeError } from "@/lib/api/problem";
import { useT, type MessageKey } from "@/i18n";

/**
 * Découvrir — recherche fédérée sur des catalogues OPDS.
 *
 * Ce que cet écran doit rendre lisible, plus que les résultats eux-mêmes :
 * **ce qui manque**. Une recherche fédérée réussit partiellement par nature, et
 * une liste courte peut vouloir dire deux choses opposées — le titre n'existe
 * nulle part, ou la moitié des catalogues n'a pas répondu. Sans l'état de
 * chaque catalogue affiché à côté, l'utilisateur conclut toujours la première.
 */

const STATUS_KEYS: Record<string, MessageKey> = {
  unreachable: "discovery.status.unreachable",
  timeout: "discovery.status.timeout",
  canceled: "discovery.status.canceled",
  "no-search": "discovery.status.no-search",
  invalid: "discovery.status.invalid",
};

/*
DiscoverySheet — la recherche fédérée, en panneau du bas.

Elle n'est plus dans le menu du compte. Ce menu répondait à « qui suis-je et que
puis-je régler » ; chercher ailleurs n'est ni l'un ni l'autre, et l'y ranger
obligeait à traverser un menu pour atteindre une action fréquente.

Le panneau monte depuis le bord inférieur, là où celui de la recherche locale
glisse depuis la droite. La distinction est délibérée : deux surfaces qui
arrivent du même endroit se confondent, et le sens d'entrée devient ici le seul
indice permanent de ce qu'on est en train d'interroger — sa propre bibliothèque
ou les catalogues du dehors.

Raccourci : Cmd/Ctrl + Maj + F. Le « F » de « fédérée », la touche Maj le
distinguant du Cmd-F du navigateur, et le tout laissant Cmd-K à la recherche
locale — la plus fréquente garde le raccourci le plus court.

L'administration des catalogues n'est plus ici. Régler une source est une
opération de configuration, elle a rejoint le hub ; ce panneau ne fait que
chercher, et il est ouvert à tout compte.
*/
export function DiscoverySheet({ onClose }: { onClose: () => void }) {
  const t = useT();

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
    /*
      `items-end` plutôt qu'un centrage : le panneau est ancré au bas de
      l'écran, d'où il vient. Le centrer annulerait le sens de son entrée.
    */
    <div className="fixed inset-0 z-[60] flex items-end justify-center bg-[var(--overlay)]">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("discovery.dialogLabel")}
        className="slide-in-bottom flex max-h-[85vh] w-full max-w-4xl flex-col overflow-hidden rounded-t-xl border border-b-0 border-border bg-surface shadow-2xl"
      >
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          {/*
            Une poignée, comme sur les feuilles du bas des systèmes mobiles :
            elle dit que la surface est ancrée en bas et qu'elle s'en ira par là.
          */}
          <span
            aria-hidden="true"
            className="absolute left-1/2 top-1.5 h-1 w-10 -translate-x-1/2 rounded-full bg-border-strong"
          />

          <h2 className="text-title font-semibold text-fg">{t("discovery.title")}</h2>

          <button
            onClick={onClose}
            aria-label={t("action.close")}
            className="pressable ml-auto grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path
                d="m4 4 8 8M12 4l-8 8"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
              />
            </svg>
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <SearchSection />
        </div>
      </div>
    </div>
  );
}

/*
DiscoverySources — l'administration des catalogues, pour le hub.

Séparée du panneau de recherche parce que ce sont deux publics et deux moments :
on cherche souvent, on configure une fois. Les garder ensemble obligeait à
montrer un onglet réservé aux administrateurs dans une surface ouverte à tous.
*/
export function DiscoverySources() {
  return <SourcesSection />;
}

// ─── Recherche ───────────────────────────────────────────────────────────────

function SearchSection() {
  const t = useT();
  const [text, setText] = useState("");
  const [submitted, setSubmitted] = useState("");

  const sources = useQuery({
    queryKey: ["discovery", "sources"],
    queryFn: api.listDiscoverySources,
  });

  const search = useQuery({
    queryKey: ["discovery", "search", submitted],
    queryFn: () => api.discoverySearch(submitted),
    // Pas de recherche à la frappe : chaque requête part interroger des
    // serveurs tiers, et en déclencher une par caractère saisi serait
    // discourtois envers eux autant qu'inutile ici.
    enabled: submitted.trim().length >= 2,
  });

  const noSources = sources.data?.items.length === 0;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-meta text-muted">{t("discovery.intro")}</p>

      <form
        onSubmit={(event) => {
          event.preventDefault();
          setSubmitted(text);
        }}
        className="flex gap-2"
      >
        <Input
          value={text}
          onChange={(event) => setText(event.target.value)}
          placeholder={t("discovery.placeholder")}
          aria-label={t("discovery.placeholder")}
          disabled={noSources}
        />
        <Button
          type="submit"
          disabled={noSources || text.trim().length < 2}
          // Le bouton garde sa largeur pendant que le champ prend le reste.
          className="shrink-0"
        >
          {t("discovery.tab.search")}
        </Button>
      </form>

      {noSources && (
        <EmptyState title={t("discovery.noSources")} description={t("discovery.noSourcesHint")} />
      )}

      {search.isFetching && (
        <div className="flex items-center gap-2 text-meta text-muted">
          <Spinner className="size-4" />
          {t("discovery.searching")}
        </div>
      )}

      {search.data && <SourceStatuses statuses={search.data.sources} />}

      {search.data && !search.isFetching && search.data.results.length === 0 && (
        <EmptyState
          title={
            search.data.sources.some((status) => status.error)
              ? t("discovery.noResultsPartial")
              : t("discovery.noResults")
          }
        />
      )}

      {search.data && search.data.results.length > 0 && (
        <ul className="flex flex-col gap-2">
          {search.data.results.map((result, index) => (
            <ResultRow key={`${result.sourceId}-${index}`} result={result} />
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * L'état de chaque catalogue, affiché avec les résultats et non à la place.
 *
 * C'est la moitié de la réponse : elle dit si la liste est complète.
 */
function SourceStatuses({ statuses }: { statuses: api.DiscoverySourceStatus[] }) {
  const t = useT();
  if (statuses.length === 0) return null;

  const failing = statuses.some((status) => status.error);

  return (
    <div className="flex flex-col gap-1.5">
      <ul className="flex flex-wrap gap-1.5">
        {statuses.map((status) => (
          <li
            key={status.sourceId}
            className={cx(
              "flex items-center gap-1.5 rounded-md border px-2 py-1 text-meta",
              status.error
                ? "border-danger/40 bg-danger/10 text-danger"
                : "border-border text-muted",
            )}
          >
            <span className="font-medium">{status.name}</span>
            <span aria-hidden="true">·</span>
            <span>
              {status.error
                ? t(STATUS_KEYS[status.error] ?? "discovery.status.unreachable")
                : t("discovery.status.ok", {
                    count: status.count,
                    ms: status.elapsedMs,
                  })}
            </span>
          </li>
        ))}
      </ul>
      {failing && <p className="text-meta text-danger">{t("discovery.partial")}</p>}
    </div>
  );
}

function ResultRow({ result }: { result: api.DiscoveryResult }) {
  const t = useT();
  const [importing, setImporting] = useState(false);
  const download = result.acquisitions?.[0]?.href;

  return (
    <li className="flex gap-3 rounded-lg border border-border p-3">
      {/*
        Image distante servie par le catalogue d'origine : boxincloud ne
        réhéberge rien de ce qu'il n'a pas. `referrerPolicy` évite d'annoncer à
        un service tiers l'adresse de l'instance qui le consulte.
      */}
      {result.coverUrl ? (
        <img
          src={result.coverUrl}
          alt=""
          referrerPolicy="no-referrer"
          loading="lazy"
          className="h-24 w-16 shrink-0 rounded object-cover"
        />
      ) : (
        <div className="h-24 w-16 shrink-0 rounded bg-surface-hover" />
      )}

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="truncate text-body font-medium text-fg">{result.title}</h3>
          {result.inLibrary && <Badge tone="accent">{t("discovery.inLibrary")}</Badge>}
        </div>

        {result.authors && result.authors.length > 0 && (
          <p className="truncate text-meta text-muted">{result.authors.join(", ")}</p>
        )}

        <p className="text-meta text-subtle">
          {t("discovery.from", { source: result.sourceName })}
          {result.published ? ` · ${result.published}` : ""}
          {result.language ? ` · ${result.language}` : ""}
        </p>

        {result.summary && (
          <p className="mt-1 line-clamp-2 text-meta text-muted">{result.summary}</p>
        )}

        <div className="mt-2 flex gap-2">
          {download && (
            <a
              href={download}
              target="_blank"
              rel="noreferrer noopener"
              className="pressable rounded border border-border px-2 py-1 text-meta text-fg hover:bg-surface-hover"
            >
              {t("discovery.download")}
            </a>
          )}
          {download && !importing && (
            <button
              type="button"
              onClick={() => setImporting(true)}
              className="pressable rounded border border-border px-2 py-1 text-meta text-fg hover:bg-surface-hover"
            >
              {t("discovery.import")}
            </button>
          )}
          {result.pageUrl && (
            <a
              href={result.pageUrl}
              target="_blank"
              rel="noreferrer noopener"
              className="pressable rounded px-2 py-1 text-meta text-muted hover:bg-surface-hover hover:text-fg"
            >
              {t("discovery.openPage")}
            </a>
          )}
        </div>

        {download && importing && (
          <ImportForm
            result={result}
            href={download}
            onDone={() => setImporting(false)}
          />
        )}
      </div>
    </li>
  );
}

/**
 * Rapatrier un résultat.
 *
 * Le formulaire tient dans la ligne du résultat plutôt que dans une boîte de
 * dialogue : c'est une action sur CETTE entrée, et l'ouvrir ailleurs obligerait
 * à répéter de quoi on parle.
 */
function ImportForm({
  result,
  href,
  onDone,
}: {
  result: api.DiscoveryResult;
  href: string;
  onDone: () => void;
}) {
  const t = useT();
  const [library, setLibrary] = useState("");
  const [folder, setFolder] = useState("");

  const libraries = useQuery({
    queryKey: ["libraries"],
    queryFn: api.listLibraries,
  });

  // La bibliothèque sélectionnée doit toujours exister : sans ce recalage, une
  // bibliothèque supprimée resterait choisie en coulisse pendant que la liste
  // en affiche une autre, et l'import partirait vers un identifiant mort.
  // Mémoïsé : `?? []` fabriquerait un tableau neuf à chaque rendu, et
  // relancerait l'effet en boucle.
  const items = useMemo(() => libraries.data?.libraries ?? [], [libraries.data]);
  useEffect(() => {
    const first = items[0];
    if (first && !items.some((entry) => entry.id === library)) {
      setLibrary(first.id);
    }
  }, [items, library]);

  const run = useMutation({
    mutationFn: () =>
      api.discoveryImport({
        sourceId: result.sourceId,
        href,
        libraryId: library,
        folder: folder || undefined,
        title: result.title,
      }),
  });

  if (libraries.data && items.length === 0) {
    return <p className="mt-2 text-meta text-danger">{t("discovery.import.noLibrary")}</p>;
  }

  // La demande a été acceptée : ce n'est plus un formulaire, c'est un suivi.
  if (run.isSuccess) {
    return <ImportProgress importId={run.data.id} />;
  }

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        run.mutate();
      }}
      className="mt-2 flex flex-col gap-2 rounded-md border border-border p-2"
    >
      <p className="text-meta text-subtle">{t("discovery.import.explain")}</p>

      <div className="flex flex-wrap gap-2">
        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.import.library")}
          <select
            value={library}
            onChange={(event) => setLibrary(event.target.value)}
            className="rounded border border-border bg-surface px-2 py-1 text-ui text-fg"
          >
            {items.map((entry) => (
              <option key={entry.id} value={entry.id}>
                {entry.name}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-1 flex-col gap-1 text-meta text-muted">
          {t("discovery.import.folder")}
          <Input value={folder} onChange={(event) => setFolder(event.target.value)} />
        </label>
      </div>

      {run.error && <p className="text-meta text-danger">{describeError(run.error, t)}</p>}

      <div className="flex gap-2">
        <Button type="submit" disabled={run.isPending || !library}>
          {run.isPending ? t("discovery.import.running") : t("discovery.import")}
        </Button>
        <Button type="button" variant="ghost" onClick={onDone} disabled={run.isPending}>
          {t("action.cancel")}
        </Button>
      </div>
    </form>
  );
}

// ─── Catalogues ──────────────────────────────────────────────────────────────

/**
 * Suivi d'un import lancé.
 *
 * L'import est une tâche de fond : la réponse ne dit plus s'il a abouti, elle
 * dit qu'il est parti. Sans ce suivi, le passage en arrière-plan reviendrait à
 * faire disparaître l'action — l'utilisateur cliquerait sans jamais savoir.
 */
function ImportProgress({ importId }: { importId: string }) {
  const t = useT();

  const imports = useQuery({
    queryKey: ["discovery", "imports"],
    queryFn: () => api.listDiscoveryImports(),
    // On n'interroge que tant que quelque chose bouge. Un intervalle qui
    // continue après la fin ferait battre une requête toutes les deux secondes
    // pendant que la fenêtre reste ouverte, pour rien.
    refetchInterval: (query) => {
      const mine = query.state.data?.items.find((entry) => entry.id === importId);
      return mine && (mine.status === "queued" || mine.status === "running")
        ? 2000
        : false;
    },
  });

  const record = imports.data?.items.find((entry) => entry.id === importId);

  if (!record || record.status === "queued" || record.status === "running") {
    return (
      <div className="mt-2 flex flex-col gap-1">
        <p className="flex items-center gap-2 text-meta text-muted">
          <Spinner className="size-4" />
          {record?.status === "running"
            ? t("discovery.import.running")
            : t("discovery.import.queued")}
        </p>
        <p className="text-meta text-subtle">{t("discovery.import.background")}</p>
      </div>
    );
  }

  if (record.status === "failed") {
    return (
      <p className="mt-2 text-meta text-danger">
        {t("discovery.import.failed")}
        {" — "}
        {t(importErrorKey(record.errorCode))}
        {record.errorDetail && (
          <code className="ml-2 font-mono text-subtle">{record.errorDetail}</code>
        )}
      </p>
    );
  }

  return (
    <p className="mt-2 text-meta text-muted">
      {t("discovery.import.done")}
      {record.objectKey ? ` — ${record.objectKey}` : ""}
    </p>
  );
}

/**
 * Traduit le code d'échec rendu par le serveur.
 *
 * Un code inconnu — un serveur plus récent que l'interface qu'il sert — retombe
 * sur « le catalogue n'a pas répondu » plutôt que d'afficher le code brut, qui
 * ne renseignerait personne.
 */
function importErrorKey(code: string | undefined): MessageKey {
  const key = `discovery.import.err.${code ?? ""}`;
  return (IMPORT_ERROR_KEYS.has(key) ? key : "discovery.import.err.unreachable") as MessageKey;
}

const IMPORT_ERROR_KEYS = new Set([
  "discovery.import.err.unreachable",
  "discovery.import.err.timeout",
  "discovery.import.err.foreign-host",
  "discovery.import.err.invalid",
  "discovery.import.err.source-gone",
  "discovery.import.err.queue",
  "discovery.import.err.unsupported-format",
  "discovery.import.err.content-mismatch",
  "discovery.import.err.exists",
  "discovery.import.err.too-large",
  "discovery.import.err.deposit-failed",
]);

function SourcesSection() {
  const t = useT();
  const client = useQueryClient();
  const [adding, setAdding] = useState(false);

  const sources = useQuery({
    queryKey: ["discovery", "sources"],
    queryFn: api.listDiscoverySources,
  });

  const invalidate = () => {
    void client.invalidateQueries({ queryKey: ["discovery"] });
  };

  return (
    <div className="flex flex-col gap-4">
      <p className="text-meta text-muted">{t("discovery.sources.intro")}</p>

      {sources.isLoading && <Spinner className="size-5" />}

      {sources.data && sources.data.items.length === 0 && !adding && (
        <EmptyState title={t("discovery.sources.empty")} />
      )}

      {sources.data && sources.data.items.length > 0 && (
        <ul className="flex flex-col gap-2">
          {sources.data.items.map((source) => (
            <SourceRow key={source.id} source={source} onChanged={invalidate} />
          ))}
        </ul>
      )}

      {adding ? (
        <SourceForm
          onDone={() => {
            setAdding(false);
            invalidate();
          }}
          onCancel={() => setAdding(false)}
        />
      ) : (
        <Button onClick={() => setAdding(true)} className="self-start">
          {t("discovery.sources.add")}
        </Button>
      )}
    </div>
  );
}

function SourceRow({
  source,
  onChanged,
}: {
  source: api.DiscoverySource;
  onChanged: () => void;
}) {
  const t = useT();
  const [verdict, setVerdict] = useState<{ ok: boolean; detail?: string } | null>(null);

  const test = useMutation({
    mutationFn: () => api.testDiscoverySource(source.id),
    onSuccess: (result) => {
      setVerdict(result);
      onChanged();
    },
  });

  const remove = useMutation({
    mutationFn: () => api.deleteDiscoverySource(source.id),
    onSuccess: onChanged,
  });

  return (
    <li className="flex flex-col gap-1 rounded-lg border border-border p-3">
      <div className="flex items-center gap-2">
        <span className="font-medium text-fg">{source.name}</span>
        {!source.enabled && <Badge>{t("discovery.sources.enabled")}</Badge>}
        <span className="ml-auto flex gap-1">
          <Button
            variant="ghost"
            onClick={() => test.mutate()}
            disabled={test.isPending}
          >
            {test.isPending ? t("discovery.sources.checking") : t("discovery.sources.test")}
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              if (confirm(t("discovery.sources.confirmDelete"))) remove.mutate();
            }}
          >
            {t("action.delete")}
          </Button>
        </span>
      </div>

      <p className="truncate text-meta text-subtle">{source.url}</p>

      {/*
        Le détail vient du catalogue distant, en anglais le plus souvent. Il est
        présenté comme un diagnostic technique sous un titre traduit, jamais
        comme une phrase à lire — aucun catalogue de traductions ne peut couvrir
        ce qu'un serveur inconnu répondra.
      */}
      {verdict && (
        <p className={cx("text-meta", verdict.ok ? "text-muted" : "text-danger")}>
          {verdict.ok ? t("discovery.sources.testOk") : t("discovery.sources.testFailed")}
          {verdict.detail && (
            <code className="ml-2 font-mono text-subtle">{verdict.detail}</code>
          )}
        </p>
      )}

      {!verdict && source.lastError && (
        <p className="text-meta text-danger">
          {t("discovery.sources.lastError")}
          <code className="ml-2 font-mono text-subtle">{source.lastError}</code>
        </p>
      )}
    </li>
  );
}

function SourceForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const t = useT();
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.createDiscoverySource({
        name,
        url,
        username: username || undefined,
        password: password || undefined,
      }),
    onSuccess: onDone,
  });

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        create.mutate();
      }}
      className="flex flex-col gap-3 rounded-lg border border-border p-3"
    >
      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.name")}
        <Input value={name} onChange={(event) => setName(event.target.value)} required />
      </label>

      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.url")}
        <Input
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          placeholder="https://…/opds"
          required
        />
        <span className="text-subtle">{t("discovery.sources.urlHint")}</span>
      </label>

      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.username")}
        <Input value={username} onChange={(event) => setUsername(event.target.value)} />
        <span className="text-subtle">{t("discovery.sources.usernameHint")}</span>
      </label>

      {username && (
        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.password")}
          <Input
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
      )}

      {create.error && (
        <p className="text-meta text-danger">{describeError(create.error, t)}</p>
      )}

      <div className="flex gap-2">
        <Button type="submit" disabled={create.isPending}>
          {create.isPending ? t("discovery.sources.checking") : t("action.save")}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          {t("action.cancel")}
        </Button>
      </div>
    </form>
  );
}
