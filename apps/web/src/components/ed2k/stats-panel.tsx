"use client";

/**
 * Statistiques et état des deux réseaux.
 *
 * Pas de graphes ici, et c'est délibéré : le serveur ne conserve aucune série
 * temporelle à cette étape, si bien qu'une courbe ne pourrait tracer que ce que
 * le navigateur a vu depuis qu'on regarde. Elle disparaîtrait au premier
 * rechargement, ce qui est pire que pas de courbe — on croit lire un historique
 * et on lit une session.
 *
 * Ce que cet écran montre à la place, aucun autre ne le montre : les deux
 * réseaux côte à côte, avec leur état de joignabilité. C'est là que se lit un
 * LowID ou un Kad derrière pare-feu, c'est-à-dire les deux raisons pour
 * lesquelles un client trouve peu de sources sans que rien ne semble cassé.
 */

import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";
import type { Ed2kConnection } from "@/lib/api/client";

import { DASH, useEd2kFormat } from "./format";
import { ID_LABELS, ID_TONES } from "./labels";
import { CommandError, PanelAction, useCommand } from "./commands";
import { Async, PanelHeader, REFRESH_MS } from "./panel";

export function StatsPanel() {
  const t = useT();
  const format = useEd2kFormat();

  const command = useCommand();

  const query = useQuery({
    queryKey: ["ed2k", "stats"],
    queryFn: api.getEd2kStats,
    refetchInterval: REFRESH_MS,
  });

  return (
    <section>
      <PanelHeader
        title={t("ed2k.section.stats")}
        hint={t("ed2k.stats.hint")}
        takenAt={query.data?.takenAt}
      >
        {/*
          Le bouton suit l'état courant. Kad démarré mais pas encore connecté
          compte comme démarré : ce qu'on propose est de l'arrêter, pas de le
          redémarrer.
        */}
        {query.data && (
          <PanelAction
            label={
              query.data.connection.kad.running ? t("ed2k.kad.stop") : t("ed2k.kad.start")
            }
            busy={command.busy}
            onClick={() =>
              void command.run(() =>
                api.setEd2kKadRunning(!query.data.connection.kad.running),
              )
            }
          />
        )}
      </PanelHeader>

      <CommandError error={command.error} onDismiss={command.clearError} />

      <Async
        query={query}
        // Les statistiques existent toujours, même toutes à zéro : un démon au
        // repos est un état, pas un écran vide.
        isEmpty={() => false}
        empty={{ title: "", description: "" }}
      >
        {(data) => (
          <div className="flex flex-col gap-4">
            <Networks connection={data.connection} />

            <Group title={t("ed2k.stats.transfer")}>
              <Figure label={t("ed2k.stats.downSpeed")} value={format.speed(data.stats.downSpeed)} />
              <Figure label={t("ed2k.stats.upSpeed")} value={format.speed(data.stats.upSpeed)} />
              {/* Zéro veut dire « aucune limite », pas « débit nul » : le
                  confondre ferait croire à un plafond bloquant. */}
              <Figure
                label={t("ed2k.stats.downLimit")}
                value={
                  data.stats.downLimit > 0
                    ? format.speed(data.stats.downLimit)
                    : t("ed2k.stats.unlimited")
                }
              />
              <Figure
                label={t("ed2k.stats.upLimit")}
                value={
                  data.stats.upLimit > 0
                    ? format.speed(data.stats.upLimit)
                    : t("ed2k.stats.unlimited")
                }
              />
              <Figure
                label={t("ed2k.stats.downOverhead")}
                value={format.speed(data.stats.downOverhead)}
              />
              <Figure
                label={t("ed2k.stats.upOverhead")}
                value={format.speed(data.stats.upOverhead)}
              />
            </Group>

            <Group title={t("ed2k.stats.peers")}>
              <Figure
                label={t("ed2k.stats.totalSources")}
                value={format.count(data.stats.totalSources)}
              />
              <Figure
                label={t("ed2k.stats.uploadQueue")}
                value={format.count(data.stats.uploadQueueLength)}
              />
              <Figure
                label={t("ed2k.stats.banned")}
                value={format.count(data.stats.bannedPeers)}
              />
            </Group>

            <Group title={t("ed2k.stats.networks")}>
              <Figure label={t("ed2k.stats.ed2kUsers")} value={format.count(data.stats.ed2kUsers)} />
              <Figure label={t("ed2k.stats.ed2kFiles")} value={format.count(data.stats.ed2kFiles)} />
              <Figure label={t("ed2k.stats.kadUsers")} value={format.count(data.stats.kadUsers)} />
              <Figure label={t("ed2k.stats.kadFiles")} value={format.count(data.stats.kadFiles)} />
            </Group>
          </div>
        )}
      </Async>
    </section>
  );
}

/**
 * Les deux réseaux, côte à côte.
 *
 * Ils sont indépendants — on peut être sur Kad sans serveur, et l'inverse — et
 * les empiler laisserait croire qu'une panne de l'un est une panne du module.
 */
function Networks({ connection }: { connection: Ed2kConnection }) {
  const t = useT();

  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="rounded-lg border border-border bg-surface p-3">
        <h3 className="text-ui font-medium text-fg">{t("ed2k.net.ed2k")}</h3>

        <div className="mt-2 flex flex-wrap items-center gap-2">
          {connection.ed2k.connected ? (
            <Badge tone="success">{t("ed2k.net.connected")}</Badge>
          ) : connection.ed2k.connecting ? (
            <Badge tone="accent">{t("ed2k.net.connecting")}</Badge>
          ) : (
            <Badge tone="neutral">{t("ed2k.net.disconnected")}</Badge>
          )}

          <Badge tone={ID_TONES[connection.ed2k.id]}>{t(ID_LABELS[connection.ed2k.id])}</Badge>
        </div>

        <p className="mt-2 truncate text-meta text-muted">
          {connection.ed2k.server
            ? `${connection.ed2k.server.name || DASH} · ${connection.ed2k.server.ip}:${connection.ed2k.server.port}`
            : t("ed2k.net.noServer")}
        </p>

        {/* Le LowID est la première cause de « je trouve peu de sources » : on
            l'explique ici plutôt que de laisser un badge énigmatique. */}
        {connection.ed2k.id === "low" && (
          <p className="mt-1 max-w-prose text-micro text-warning">{t("ed2k.net.lowIdHint")}</p>
        )}
      </div>

      <div className="rounded-lg border border-border bg-surface p-3">
        <h3 className="text-ui font-medium text-fg">{t("ed2k.net.kad")}</h3>

        <div className="mt-2 flex flex-wrap items-center gap-2">
          {!connection.kad.running ? (
            <Badge tone="neutral">{t("ed2k.net.stopped")}</Badge>
          ) : connection.kad.connected ? (
            <Badge tone="success">{t("ed2k.net.connected")}</Badge>
          ) : (
            <Badge tone="accent">{t("ed2k.net.searching")}</Badge>
          )}

          {connection.kad.firewalled && (
            <Badge tone="warning">{t("ed2k.net.firewalled")}</Badge>
          )}
          {connection.kad.firewalledUdp && (
            <Badge tone="warning">{t("ed2k.net.firewalledUdp")}</Badge>
          )}
        </div>

        <p className="mt-2 max-w-prose text-meta text-muted">
          {connection.kad.running ? t("ed2k.net.kadHint") : t("ed2k.net.kadStoppedHint")}
        </p>
      </div>
    </div>
  );
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-surface p-3">
      <h3 className="text-micro uppercase tracking-wide text-subtle">{title}</h3>
      <dl className="mt-2 grid gap-x-4 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">{children}</dl>
    </div>
  );
}

/** Un chiffre et son libellé. Aligné en chiffres tabulaires : une colonne de
 *  nombres qui danse d'une ligne à l'autre se compare mal. */
function Figure({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="truncate text-meta text-muted">{label}</dt>
      <dd className="shrink-0 text-ui tabular-nums text-fg">{value}</dd>
    </div>
  );
}

