"use client";

/**
 * Envois : ce qui part, et qui attend son tour.
 *
 * Les deux tableaux sont sur le même écran parce qu'ils ne se lisent que
 * l'un par l'autre. Cinq transferts sortants ne veulent rien dire seuls ; avec
 * trois cents pairs en file derrière, ils disent que l'instance est demandée et
 * que sa bande passante est le goulot.
 *
 * La colonne « Total » n'est pas décorative : sur eD2k, ce qu'on a déjà donné à
 * quelqu'un détermine ce qu'il nous rendra. C'est la seule vue sur ce crédit.
 */

import { useQuery } from "@tanstack/react-query";

import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";

import { DASH, useEd2kFormat } from "./format";
import { Async, PanelHeader, REFRESH_MS } from "./panel";
import { DataTable, Num, Row, Text, type Column } from "./table";

const UPLOAD_COLUMNS: Column[] = [
  { key: "peer", label: "ed2k.col.peer", width: "minmax(160px, 2fr)" },
  { key: "software", label: "ed2k.col.software", width: "minmax(110px, 1fr)" },
  { key: "file", label: "ed2k.col.file", width: "minmax(200px, 3fr)" },
  { key: "speed", label: "ed2k.col.speed", width: "100px", align: "right" },
  { key: "sent", label: "ed2k.col.sent", width: "92px", align: "right" },
  { key: "session", label: "ed2k.col.session", width: "92px", align: "right" },
  { key: "total", label: "ed2k.col.total", width: "92px", align: "right" },
];

const QUEUE_COLUMNS: Column[] = [
  { key: "peer", label: "ed2k.col.peer", width: "minmax(160px, 2fr)" },
  { key: "address", label: "ed2k.col.address", width: "minmax(140px, 1fr)" },
  { key: "file", label: "ed2k.col.file", width: "minmax(200px, 2fr)" },
  { key: "score", label: "ed2k.col.score", width: "88px", align: "right" },
  { key: "waiting", label: "ed2k.col.waiting", width: "110px", align: "right" },
];

export function UploadsPanel() {
  const t = useT();
  const format = useEd2kFormat();

  const query = useQuery({
    queryKey: ["ed2k", "uploads"],
    queryFn: api.listEd2kUploads,
    refetchInterval: REFRESH_MS,
  });

  return (
    <section>
      <PanelHeader
        title={t("ed2k.section.uploads")}
        hint={t("ed2k.uploads.hint")}
        takenAt={query.data?.takenAt}
      />

      <Async
        query={query}
        // Vide veut dire « ni transfert ni attente » : une file pleine sans
        // transfert en cours est une information, pas un écran vide.
        isEmpty={(data) => data.uploads.length === 0 && data.queuedPeers.length === 0}
        empty={{ title: t("ed2k.uploads.empty"), description: t("ed2k.uploads.emptyHint") }}
      >
        {(data) => (
          <div className="flex flex-col gap-5">
            <div>
              <h3 className="mb-1.5 text-ui font-medium text-fg">{t("ed2k.uploads.active")}</h3>

              {data.uploads.length === 0 ? (
                <p className="rounded-lg border border-dashed border-border px-3 py-4 text-meta text-subtle">
                  {t("ed2k.uploads.empty")}
                </p>
              ) : (
                <DataTable
                  items={data.uploads}
                  columns={UPLOAD_COLUMNS}
                  minWidth={980}
                  label={t("ed2k.uploads.active")}
                  filterHint={t("ed2k.table.filterPeers")}
                  searchText={(upload) => `${upload.name} ${upload.fileName} ${upload.ip}`}
                  renderRow={(upload, index) => (
                    <Row key={`${upload.userHash}-${upload.fileHash}`} striped={index % 2 === 1}>
                      <Text tone="fg" title={upload.userHash}>
                        {upload.name || DASH}
                      </Text>
                      <Text>
                        {[upload.software, upload.version].filter(Boolean).join(" ") || DASH}
                      </Text>
                      <Text title={upload.fileHash}>{upload.fileName || DASH}</Text>
                      <Num tone={upload.speed > 0 ? "fg" : "subtle"}>
                        {format.speed(upload.speed)}
                      </Num>
                      <Num>{format.bytes(upload.transferred)}</Num>
                      <Num>{format.bytes(upload.sessionUploaded)}</Num>
                      <Num tone="subtle">{format.bytes(upload.totalUploaded)}</Num>
                    </Row>
                  )}
                />
              )}
            </div>

            <div>
              <h3 className="mb-1.5 text-ui font-medium text-fg">{t("ed2k.uploads.queue")}</h3>

              {data.queuedPeers.length === 0 ? (
                <p className="rounded-lg border border-dashed border-border px-3 py-4 text-meta text-subtle">
                  {t("ed2k.uploads.queueEmpty")}
                </p>
              ) : (
                <DataTable
                  items={data.queuedPeers}
                  columns={QUEUE_COLUMNS}
                  minWidth={880}
                  label={t("ed2k.uploads.queue")}
                  filterHint={t("ed2k.table.filterPeers")}
                  searchText={(peer) => `${peer.name} ${peer.ip} ${peer.fileHash}`}
                  renderRow={(peer, index) => (
                    <Row key={`${peer.userHash}-${peer.fileHash}-${index}`} striped={index % 2 === 1}>
                      <Text tone="fg" title={peer.userHash}>
                        {peer.name || DASH}
                      </Text>
                      <Text>{peer.ip ? `${peer.ip}:${peer.port}` : DASH}</Text>
                      <Text title={peer.fileHash}>{peer.fileHash}</Text>

                      {/* Le score mêle ancienneté et crédits : il explique
                          qu'un pair arrivé après passe devant. */}
                      <Num>{format.count(peer.score)}</Num>

                      <Num tone="subtle">
                        {peer.waitedSince ? format.since(peer.waitedSince) : DASH}
                      </Num>
                    </Row>
                  )}
                />
              )}
            </div>
          </div>
        )}
      </Async>
    </section>
  );
}
