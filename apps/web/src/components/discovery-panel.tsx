"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Badge, Button, EmptyState, Input, Spinner, cx } from "./ui";
import * as api from "@/lib/api/endpoints";
import { describeError, rawDetail } from "@/lib/api/problem";
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
    <div className="fixed inset-0 z-[60] flex items-end bg-[var(--overlay)]">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("discovery.dialogLabel")}
        /*
          Pleine largeur, sans arrondi ni bordure latérale : la feuille est
          ancrée au bord inférieur de la fenêtre, pas posée dessus. Une carte
          centrée aux angles arrondis se lit comme une boîte de dialogue — ce
          qu'elle n'est pas — et gaspille la largeur dont une liste de
          résultats a besoin.
        */
        className="slide-in-bottom flex max-h-[85vh] w-full flex-col overflow-hidden border-t border-border bg-surface shadow-2xl"
      >
        {/*
          L'en-tête suit la même colonne que le contenu : un titre collé au
          bord gauche de l'écran et une croix collée au bord droit, avec le
          formulaire centré entre les deux, se liraient comme trois éléments
          sans rapport.
        */}
        <header className="border-b border-border px-4 py-3">
          <div className="mx-auto flex w-full max-w-5xl items-center gap-3">
            <h2 className="text-title font-semibold text-fg">{t("discovery.title")}</h2>

            <span className="text-meta text-subtle">⌘⇧F</span>

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
          </div>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          {/*
            La feuille prend toute la largeur, son CONTENU non : une ligne de
            texte qui traverse un écran large ne se lit pas — l'œil perd le
            début de la ligne suivante. La colonne est centrée et bornée.
          */}
          <div className="mx-auto w-full max-w-5xl">
            <SearchSection />
          </div>
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

      {/*
        Une source lue au gabarit n'a pas forcément d'adresse : ses miroirs
        viennent du gabarit. Laisser la ligne vide donnerait l'impression d'une
        configuration incomplète alors que c'est le cas normal.
      */}
      <p className="truncate text-meta text-subtle">
        {source.url || t("discovery.sources.templateMirrors")}
      </p>

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

/*
Formulaire d'ajout d'un catalogue.

Le choix du genre n'apparaît QUE si l'instance a chargé des gabarits — et elle
n'en livre aucun par défaut. Montrer un menu déroulant à une seule entrée
demanderait à l'administrateur de choisir là où il n'y a rien à choisir, et
ferait passer OPDS pour une option parmi d'autres alors qu'il est le cas normal.
*/
function SourceForm({ onDone, onCancel }: { onDone: () => void; onCancel: () => void }) {
  const t = useT();
  const [name, setName] = useState("");
  // Vide = OPDS. Le défaut est côté serveur ; on ne l'envoie pas.
  const [kind, setKind] = useState("");
  const [url, setUrl] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  // Les règles d'un site décrit ici. Inertes tant que « Site web » n'est pas
  // choisi ; le serveur refuse d'ailleurs des règles sur un autre genre.
  const [searchUrl, setSearchUrl] = useState("");
  const [row, setRow] = useState("");
  const [selTitle, setSelTitle] = useState("");
  const [selAuthor, setSelAuthor] = useState("");
  const [selCover, setSelCover] = useState("");
  const [selLink, setSelLink] = useState("");
  const [ignoreRobots, setIgnoreRobots] = useState(false);
  const [format, setFormat] = useState<"html" | "json">("html");

  const templates = useQuery({
    queryKey: ["discovery", "scraper-templates"],
    queryFn: api.listScraperTemplates,
  });

  const chosen = templates.data?.items.find((item) => item.kind === kind);
  const isWeb = kind === "web";

  const create = useMutation({
    mutationFn: () =>
      api.createDiscoverySource({
        name,
        kind: kind || undefined,
        // Sur un gabarit, une adresse vide fait retomber sur ses miroirs. La
        // transmettre vide plutôt que de l'omettre ne changerait rien côté
        // serveur, mais l'omission dit mieux ce qu'on veut.
        url: url || undefined,
        // Les champs vides sont omis : le serveur ne distingue pas « absent »
        // de « vide », et lui envoyer des sélecteurs blancs le ferait chercher
        // ce que personne n'a désigné.
        template: isWeb
          ? {
              searchUrl,
              row,
              title: selTitle,
              author: selAuthor || undefined,
              cover: selCover || undefined,
              link: selLink || undefined,
              ignoreRobots: ignoreRobots || undefined,
            }
          : undefined,
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

      {/*
        Le choix du type est TOUJOURS affiché, même quand aucun gabarit n'est
        chargé — et c'est le contraire de ce que faisait la première version.

        Elle ne le montrait que si l'instance avait des gabarits, pour éviter un
        menu à une seule entrée. L'intention était bonne, le résultat mauvais :
        aucun gabarit n'étant livré par défaut, le menu n'apparaissait jamais,
        et un administrateur ne pouvait pas deviner que le moteur existait. Une
        fonctionnalité qu'on ne peut pas découvrir n'existe pas.

        Un menu à une entrée assorti d'une explication vaut mieux que rien du
        tout : il dit ce qui est possible, et pourquoi ça ne l'est pas encore.
      */}
      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.kind")}
        <select
          value={kind}
          onChange={(event) => setKind(event.target.value)}
          className="h-9 rounded-md border border-border bg-surface px-2 text-ui text-fg"
        >
          <option value="">{t("discovery.sources.kindOpds")}</option>
          {/*
            « Site web » est proposé en dur, et toujours. Il ne dépend d'aucun
            gabarit chargé — le serveur l'enregistre systématiquement — et c'est
            lui qui rend le moteur d'extraction atteignable sans toucher au
            disque de l'instance.
          */}
          <option value="web">{t("discovery.sources.kindWeb")}</option>
          {(templates.data?.items ?? []).map((template) => (
            <option key={template.kind} value={template.kind}>
              {template.name}
            </option>
          ))}
        </select>

        {templates.data?.items.length === 0 && (
          <span className="text-subtle">{t("discovery.sources.noTemplates")}</span>
        )}
        {chosen?.license && <span className="text-subtle">{chosen.license}</span>}
      </label>

      {/*
        Le champ d'adresse OPDS disparaît pour un site web : ce n'est pas un flux
        qu'on désigne, c'est une recherche. Le laisser afficherait deux champs
        d'adresse dont un seul compte.
      */}
      {!isWeb && (
        <label className="flex flex-col gap-1 text-meta text-muted">
          {chosen ? t("discovery.sources.mirror") : t("discovery.sources.url")}
          <Input
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            placeholder={chosen ? (chosen.mirrors?.[0] ?? "https://…") : "https://…/opds"}
            // Un gabarit déclare déjà ses miroirs : l'adresse ne se saisit que le
            // jour où l'un d'eux change.
            required={!chosen}
          />
          <span className="text-subtle">
            {chosen ? t("discovery.sources.mirrorHint") : t("discovery.sources.urlHint")}
          </span>
        </label>
      )}

      {isWeb && <WebFields
        searchUrl={searchUrl} setSearchUrl={setSearchUrl}
        row={row} setRow={setRow}
        title={selTitle} setTitle={setSelTitle}
        author={selAuthor} setAuthor={setSelAuthor}
        cover={selCover} setCover={setSelCover}
        link={selLink} setLink={setSelLink}
        ignoreRobots={ignoreRobots} setIgnoreRobots={setIgnoreRobots}
        format={format} setFormat={setFormat}
      />}

      {/*
        Ni identifiant ni mot de passe pour un site web : on lit des pages
        publiques, il n'y a pas de session à ouvrir. Les afficher suggérerait
        une authentification que le moteur ne sait pas faire.
      */}
      {!isWeb && (
        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.username")}
          <Input value={username} onChange={(event) => setUsername(event.target.value)} />
          <span className="text-subtle">{t("discovery.sources.usernameHint")}</span>
        </label>
      )}

      {!isWeb && username && (
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
        <p className="text-meta text-danger">
          {describeError(create.error, t)}
          {/*
            Le diagnostic du serveur, quand il en dit plus que la règle. En
            police à chasse fixe et sous le message traduit : c'est une trace
            technique, pas une phrase — elle cite souvent le site distant, en
            anglais.
          */}
          {rawDetail(create.error) && (
            <code className="mt-1 block font-mono text-subtle">
              {rawDetail(create.error)}
            </code>
          )}
        </p>
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

/*
WebFields — décrire où lire les résultats d'un site.

Six champs, pas trente. Le moteur en accepte davantage dans un gabarit sur
disque — miroirs, expressions rationnelles, suivi de fiche — et rien de tout
cela n'est ici : un formulaire long ne serait rempli correctement par personne,
et chaque possibilité offerte est une possibilité de se tromper sans que rien ne
le signale.

L'ordre suit celui dans lequel on s'y prend réellement. On cherche d'abord sur
le site pour obtenir une adresse ; on repère ensuite le bloc qui se répète ; on
désigne enfin ce qu'il contient. Demander les sélecteurs avant l'adresse
obligerait à faire les choses dans le désordre.
*/
function WebFields(props: {
  searchUrl: string;
  setSearchUrl: (v: string) => void;
  row: string;
  setRow: (v: string) => void;
  title: string;
  setTitle: (v: string) => void;
  author: string;
  setAuthor: (v: string) => void;
  cover: string;
  setCover: (v: string) => void;
  link: string;
  setLink: (v: string) => void;
  ignoreRobots: boolean;
  setIgnoreRobots: (v: boolean) => void;
  format: "html" | "json";
  setFormat: (v: "html" | "json") => void;
}) {
  const t = useT();
  const json = props.format === "json";

  return (
    <div className="flex flex-col gap-3 rounded-md border border-border bg-surface-hover/40 p-3">
      <p className="text-meta text-muted">
        {json ? t("discovery.sources.jsonIntro") : t("discovery.sources.webIntro")}
      </p>

      {/*
        Le format d'abord : il change la NATURE des champs suivants — sélecteurs
        CSS ou chemins — et le demander après les avoir fait remplir obligerait
        à tout reprendre.
      */}
      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.format")}
        <select
          value={props.format}
          onChange={(event) => props.setFormat(event.target.value as "html" | "json")}
          className="h-9 rounded-md border border-border bg-surface px-2 text-ui text-fg"
        >
          <option value="html">{t("discovery.sources.formatHtml")}</option>
          <option value="json">{t("discovery.sources.formatJson")}</option>
        </select>
      </label>

      <label className="flex flex-col gap-1 text-meta text-muted">
        {t("discovery.sources.searchUrl")}
        <Input
          value={props.searchUrl}
          onChange={(event) => props.setSearchUrl(event.target.value)}
          placeholder={
            json
              ? "https://exemple.org/api/search?q={terms}&output=json"
              : "https://exemple.org/recherche?q={terms}"
          }
          required
        />
        <span className="text-subtle">{t("discovery.sources.searchUrlHint")}</span>
      </label>

      <label className="flex flex-col gap-1 text-meta text-muted">
        {json ? t("discovery.sources.rowJson") : t("discovery.sources.row")}
        <Input
          value={props.row}
          onChange={(event) => props.setRow(event.target.value)}
          placeholder={json ? "response.docs" : "ul.results > li"}
          // En JSON, une racine tableau est légitime : certaines API rendent
          // directement la liste, sans objet enveloppant.
          required={!json}
        />
        <span className="text-subtle">
          {json ? t("discovery.sources.rowJsonHint") : t("discovery.sources.rowHint")}
        </span>
      </label>

      {/*
        Les quatre sélecteurs de champ tiennent sur deux colonnes : ils sont
        courts, de même nature, et les empiler sur quatre lignes ferait paraître
        le formulaire deux fois plus long qu'il ne l'est.
      */}
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.selTitle")}
          <Input
            value={props.title}
            onChange={(event) => props.setTitle(event.target.value)}
            placeholder={json ? "title" : "h3 a"}
            required
          />
        </label>

        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.selLink")}
          <Input
            value={props.link}
            onChange={(event) => props.setLink(event.target.value)}
            placeholder={json ? "formats.application/epub+zip" : "a.download"}
          />
        </label>

        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.selAuthor")}
          <Input
            value={props.author}
            onChange={(event) => props.setAuthor(event.target.value)}
            placeholder={json ? "authors.#.name" : "span.author"}
          />
        </label>

        <label className="flex flex-col gap-1 text-meta text-muted">
          {t("discovery.sources.selCover")}
          <Input
            value={props.cover}
            onChange={(event) => props.setCover(event.target.value)}
            placeholder={json ? "cover_url" : "img"}
          />
        </label>
      </div>

      <span className="text-meta text-subtle">{t("discovery.sources.webProbeHint")}</span>

      {/*
        La dérogation est en bas, décochée, et assortie de sa conséquence.
        La mettre en évidence inviterait à la cocher « au cas où » ; l'omettre
        laisserait sans issue un administrateur qui a autorité sur le site.
      */}
      <label className="flex items-start gap-2 text-meta text-muted">
        <input
          type="checkbox"
          checked={props.ignoreRobots}
          onChange={(event) => props.setIgnoreRobots(event.target.checked)}
          className="mt-0.5"
        />
        <span>
          {t("discovery.sources.ignoreRobots")}
          <span className="block text-subtle">
            {t("discovery.sources.ignoreRobotsHint")}
          </span>
        </span>
      </label>
    </div>
  );
}
