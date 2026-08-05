"use client";

/**
 * Serveurs eD2k.
 *
 * Le serveur JOINT est mis en évidence, et c'est la seule information que cet
 * écran apporte vraiment : on est relié à un seul serveur à la fois, et une
 * liste de trente entrées où rien ne le distingue oblige à aller chercher
 * ailleurs ce que le tableau pourrait dire.
 *
 * La colonne des échecs mérite sa place. Un serveur mort le reste, et c'est ce
 * compteur — pas le ping, qui n'est mesuré que sur les serveurs joints — qui
 * permet de repérer les entrées à retirer d'une liste importée il y a deux ans.
 */

import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";

import { DASH, useEd2kFormat } from "./format";
import { PRIORITY_LABELS } from "./labels";
import { ActionButton, CommandError, PanelAction, useCommand } from "./commands";
import { Async, PanelHeader, REFRESH_MS } from "./panel";
import { Num, Row, Table, Text, type Column } from "./table";

const COLUMNS: Column[] = [
  { key: "name", label: "ed2k.col.name", width: "minmax(200px, 3fr)" },
  { key: "address", label: "ed2k.col.address", width: "minmax(150px, 1fr)" },
  { key: "users", label: "ed2k.col.users", width: "96px", align: "right" },
  { key: "files", label: "ed2k.col.files", width: "104px", align: "right" },
  { key: "ping", label: "ed2k.col.ping", width: "84px", align: "right" },
  { key: "failed", label: "ed2k.col.failed", width: "84px", align: "right" },
  { key: "priority", label: "ed2k.col.priority", width: "104px" },
  { key: "actions", label: "ed2k.col.actions", width: "110px" },
];

export function ServersPanel() {
  const t = useT();
  const format = useEd2kFormat();

  const command = useCommand();

  const query = useQuery({
    queryKey: ["ed2k", "servers"],
    queryFn: api.listEd2kServers,
    // Une liste de serveurs ne bouge qu'à la connexion ou à l'import : la
    // scruter à la seconde ne montrerait rien de plus.
    refetchInterval: REFRESH_MS * 5,
  });

  return (
    <section>
      <PanelHeader
        title={t("ed2k.section.servers")}
        hint={t("ed2k.servers.hint")}
        takenAt={query.data?.takenAt}
      >
        {/*
          La connexion automatique est le geste courant : on veut être
          connecté, rarement à un serveur précis. Elle a donc sa place en
          en-tête, là où le choix ligne par ligne reste possible sans être
          imposé.
        */}
        <PanelAction
          label={t("ed2k.servers.connectAuto")}
          busy={command.busy}
          onClick={() => void command.run(() => api.connectEd2kServer())}
        />
        <PanelAction
          label={t("ed2k.servers.disconnect")}
          variant="ghost"
          busy={command.busy}
          onClick={() => void command.run(() => api.disconnectEd2kServer())}
        />
      </PanelHeader>

      <CommandError error={command.error} onDismiss={command.clearError} />

      <Async
        query={query}
        isEmpty={(data) => data.servers.length === 0}
        empty={{
          title: t("ed2k.servers.empty"),
          description: t("ed2k.servers.emptyHint"),
        }}
      >
        {(data) => (
          <Table columns={COLUMNS} minWidth={940} label={t("ed2k.section.servers")}>
            {data.servers.map((server, index) => (
              <Row key={`${server.ip}:${server.port}`} striped={index % 2 === 1}>
                <span className="flex min-w-0 items-center gap-2">
                  <span className="truncate text-fg" title={server.description || undefined}>
                    {server.name || DASH}
                  </span>
                  {server.connected && (
                    <Badge tone="success">{t("ed2k.servers.connected")}</Badge>
                  )}
                  {server.static && <Badge tone="neutral">{t("ed2k.servers.static")}</Badge>}
                </span>

                <Text>
                  {server.ip}:{server.port}
                </Text>

                <Num>{format.count(server.users)}</Num>
                <Num>{format.count(server.files)}</Num>

                {/* Le ping n'est mesuré que sur un serveur joint : zéro veut
                    dire « jamais mesuré », pas « instantané ». */}
                <Num tone={server.ping > 0 ? "fg" : "subtle"}>
                  {server.ping > 0 ? `${server.ping} ms` : DASH}
                </Num>

                <Num tone={server.failed > 0 ? "danger" : "subtle"}>
                  {server.failed > 0 ? format.count(server.failed) : DASH}
                </Num>

                <Text>{t(PRIORITY_LABELS[server.priority])}</Text>

                <span className="min-w-0">
                  {server.connected ? (
                    <ActionButton
                      label={t("ed2k.servers.disconnect")}
                      disabled={command.busy}
                      onClick={() => void command.run(() => api.disconnectEd2kServer())}
                    />
                  ) : (
                    <ActionButton
                      label={t("ed2k.servers.connect")}
                      disabled={command.busy}
                      onClick={() =>
                        void command.run(() =>
                          api.connectEd2kServer({ ip: server.ip, port: server.port }),
                        )
                      }
                    />
                  )}
                </span>
              </Row>
            ))}
          </Table>
        )}
      </Async>
    </section>
  );
}
