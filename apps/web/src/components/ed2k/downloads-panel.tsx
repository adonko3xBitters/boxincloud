"use client";

/**
 * File de téléchargement.
 *
 * L'écran central du module : c'est celui qu'on laisse ouvert. Il répond à
 * trois questions, dans cet ordre — qu'est-ce qui avance, qu'est-ce qui
 * n'avance plus, et pourquoi.
 *
 * D'où la barre par ligne : « 72 % » écrit demande une lecture, une barre se
 * compare d'un coup d'œil à celles du dessus et du dessous. Et d'où la colonne
 * « Parties » : un fichier dont aucune source ne détient certaines parties ne
 * finira JAMAIS, et rien d'autre ne le montre avant des heures d'attente.
 */

import { Fragment, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Badge, Button, Input } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";
import type { Ed2kDownload } from "@/lib/api/client";

import { DASH, receivedShare, useEd2kFormat } from "./format";
import { PRIORITY_LABELS, STATUS_LABELS, STATUS_TONES } from "./labels";
import { ActionButton, CommandError, ConfirmButton, PanelAction, useCommand } from "./commands";
import { Async, PanelHeader, REFRESH_MS } from "./panel";
import { Expansion, Num, Progress, Row, Table, Text, type Column } from "./table";

const COLUMNS: Column[] = [
  { key: "name", label: "ed2k.col.name", width: "minmax(240px, 3fr)" },
  { key: "received", label: "ed2k.col.received", width: "84px", align: "right" },
  { key: "size", label: "ed2k.col.size", width: "92px", align: "right" },
  { key: "speed", label: "ed2k.col.speed", width: "100px", align: "right" },
  { key: "eta", label: "ed2k.col.eta", width: "80px", align: "right" },
  { key: "sources", label: "ed2k.col.sources", width: "92px", align: "right" },
  { key: "parts", label: "ed2k.col.parts", width: "88px", align: "right" },
  { key: "priority", label: "ed2k.col.priority", width: "96px" },
  { key: "status", label: "ed2k.col.status", width: "120px" },
  { key: "actions", label: "ed2k.col.actions", width: "150px" },
];

export function DownloadsPanel() {
  const t = useT();

  /*
    Une seule ligne dépliée à la fois.

    Chaque dépliage déclenche une requête de sources, et une source coûte une
    ligne : garder trois fichiers ouverts sur un tableau qui se rafraîchit
    toutes les deux secondes multiplierait le trafic pour un écran qu'on ne
    peut de toute façon pas lire d'un seul regard.
  */
  const [opened, setOpened] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["ed2k", "downloads"],
    queryFn: api.listEd2kDownloads,
    refetchInterval: REFRESH_MS,
  });

  const command = useCommand();
  const [adding, setAdding] = useState(false);

  return (
    <section>
      <PanelHeader
        title={t("ed2k.section.downloads")}
        hint={t("ed2k.downloads.hint")}
        takenAt={query.data?.takenAt}
      >
        <PanelAction
          label={t("ed2k.link.add")}
          variant="primary"
          onClick={() => setAdding((open) => !open)}
        />
      </PanelHeader>

      {adding && <AddLinkForm command={command} onDone={() => setAdding(false)} />}
      <CommandError error={command.error} onDismiss={command.clearError} />

      <Async
        query={query}
        isEmpty={(data) => data.downloads.length === 0}
        empty={{ title: t("ed2k.downloads.empty"), description: t("ed2k.downloads.emptyHint") }}
      >
        {(data) => (
          <Table columns={COLUMNS} minWidth={1060} label={t("ed2k.section.downloads")}>
            {data.downloads.map((download, index) => (
              <Fragment key={download.hash}>
                <DownloadRow
                  download={download}
                  striped={index % 2 === 1}
                  expanded={opened === download.hash}
                  onToggle={() =>
                    setOpened((current) => (current === download.hash ? null : download.hash))
                  }
                  command={command}
                />
                {opened === download.hash && (
                  <Expansion>
                    <SourceList hash={download.hash} />
                  </Expansion>
                )}
              </Fragment>
            ))}
          </Table>
        )}
      </Async>
    </section>
  );
}

function DownloadRow({
  download,
  striped,
  expanded,
  onToggle,
  command,
}: {
  download: Ed2kDownload;
  striped: boolean;
  expanded: boolean;
  onToggle: () => void;
  command: ReturnType<typeof useCommand>;
}) {
  const t = useT();
  const format = useEd2kFormat();

  const share = receivedShare(download.sizeDone, download.size);

  // Ce qui a transité en plus de ce qui est acquis : des blocs corrompus,
  // rejetés à la vérification. C'est l'explication d'un téléchargement qui
  // « recule », et elle n'existe nulle part ailleurs.
  const wasted = Math.max(0, download.sizeXfer - download.sizeDone);

  // Une partie que personne ne détient bloque le fichier définitivement.
  const incomplete = download.partCount > 0 && download.availableParts < download.partCount;

  return (
    <Row
      striped={striped}
      expanded={expanded}
      onClick={onToggle}
      label={t("ed2k.downloads.expand", { name: download.name })}
    >
      <span className="min-w-0">
        <span className="block truncate text-fg" title={download.name}>
          {download.name}
        </span>
        <Progress
          value={share}
          tone={STATUS_TONES[download.status]}
          label={t("ed2k.col.progress")}
        />
      </span>

      <Num
        tone={wasted > 0 ? "warning" : "fg"}
        title={wasted > 0 ? t("ed2k.downloads.wasted", { value: format.bytes(wasted) }) : undefined}
      >
        {format.percent(share)}
      </Num>

      <Num>{format.bytes(download.size)}</Num>
      <Num tone={download.speed > 0 ? "fg" : "subtle"}>{format.speed(download.speed)}</Num>
      <Num>{download.etaSeconds === undefined ? DASH : format.duration(download.etaSeconds)}</Num>

      {/* Sources qui transfèrent, sur sources connues. Le second nombre sans le
          premier fait croire à cent contributeurs quand trois travaillent. */}
      <Num tone={download.sources.transferring > 0 ? "fg" : "subtle"}>
        {`${format.count(download.sources.transferring)} / ${format.count(download.sources.total)}`}
      </Num>

      <Num tone={incomplete ? "warning" : "muted"}>
        {`${format.count(download.availableParts)} / ${format.count(download.partCount)}`}
      </Num>

      <Text>{t(PRIORITY_LABELS[download.priority])}</Text>

      <span className="min-w-0">
        <Badge tone={STATUS_TONES[download.status] === "danger" ? "danger" : "neutral"}>
          {t(STATUS_LABELS[download.status])}
        </Badge>
      </span>

      {/*
        Un seul bouton de bascule plutôt que deux : « pause » et « reprendre »
        ne peuvent jamais s'appliquer en même temps, et les afficher tous les
        deux ferait chercher lequel est actif.
      */}
      <span className="flex min-w-0 items-center gap-0.5">
        {download.status === "paused" ? (
          <ActionButton
            label={t("ed2k.action.resume")}
            disabled={command.busy}
            onClick={() => void command.run(() => api.actOnEd2kDownload(download.hash, "resume"))}
          />
        ) : (
          <ActionButton
            label={t("ed2k.action.pause")}
            disabled={command.busy}
            onClick={() => void command.run(() => api.actOnEd2kDownload(download.hash, "pause"))}
          />
        )}

        <ConfirmButton
          label={t("ed2k.action.cancel")}
          confirmLabel={t("ed2k.action.cancelConfirm")}
          disabled={command.busy}
          onConfirm={() => void command.run(() => api.actOnEd2kDownload(download.hash, "cancel"))}
        />
      </span>
    </Row>
  );
}

const SOURCE_COLUMNS: Column[] = [
  { key: "peer", label: "ed2k.col.peer", width: "minmax(180px, 2fr)" },
  { key: "software", label: "ed2k.col.software", width: "minmax(120px, 1fr)" },
  { key: "address", label: "ed2k.col.address", width: "minmax(140px, 1fr)" },
  { key: "speed", label: "ed2k.col.speed", width: "100px", align: "right" },
  { key: "rank", label: "ed2k.col.rank", width: "72px", align: "right" },
  { key: "available", label: "ed2k.col.available", width: "96px", align: "right" },
];

/**
 * Sources d'un fichier.
 *
 * Requête à part, faite seulement au dépliage : un fichier populaire compte
 * plusieurs centaines de sources, et les charger avec la file multiplierait
 * par cent le poids d'un écran dont on ne regarde qu'une ligne à la fois.
 */
function SourceList({ hash }: { hash: string }) {
  const t = useT();
  const format = useEd2kFormat();

  const query = useQuery({
    queryKey: ["ed2k", "downloads", hash, "sources"],
    queryFn: () => api.listEd2kDownloadSources(hash),
    refetchInterval: REFRESH_MS,
  });

  return (
    <Async
      query={query}
      isEmpty={(data) => data.sources.length === 0}
      empty={{
        title: t("ed2k.downloads.sourcesEmpty"),
        description: t("ed2k.downloads.sourcesEmptyHint"),
      }}
    >
      {(data) => (
        <Table columns={SOURCE_COLUMNS} minWidth={780} label={t("ed2k.col.sources")}>
          {data.sources.map((source, index) => (
            <Row key={`${source.userHash}-${index}`} striped={index % 2 === 1}>
              <Text tone={source.downloading ? "fg" : "muted"} title={source.userHash}>
                {source.name || DASH}
              </Text>

              <Text>{[source.software, source.version].filter(Boolean).join(" ") || DASH}</Text>

              {/* Un LowID n'a pas d'adresse joignable : la colonne le dit
                  plutôt que d'afficher un vide qu'on lirait comme un bug. */}
              <Text tone={source.lowId ? "subtle" : "muted"}>
                {source.lowId ? t("ed2k.id.low") : source.ip ? `${source.ip}:${source.port}` : DASH}
              </Text>

              <Num tone={source.speed > 0 ? "fg" : "subtle"}>{format.speed(source.speed)}</Num>

              {/* Rang zéro = pas en file : soit on transfère, soit le pair ne
                  nous a pas classés. Un « 0 » se lirait « premier ». */}
              <Num tone="subtle">
                {source.queueRank > 0 ? format.count(source.queueRank) : DASH}
              </Num>

              <Num>{format.count(source.availableParts)}</Num>
            </Row>
          ))}
        </Table>
      )}
    </Async>
  );
}

/**
 * Ajout d'un lien ed2k://.
 *
 * Un champ, pas une boîte de dialogue : le geste consiste à coller et valider,
 * et une modale ajouterait deux clics à une action qu'on répète.
 *
 * Le champ se vide APRÈS le succès seulement. Un lien refusé — mauvais
 * protocole, forme invalide — doit rester à l'écran pour être corrigé plutôt
 * que d'être à recoller.
 */
function AddLinkForm({
  command,
  onDone,
}: {
  command: ReturnType<typeof useCommand>;
  onDone: () => void;
}) {
  const t = useT();
  const [link, setLink] = useState("");

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void command.run(() => api.addEd2kLink(link)).then(() => {
          setLink("");
          onDone();
        });
      }}
      className="mb-3 flex flex-wrap items-end gap-2 rounded-lg border border-border bg-surface p-3"
    >
      <div className="min-w-[280px] flex-1">
        <Input
          name="link"
          label={t("ed2k.link.label")}
          hint={t("ed2k.link.hint")}
          value={link}
          onChange={(event) => setLink(event.target.value)}
          placeholder="ed2k://|file|…"
          autoComplete="off"
        />
      </div>

      <Button type="submit" size="sm" loading={command.busy} disabled={link.trim() === ""}>
        {t("ed2k.link.submit")}
      </Button>
      <Button type="button" variant="ghost" size="sm" onClick={onDone}>
        {t("action.cancel")}
      </Button>
    </form>
  );
}
