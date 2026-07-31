"use client";

import { useEffect, useState } from "react";
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

export function DiscoveryPanel({
  canAdmin = false,
  onClose,
}: {
  /*
    Déclarer un catalogue est une décision d'administration : l'adresse est
    jointe par le serveur. L'onglet est donc masqué aux autres comptes — le
    serveur refuserait de toute façon, et proposer une porte fermée n'aide
    personne.
  */
  canAdmin?: boolean;
  onClose: () => void;
}) {
  const t = useT();
  const [tab, setTab] = useState<"search" | "sources">("search");

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
        aria-label={t("discovery.dialogLabel")}
        className="rise-in flex h-[80vh] w-full max-w-4xl flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-fg">{t("discovery.title")}</h2>

          <div className="ml-2 flex items-center gap-0.5 rounded-md border border-border p-0.5">
            {(canAdmin ? (["search", "sources"] as const) : (["search"] as const)).map((option) => (
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
                {t(option === "search" ? "discovery.tab.search" : "discovery.tab.sources")}
              </button>
            ))}
          </div>

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
          {tab === "sources" && canAdmin ? <SourcesSection /> : <SearchSection />}
        </div>
      </div>
    </div>
  );
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
          className="flex-1"
        />
        <Button type="submit" disabled={noSources || text.trim().length < 2}>
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
      </div>
    </li>
  );
}

// ─── Catalogues ──────────────────────────────────────────────────────────────

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
