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

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Badge, Button, Input } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";

import { DASH, useEd2kFormat } from "./format";
import { PRIORITY_LABELS } from "./labels";
import { ActionButton, CommandError, ConfirmButton, PanelAction, useCommand } from "./commands";
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
  { key: "actions", label: "ed2k.col.actions", width: "170px" },
];

export function ServersPanel() {
  const t = useT();
  const format = useEd2kFormat();

  const command = useCommand();
  const [form, setForm] = useState<"import" | "add" | null>(null);

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
        {/*
          L'import passe AVANT la connexion, et en bouton principal.

          Sur une instance neuve la liste est vide, et « se connecter » n'a alors
          rien à joindre : le premier geste est de remplir la liste. Mettre la
          connexion en tête aurait fait cliquer sur un bouton qui échoue.
        */}
        <PanelAction
          label={t("ed2k.servers.import")}
          variant="primary"
          onClick={() => setForm(form === "import" ? null : "import")}
        />
        <PanelAction
          label={t("ed2k.servers.add")}
          variant="ghost"
          onClick={() => setForm(form === "add" ? null : "add")}
        />
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

      {form === "import" && (
        <ImportForm command={command} onDone={() => setForm(null)} />
      )}
      {form === "add" && <AddServerForm command={command} onDone={() => setForm(null)} />}

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

                <span className="flex min-w-0 items-center gap-1">
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

                  {/*
                    Retirer ne détruit rien qu'on ne puisse retrouver : un
                    ré-import remet la liste. La confirmation en deux temps
                    protège malgré tout du clic voisin, la colonne étant étroite.
                  */}
                  <ConfirmButton
                    label={t("ed2k.servers.remove")}
                    confirmLabel={t("ed2k.servers.removeConfirm")}
                    disabled={command.busy}
                    onConfirm={() =>
                      void command.run(() => api.removeEd2kServer(server.ip, server.port))
                    }
                  />
                </span>
              </Row>
            ))}
          </Table>
        )}
      </Async>
    </section>
  );
}

/**
 * Import d'une liste publiée.
 *
 * Le champ est une simple adresse : la validation appartient au serveur, qui
 * n'accepte que http et https — un `file://` ferait lire au démon son propre
 * disque, et c'est le seul contrôle qui ait un sens ici.
 */
function ImportForm({
  command,
  onDone,
}: {
  command: ReturnType<typeof useCommand>;
  onDone: () => void;
}) {
  const t = useT();
  const [url, setUrl] = useState("https://upd.emule-security.org/server.met");

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void command.run(() => api.importEd2kServerList(url)).then(onDone);
      }}
      className="mb-3 flex flex-wrap items-end gap-2 rounded-lg border border-border bg-surface p-3"
    >
      <div className="min-w-[280px] flex-1">
        <Input
          name="url"
          label={t("ed2k.servers.importUrl")}
          hint={t("ed2k.servers.importHint")}
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          autoComplete="off"
        />
      </div>

      <Button type="submit" size="sm" loading={command.busy} disabled={url.trim() === ""}>
        {t("ed2k.servers.importSubmit")}
      </Button>
      <Button type="button" variant="ghost" size="sm" onClick={onDone}>
        {t("action.cancel")}
      </Button>
    </form>
  );
}

/** Ajout d'un serveur à la main, pour ce qui ne figure sur aucune liste. */
function AddServerForm({
  command,
  onDone,
}: {
  command: ReturnType<typeof useCommand>;
  onDone: () => void;
}) {
  const t = useT();
  const [ip, setIp] = useState("");
  const [port, setPort] = useState("4661");
  const [name, setName] = useState("");

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void command
          .run(() => api.addEd2kServer({ ip, port: Number(port), name: name || undefined }))
          .then(onDone);
      }}
      className="mb-3 flex flex-wrap items-end gap-2 rounded-lg border border-border bg-surface p-3"
    >
      <div className="min-w-[180px] flex-1">
        <Input
          name="ip"
          label={t("ed2k.servers.addIp")}
          hint={t("ed2k.servers.addHint")}
          value={ip}
          onChange={(event) => setIp(event.target.value)}
          autoComplete="off"
        />
      </div>

      <div className="w-24">
        <Input
          name="port"
          label={t("ed2k.servers.addPort")}
          inputMode="numeric"
          value={port}
          onChange={(event) => setPort(event.target.value)}
          autoComplete="off"
        />
      </div>

      <div className="min-w-[160px] flex-1">
        <Input
          name="name"
          label={t("ed2k.servers.addName")}
          value={name}
          onChange={(event) => setName(event.target.value)}
          autoComplete="off"
        />
      </div>

      <Button
        type="submit"
        size="sm"
        loading={command.busy}
        disabled={ip.trim() === "" || port.trim() === ""}
      >
        {t("ed2k.servers.addSubmit")}
      </Button>
      <Button type="button" variant="ghost" size="sm" onClick={onDone}>
        {t("action.cancel")}
      </Button>
    </form>
  );
}
