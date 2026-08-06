"use client";

/**
 * Recherche sur les réseaux eD2k et Kad.
 *
 * # Une seule recherche à la fois, et il faut le dire
 *
 * Le démon n'en tient qu'une : en démarrer une seconde efface les résultats de
 * la première. Ce n'est pas un choix qu'on pourrait assouplir côté interface,
 * et deux personnes qui cherchent en même temps se marcheront dessus.
 *
 * L'écran le dit plutôt que de le laisser découvrir — quelqu'un dont les
 * résultats disparaissent sans explication conclut à un bogue.
 *
 * # Sondage tant que la recherche tourne, et pas après
 *
 * Le protocole ne notifie rien : la progression se demande. Le sondage s'arrête
 * dès qu'elle atteint son terme, sinon une recherche finie continuerait
 * d'interroger le démon toutes les secondes pendant que l'onglet reste ouvert.
 */

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Badge, Button, Input } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";
import type { Ed2kSearchNetwork } from "@/lib/api/endpoints";

import { ActionButton, CommandError, useCommand } from "./commands";
import { DASH, useEd2kFormat } from "./format";
import { Async, PanelHeader } from "./panel";
import { DataTable, Num, Row, type Column } from "./table";

const COLUMNS: Column[] = [
  { key: "name", label: "ed2k.col.name", width: "minmax(280px, 4fr)" },
  { key: "size", label: "ed2k.col.size", width: "96px", align: "right" },
  { key: "complete", label: "ed2k.col.completeSources", width: "112px", align: "right" },
  { key: "sources", label: "ed2k.col.sources", width: "96px", align: "right" },
  { key: "actions", label: "ed2k.col.actions", width: "120px" },
];

const NETWORKS: Ed2kSearchNetwork[] = ["global", "server", "kad"];

export function SearchPanel() {
  const t = useT();
  const command = useCommand();

  const [query, setQuery] = useState("");
  const [network, setNetwork] = useState<Ed2kSearchNetwork>("global");
  const [started, setStarted] = useState(false);

  /*
    Le sondage s'arrête à la fin de la recherche.

    `started` distingue « pas encore cherché » de « recherche terminée sans
    résultat » : le démon rend la même chose dans les deux cas, et sans ce
    drapeau l'écran d'accueil afficherait « aucun résultat » avant même qu'on
    ait tapé quoi que ce soit.
  */
  const results = useQuery({
    queryKey: ["ed2k", "search"],
    queryFn: api.getEd2kSearchResults,
    enabled: started,
    refetchInterval: (query) => (query.state.data?.complete === false ? 1000 : false),
  });

  // Quitter l'écran arrête la recherche : la laisser tourner consommerait la
  // bande passante du démon pour des résultats que personne ne lit.
  useEffect(() => {
    return () => {
      if (started) void api.stopEd2kSearch().catch(() => {});
    };
  }, [started]);

  return (
    <section>
      <PanelHeader title={t("ed2k.section.search")} hint={t("ed2k.search.hint")} />

      <form
        onSubmit={(event) => {
          event.preventDefault();
          void command
            .run(() => api.startEd2kSearch({ query, network }))
            .then(() => setStarted(true));
        }}
        className="mb-3 flex flex-wrap items-end gap-2 rounded-lg border border-border bg-surface p-3"
      >
        <div className="min-w-[220px] flex-1">
          <Input
            name="query"
            label={t("ed2k.search.query")}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            autoComplete="off"
          />
        </div>

        <fieldset className="flex gap-1">
          <legend className="sr-only">{t("ed2k.search.network")}</legend>
          {NETWORKS.map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => setNetwork(option)}
              aria-pressed={network === option}
              className={
                network === option
                  ? "pressable rounded-md bg-accent px-2.5 py-1.5 text-ui text-inverted"
                  : "pressable rounded-md border border-border px-2.5 py-1.5 text-ui text-muted hover:bg-surface-hover hover:text-fg"
              }
            >
              {t(`ed2k.search.net.${option}` as never)}
            </button>
          ))}
        </fieldset>

        <Button type="submit" size="md" loading={command.busy} disabled={query.trim() === ""}>
          {t("ed2k.search.submit")}
        </Button>
      </form>

      <CommandError error={command.error} onDismiss={command.clearError} />

      {/*
        L'avertissement est permanent, pas contextuel : il ne sert à rien de
        prévenir APRÈS que les résultats d'un collègue ont disparu.
      */}
      <p className="mb-3 max-w-prose text-micro text-subtle">{t("ed2k.search.shared")}</p>

      {started && results.data && !results.data.complete && (
        <Progress percent={results.data.progress} />
      )}

      {started ? (
        <Async
          query={results}
          isEmpty={(data) => data.results.length === 0 && data.complete}
          empty={{ title: t("ed2k.search.empty"), description: t("ed2k.search.emptyHint") }}
        >
          {(data) => (
            <DataTable
              items={data.results}
              columns={COLUMNS}
              minWidth={860}
              label={t("ed2k.section.search")}
              filterHint={t("ed2k.table.filterFiles")}
              // Affiner SANS relancer : le démon ne tient qu'une recherche à la
              // fois, et resserrer les mots-clés effacerait ce qui est déjà
              // remonté. Le filtre travaille sur ce qu'on a.
              searchText={(result) => result.name}
              renderRow={(result, index) => (
                <Row key={result.hash} striped={index % 2 === 1}>
                  <span className="min-w-0">
                    <span className="block truncate text-fg" title={result.hash}>
                      {result.name || DASH}
                    </span>
                  </span>

                  <SizeCell size={result.size} />

                  {/*
                    Les sources COMPLÈTES d'abord, et mises en valeur : ce sont
                    elles qui disent si un fichier finira. Cent sources
                    partielles qui n'ont jamais la même partie ne complètent
                    rien, et une colonne unique le masquerait.
                  */}
                  <Num tone={result.completeSources > 0 ? "fg" : "warning"}>
                    {result.completeSources}
                  </Num>
                  <Num tone="muted">{result.sources}</Num>

                  <span className="min-w-0">
                    {result.alreadyQueued ? (
                      <Badge tone="neutral">{t("ed2k.search.queued")}</Badge>
                    ) : (
                      <ActionButton
                        label={t("ed2k.search.download")}
                        disabled={command.busy}
                        onClick={() =>
                          void command.run(() => api.downloadEd2kSearchResult(result.hash))
                        }
                      />
                    )}
                  </span>
                </Row>
              )}
            />
          )}
        </Async>
      ) : (
        <p className="py-10 text-center text-meta text-subtle">{t("ed2k.search.idle")}</p>
      )}
    </section>
  );
}

function SizeCell({ size }: { size: number }) {
  const format = useEd2kFormat();
  return <Num>{format.bytes(size)}</Num>;
}

/**
 * Barre de progression de la recherche.
 *
 * Affichée seulement pendant : une barre à 100 % laissée à l'écran fait croire
 * qu'une recherche tourne encore.
 */
function Progress({ percent }: { percent: number }) {
  const t = useT();

  return (
    <div className="mb-3 flex items-center gap-2">
      <div className="h-1 flex-1 overflow-hidden rounded-full bg-surface-sunken">
        <div
          className="h-full bg-accent transition-[width] duration-(--motion-duration-normal)"
          style={{ width: `${Math.min(Math.max(percent, 0), 100)}%` }}
        />
      </div>
      <span className="shrink-0 text-micro tabular-nums text-subtle">
        {t("ed2k.search.searching", { percent })}
      </span>
    </div>
  );
}

